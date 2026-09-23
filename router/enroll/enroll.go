/*
	Copyright NetFoundry Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package enroll

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hanzozt/zt/v2/router/env"

	"github.com/go-resty/resty/v2"
	"github.com/hanzozt/edge-api/rest_model"
	"github.com/hanzozt/identity/certtools"
	"github.com/hanzozt/sdk-golang/zt"
	"github.com/hanzozt/sdk-golang/zt/enroll"
	"github.com/michaelquigley/pfxlog"
)

type apiPost struct {
	ServerCertCsr string `json:"serverCertCsr"`
	CertCsr       string `json:"certCsr"`
}

type Enroller interface {
	Enroll(jwt []byte, silent bool, engine string, keyAlg zt.KeyAlgVar) error
}

type RestEnroller struct {
	fullConfig *env.Config
	config     *env.EdgeConfig

	// Issuer, when set, replaces the token's issuer as the controller address
	// enrollment verifies against and posts to. A router beside its controller
	// enrolls over loopback while the token names the public address, which
	// may sit behind a TLS-terminating proxy.
	Issuer string
}

func NewRestEnroller(config *env.Config) *RestEnroller {
	return &RestEnroller{
		fullConfig: config,
		config:     config.Edge,
	}
}

func (re *RestEnroller) Enroll(jwtBuf []byte, silent bool, engine string, keyAlg zt.KeyAlgVar) error {
	log := pfxlog.Logger()

	if re.config == nil {
		return errors.New("no configuration provided")
	}

	identityConfig := re.fullConfig.IdConfig

	if re.config.RouterConfig.Id != nil {
		log.Warnf("identity detected, note that any identity information will be overwritten when enrolling")
	}

	ec, err := parseToken(strings.TrimSpace(string(jwtBuf)), re.Issuer)
	if err != nil {
		log.WithField("cause", err).Fatal("failed to parse JWT")
	}

	log.Debug("JWT parsed")

	rootCaPool := x509.NewCertPool()
	rootCaPool.AddCert(ec.SignatureCert)

	rootCas := enroll.FetchCertificates(ec.Issuer, rootCaPool)

	if len(rootCas) == 0 {
		log.Fatal("no valid root CAs were found")
	}

	var engUrl *url.URL

	if engine != "" {
		if engUrl, err = url.Parse(engine); err != nil {
			return fmt.Errorf("could not parse engine string: %s", err)
		}
	}

	//writes key if it is file based
	var key crypto.PrivateKey
	if keyAlg.EC() {
		key, err = certtools.GetKey(engUrl, identityConfig.Key, "ec:P-256")
	} else if keyAlg.RSA() {
		key, err = certtools.GetKey(engUrl, identityConfig.Key, "rsa:4096")
	} else {
		panic(fmt.Sprintf("invalid KeyAlg specified: %s", keyAlg.Get()))
	}

	if err != nil {
		return fmt.Errorf("could not obtain private key: %s", err)
	}

	subject := csrSubject(ec.Subject, re.config.Csr)

	serverCsr, err := CreateCsr(key, x509.UnknownSignatureAlgorithm, subject, re.config.Csr.Sans)

	if err != nil {
		return fmt.Errorf("failed to generate server CSR: %s", err)
	}

	clientCsr, err := CreateCsr(key, x509.UnknownSignatureAlgorithm, subject, re.config.Csr.Sans)

	if err != nil {
		return fmt.Errorf("failed to generate client CSR: %s", err)
	}

	er := &apiPost{
		CertCsr:       clientCsr,
		ServerCertCsr: serverCsr,
	}

	client := resty.New()

	caCertPool := x509.NewCertPool()
	for _, cert := range rootCas {
		caCertPool.AddCert(cert)
	}

	tc := &tls.Config{
		RootCAs:    caCertPool,
		MinVersion: tls.VersionTLS12,
	}

	client.SetTLSClientConfig(tc)

	envelope, err := re.Send(client, ec.EnrolmentUrl(), er)

	if err != nil {
		return err
	}

	resp := envelope.Data

	if resp.Cert == "" {
		return fmt.Errorf("enrollment response did not contain a cert")
	}

	if resp.ServerCert == "" {
		return fmt.Errorf("enrollment response did not contain a server cert")
	}

	if resp.Ca == "" {
		return fmt.Errorf("enrollment response did not contain a CA chain")
	}

	if err = os.WriteFile(identityConfig.Cert, []byte(resp.Cert), 0600); err != nil {
		return fmt.Errorf("unable to write client cert to [%s]: %s", identityConfig.Cert, err)
	}

	if err = os.WriteFile(identityConfig.ServerCert, []byte(resp.ServerCert), 0600); err != nil {
		return fmt.Errorf("unable to write server cert to [%s]: %s", identityConfig.ServerCert, err)
	}

	if err = os.WriteFile(identityConfig.CA, []byte(resp.Ca), 0600); err != nil {
		return fmt.Errorf("unable to write CA certs to [%s]: %s", identityConfig.CA, err)
	}

	var controllers []string
	if len(re.fullConfig.Ctrl.InitialEndpoints) > 0 {
		for _, ep := range re.fullConfig.Ctrl.InitialEndpoints {
			controllers = append(controllers, ep.String())
		}
	} else {
		// if config is missing endpoints, try to get them from the JWT claimset
		claims := jwt.MapClaims{}
		parser := jwt.NewParser()
		_, _, err := parser.ParseUnverified(strings.TrimSpace(string(jwtBuf)), claims)
		if err == nil {
			if ctrl, ok := claims["ctrl"]; ok {
				controllers = append(controllers, ctrl.(string))
			}

			if ctrls, ok := claims["ctrls"]; ok {
				if ctrlsSlice, ok := ctrls.([]interface{}); ok {
					for _, ctrl := range ctrlsSlice {
						controllers = append(controllers, ctrl.(string))
					}
				}
			}
		} else {
			log.Warnf("failed to parse JWT for custom claims: %v", err)
		}
	}

	if len(controllers) == 0 {
		controllers = ec.Controllers
	}

	if err = re.fullConfig.SaveControllerEndpoints(controllers); err != nil {
		return err
	}

	if re.fullConfig.Edge.Db != "" {
		var fileInfo os.FileInfo
		if fileInfo, err = os.Stat(re.fullConfig.Edge.Db); err == nil {
			if !fileInfo.IsDir() {
				log.WithField("path", re.fullConfig.Edge.Db).Info("deleting existing router data model save file")
				if err = os.Remove(re.fullConfig.Edge.Db); err != nil {
					log.WithField("path", re.fullConfig.Edge.Db).WithError(err).
						Error("failed to delete existing router data model save file")
				}
			}
		}
	}

	log.Info("registration complete")
	return nil
}

// parseToken verifies an enrollment token against the certificate its issuer
// serves, reading the issuer from the token unless one is given.
func parseToken(token, issuer string) (*zt.EnrollmentClaims, error) {
	if issuer == "" {
		ec, _, err := enroll.ParseToken(token)
		return ec, err
	}

	ec := &zt.EnrollmentClaims{}
	_, err := jwt.NewParser().ParseWithClaims(token, ec, func(t *jwt.Token) (any, error) {
		ec.Issuer = issuer
		return enroll.ValidateToken(t)
	})
	return ec, err
}

func (re *RestEnroller) Send(client *resty.Client, enrollUrl string, e *apiPost) (*rest_model.EnrollmentCertsEnvelope, error) {
	envelope := rest_model.EnrollmentCertsEnvelope{}

	resp, err := client.R().
		SetBody(e).
		Post(enrollUrl)

	if err != nil {
		return nil, err
	}

	body := resp.Body()

	err = json.Unmarshal(body, &envelope)

	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("enrollment failed received HTTP status [%s]: %s", resp.Status(), resp.Body())
	}

	return &envelope, nil
}
