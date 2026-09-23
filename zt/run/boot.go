package run

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hanzozt/sdk-golang/zt"
	"github.com/hanzozt/zt/v2/common/version"
	"github.com/hanzozt/zt/v2/controller"
	"github.com/hanzozt/zt/v2/controller/change"
	"github.com/hanzozt/zt/v2/controller/config"
	"github.com/hanzozt/zt/v2/controller/db"
	"github.com/hanzozt/zt/v2/controller/fields"
	"github.com/hanzozt/zt/v2/controller/model"
	"github.com/hanzozt/zt/v2/controller/models"
	"github.com/hanzozt/zt/v2/controller/server"
	"github.com/hanzozt/zt/v2/router/enroll"
	"github.com/hanzozt/zt/v2/router/env"
	"github.com/hanzozt/zt/v2/zt/cmd/create"
	"github.com/hanzozt/zt/v2/zt/constants"
	"github.com/michaelquigley/pfxlog"
	"github.com/spf13/cobra"
)

// The roles share one pod: the API listens on apiPort behind whatever address
// it advertises, and the control plane listens on loopback only, for the
// router beside it.
const (
	loopback = "127.0.0.1"
	apiPort  = constants.DefaultCtrlEdgeAdvertisedPort
	ctrlPort = constants.DefaultCtrlAdvertisedPort

	iamIssuer   = "ZT_IAM_ISSUER"
	iamJwks     = "ZT_IAM_JWKS"
	iamAudience = "ZT_IAM_AUDIENCE"
	iamAdmins   = "ZT_IAM_ADMINS"

	// iam names the ext-jwt signer and the auth policy that admits only it.
	iam = "iam"
)

// Controller runs the controller role from the environment. An empty ZT_HOME
// gets its PKI and config first; every boot then converges the database on
// Hanzo IAM as the only way in before the controller runs.
func Controller(cmd *cobra.Command, opts Options) error {
	home, err := need(constants.HomeVarName)
	if err != nil {
		return err
	}
	addr, err := need(constants.CtrlAdvertisedAddressVarName)
	if err != nil {
		return err
	}
	port := os.Getenv(constants.CtrlAdvertisedPortVarName)
	if port == "" {
		port = apiPort
	}
	issuer, err := need(iamIssuer)
	if err != nil {
		return err
	}
	jwks, err := need(iamJwks)
	if err != nil {
		return err
	}
	var admins []string
	for _, s := range strings.Split(os.Getenv(iamAdmins), ",") {
		if s = strings.TrimSpace(s); s != "" {
			admins = append(admins, s)
		}
	}
	if len(admins) == 0 {
		return fmt.Errorf("%s names no subject, so nobody could administer the network", iamAdmins)
	}

	cfg := filepath.Join(home, "ctrl.yaml")
	pki := &QuickstartOpts{Home: home, TrustDomain: "zt", InstanceID: "ctrl", out: os.Stdout, errOut: os.Stderr}
	pki.pkiEnv()
	if _, err := os.Stat(cfg); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Join(home, "db"), 0o700); err != nil {
			return err
		}
		pki.CreateMinimalPki()
		if err := writeControllerConfig(cfg, addr, port); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if err := seed(cfg, issuer, jwks, os.Getenv(iamAudience), admins, os.Getenv(constants.RouterNameVarName), home); err != nil {
		return err
	}

	(&ControllerAction{Options: opts}).Run(cmd, []string{cfg})
	return nil
}

// Router runs the router role beside its controller. Until the router holds a
// certificate it waits for the enrollment token the controller writes into
// ZT_HOME, enrolls over loopback and deletes the token.
func Router(cmd *cobra.Command, opts Options) error {
	home, err := need(constants.HomeVarName)
	if err != nil {
		return err
	}
	name, err := need(constants.RouterNameVarName)
	if err != nil {
		return err
	}
	cfg := filepath.Join(home, name+".yaml")

	data := &create.ConfigTemplateValues{}
	data.PopulateConfigValues()
	create.SetRouterIdentity(&data.Router, name)
	if _, err := os.Stat(data.Router.IdentityCert); errors.Is(err, fs.ErrNotExist) {
		if err := enrollRouter(cfg, filepath.Join(home, name+".jwt"), name, data); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	(&RouterAction{Options: opts}).Run(cmd, []string{cfg})
	return nil
}

func need(name string) (string, error) {
	if v := os.Getenv(name); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s is required", name)
}

func writeControllerConfig(cfg, addr, port string) error {
	c := create.NewCmdCreateConfigController()
	populate := c.PreRun
	c.PreRun = func(cc *cobra.Command, args []string) {
		populate(cc, args)
		d := &c.ConfigData.Controller
		d.Ctrl.BindAddress, d.Ctrl.AdvertisedAddress, d.Ctrl.AdvertisedPort = loopback, loopback, ctrlPort
		d.EdgeApi.Address, d.EdgeApi.Port = addr, port
		d.Web.BindPoints.InterfacePort = apiPort
		d.Web.BindPoints.AddressAddress, d.Web.BindPoints.AddressPort = addr, port
	}
	c.SetArgs([]string{"--output=" + cfg})
	return c.Execute()
}

// seed creates what is missing and leaves what exists untouched: the default
// admin the database requires, the IAM signer, the auth policy that admits
// only it, an admin identity per IAM subject, and the edge router with its
// enrollment token.
func seed(cfg, issuer, jwks, audience string, admins []string, router, home string) error {
	c, err := config.LoadConfig(cfg)
	if err != nil {
		return err
	}
	host, err := controller.NewController(c, version.GetCmdBuildInfo())
	if err != nil {
		return err
	}
	edge, err := server.NewController(host)
	if err != nil {
		return err
	}
	edge.Initialize()
	defer edge.Shutdown()

	m := edge.AppEnv.Managers
	ctx := change.New().SetSourceType("boot").SetChangeAuthorType(change.AuthorTypeController)

	// The database will not run without a default admin. Its password is
	// random, exists only as the database's hash, and is never used.
	if admin, err := m.Identity.ReadDefaultAdmin(); err != nil {
		return err
	} else if admin == nil {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return err
		}
		if err := m.Identity.InitializeDefaultAdmin("admin", base64.RawURLEncoding.EncodeToString(secret), "Default Admin"); err != nil {
			return err
		}
	}

	// Passwords are never a way in.
	policy, err := m.AuthPolicy.Read(db.DefaultAuthPolicyId)
	if err != nil {
		return err
	}
	if policy.Primary.Updb.Allowed {
		policy.Primary.Updb.Allowed = false
		if err := m.AuthPolicy.Update(policy, fields.UpdatedFieldsMap{db.FieldAuthPolicyPrimaryUpdbAllowed: struct{}{}}, ctx); err != nil {
			return err
		}
	}

	claim := "sub"
	signer := &model.ExternalJwtSigner{
		BaseEntity:     models.BaseEntity{Id: iam},
		Name:           iam,
		Issuer:         &issuer,
		JwksEndpoint:   &jwks,
		ClaimsProperty: &claim,
		UseExternalId:  true,
		Enabled:        true,
	}
	if audience != "" {
		signer.Audience = &audience
	}
	if err := ensure(m.ExternalJwtSigner.IsEntityPresent, iam, func() error {
		return m.ExternalJwtSigner.Create(signer, ctx)
	}); err != nil {
		return err
	}
	if err := ensure(m.AuthPolicy.IsEntityPresent, iam, func() error {
		return m.AuthPolicy.Create(&model.AuthPolicy{
			BaseEntity: models.BaseEntity{Id: iam},
			Name:       iam,
			Primary: model.AuthPolicyPrimary{
				ExtJwt: model.AuthPolicyExtJwt{Allowed: true, AllowedExtJwtSigners: []string{iam}},
			},
		}, ctx)
	}); err != nil {
		return err
	}

	for _, sub := range admins {
		if found, err := m.Identity.ReadByExternalId(sub); err != nil {
			return err
		} else if found != nil {
			continue
		}
		if err := m.Identity.Create(&model.Identity{
			Name:           sub,
			IdentityTypeId: db.DefaultIdentityType,
			IsAdmin:        true,
			ExternalId:     &sub,
			AuthPolicyId:   iam,
		}, ctx); err != nil {
			return err
		}
	}

	if router == "" {
		return nil
	}

	// Every identity may use every router and every service is reachable
	// through them; service policies alone decide who dials what.
	if err := ensure(m.EdgeRouterPolicy.IsEntityPresent, "all", func() error {
		return m.EdgeRouterPolicy.Create(&model.EdgeRouterPolicy{BaseEntity: models.BaseEntity{Id: "all"}, Name: "all",
			Semantic: db.SemanticAnyOf, IdentityRoles: []string{"#all"}, EdgeRouterRoles: []string{"#all"}}, ctx)
	}); err != nil {
		return err
	}
	if err := ensure(m.ServiceEdgeRouterPolicy.IsEntityPresent, "all", func() error {
		return m.ServiceEdgeRouterPolicy.Create(&model.ServiceEdgeRouterPolicy{BaseEntity: models.BaseEntity{Id: "all"}, Name: "all",
			Semantic: db.SemanticAnyOf, ServiceRoles: []string{"#all"}, EdgeRouterRoles: []string{"#all"}}, ctx)
	}); err != nil {
		return err
	}

	found, err := m.EdgeRouter.BaseList(fmt.Sprintf("name = %q limit 1", router))
	if err != nil {
		return err
	}
	if len(found.Entities) > 0 {
		return nil
	}
	er := &model.EdgeRouter{Name: router}
	if err := m.EdgeRouter.Create(er, ctx); err != nil {
		return err
	}
	var token string
	if err := m.EdgeRouter.CollectEnrollments(er.Id, func(e *model.Enrollment) error {
		token = e.Jwt
		return nil
	}); err != nil {
		return err
	}
	path := filepath.Join(home, router+".jwt")
	if err := os.WriteFile(path+".tmp", []byte(token), 0o600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// ensure creates the entity named id unless it is already present.
func ensure(present func(string) (bool, error), id string, create func() error) error {
	if ok, err := present(id); err != nil || ok {
		return err
	}
	return create()
}

func enrollRouter(cfg, token, name string, data *create.ConfigTemplateValues) error {
	await("enrollment token "+token, func() bool {
		_, err := os.Stat(token)
		return err == nil
	})

	data.Controller.Ctrl.AdvertisedAddress, data.Controller.Ctrl.AdvertisedPort = loopback, ctrlPort
	c := create.NewCmdCreateConfigRouterEdge(&create.CreateConfigRouterOptions{}, data)
	c.SetArgs([]string{"--routerName=" + name, "--output=" + cfg, "--private", "--tunnelerMode=none"})
	if err := c.Execute(); err != nil {
		return err
	}

	api := net.JoinHostPort(loopback, apiPort)
	await("controller at "+api, func() bool {
		conn, err := net.DialTimeout("tcp", api, time.Second)
		if err == nil {
			_ = conn.Close()
		}
		return err == nil
	})

	jwt, err := os.ReadFile(token)
	if err != nil {
		return err
	}
	conf, err := env.LoadConfigWithOptions(cfg, false)
	if err != nil {
		return err
	}
	e := enroll.NewRestEnroller(conf)
	e.Issuer = "https://" + api
	if err := e.Enroll(jwt, true, "", zt.KeyAlgVar("EC")); err != nil {
		return err
	}
	return os.Remove(token)
}

func await(what string, ready func() bool) {
	for i := 0; !ready(); i++ {
		if i%30 == 0 {
			pfxlog.Logger().Infof("waiting for %s", what)
		}
		time.Sleep(time.Second)
	}
}
