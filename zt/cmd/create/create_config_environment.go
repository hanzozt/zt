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

package create

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	"github.com/hanzozt/zt/v2/zt/cmd/templates"
	"github.com/hanzozt/zt/v2/zt/constants"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type EnvVariableTemplateData struct {
	OSCommentPrefix string
	OSVarDeclare    string
	EnvVars         []EnvVar
}

type EnvVar struct {
	Name        string
	Description string
	Value       string
}

var (
	createConfigEnvironmentLong = templates.LongDesc(`
		Displays available environment variable manual overrides
`)

	createConfigEnvironmentExample = templates.Examples(`
		# Display environment variables and their values 
		zt create config environment

		# Print an environment file to the console
		zt create config environment --output stdout
	`)
)

//go:embed config_templates/environment.yml
var environmentConfigTemplate string

var environmentOptions *CreateConfigEnvironmentOptions

// CreateConfigEnvironmentOptions the options for the create environment command
type CreateConfigEnvironmentOptions struct {
	CreateConfigOptions
	EnvVariableTemplateData

	DisableOSVarDeclare bool
}

func (options *CreateConfigEnvironmentOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&options.DisableOSVarDeclare, "no-shell", false, "Disable printing assignments prefixed with 'SET' (Windows) or 'export' (Unix)")
}

// NewCmdCreateConfigEnvironment creates a command object for the "environment" command
func NewCmdCreateConfigEnvironment() *cobra.Command {
	environmentOptions = &CreateConfigEnvironmentOptions{}

	data := &ConfigTemplateValues{}

	cmd := &cobra.Command{
		Use:     "environment",
		Short:   "Display config environment variables",
		Aliases: []string{"env"},
		Long:    createConfigEnvironmentLong,
		Example: createConfigEnvironmentExample,
		PreRun: func(cmd *cobra.Command, args []string) {
			data.PopulateConfigValues()
			// Set router identities
			SetRouterIdentity(&data.Router, validateRouterName(os.Getenv(constants.RouterNameVarName)))
			// Set up other identity info
			SetControllerIdentity(&data.Controller)
			SetEdgeConfig(&data.Controller)
			SetWebConfig(&data.Controller)
			SetConsoleConfig(&data.Controller.Web.BindPoints.Console)

			environmentOptions.EnvVars = []EnvVar{
				{constants.HomeVarName, constants.HomeVarDescription, data.Home},
				{constants.NetworkNameVarName, constants.NetworkNameVarDescription, data.HostnameOrNetworkName},
				{constants.PkiCtrlCertVarName, constants.PkiCtrlCertVarDescription, data.Controller.Identity.Cert},
				{constants.PkiCtrlServerCertVarName, constants.PkiCtrlServerCertVarDescription, data.Controller.Identity.ServerCert},
				{constants.PkiCtrlKeyVarName, constants.PkiCtrlKeyVarDescription, data.Controller.Identity.Key},
				{constants.PkiCtrlCAVarName, constants.PkiCtrlCAVarDescription, data.Controller.Identity.Ca},
				{constants.CtrlDatabaseFileVarName, constants.CtrlDatabaseFileVarDescription, data.Controller.Database.DatabaseFile},
				{constants.CtrlBindAddressVarName, constants.CtrlBindAddressVarDescription, data.Controller.Ctrl.BindAddress},
				{constants.CtrlAdvertisedAddressVarName, constants.CtrlAdvertisedAddressVarDescription, data.Controller.Ctrl.AdvertisedAddress},
				{constants.CtrlAdvertisedPortVarName, constants.CtrlAdvertisedPortVarDescription, data.Controller.Ctrl.AdvertisedPort},
				{constants.CtrlConsoleLocationVarName, constants.CtrlConsoleLocationVarDescription, data.Controller.Web.BindPoints.Console.Location},
				{constants.CtrlEdgeAdvertisedAddressVarName, constants.CtrlEdgeAdvertisedAddressVarDescription, data.Controller.EdgeApi.Address},
				{constants.CtrlEdgeAltAdvertisedAddressVarName, constants.CtrlEdgeAltAdvertisedAddressVarDescription, data.Controller.EdgeApi.Address},
				{constants.CtrlEdgeAdvertisedPortVarName, constants.CtrlEdgeAdvertisedPortVarDescription, data.Controller.EdgeApi.Port},
				{constants.CtrlEdgeBindAddressVarName, constants.CtrlEdgeBindAddressVarDescription, data.Controller.EdgeApi.Address},
				{constants.PkiSignerCertVarName, constants.PkiSignerCertVarDescription, data.Controller.EdgeEnrollment.SigningCert},
				{constants.PkiSignerKeyVarName, constants.PkiSignerKeyVarDescription, data.Controller.EdgeEnrollment.SigningCertKey},
				{constants.CtrlEdgeIdentityEnrollmentDurationVarName, constants.CtrlEdgeIdentityEnrollmentDurationVarDescription, strconv.FormatInt(int64(data.Controller.EdgeEnrollment.EdgeIdentityDuration.Minutes()), 10)},
				{constants.CtrlEdgeRouterEnrollmentDurationVarName, constants.CtrlEdgeRouterEnrollmentDurationVarDescription, strconv.FormatInt(int64(data.Controller.EdgeEnrollment.EdgeRouterDuration.Minutes()), 10)},
				{constants.CtrlEdgeAdvertisedAddressVarName, constants.CtrlEdgeAdvertisedAddressVarDescription, data.Controller.Web.BindPoints.AddressAddress},
				{constants.CtrlEdgeAdvertisedPortVarName, constants.CtrlEdgeAdvertisedPortVarDescription, data.Controller.Web.BindPoints.AddressPort},
				{constants.CtrlPkiEdgeCertVarName, constants.CtrlPkiEdgeCertVarDescription, data.Controller.Web.Identity.Cert},
				{constants.CtrlPkiEdgeServerCertVarName, constants.CtrlPkiEdgeServerCertVarDescription, data.Controller.Web.Identity.ServerCert},
				{constants.CtrlPkiEdgeKeyVarName, constants.CtrlPkiEdgeKeyVarDescription, data.Controller.Web.Identity.Key},
				{constants.CtrlPkiEdgeCAVarName, constants.CtrlPkiEdgeCAVarDescription, data.Controller.Web.Identity.Ca},
				{constants.PkiAltServerCertVarName, constants.PkiAltServerCertVarDescription, data.Controller.Web.Identity.AltServerCert},
				{constants.PkiAltServerKeyVarName, constants.PkiAltServerKeyVarDescription, data.Controller.Web.Identity.AltServerKey},
				{constants.RouterNameVarName, constants.RouterNameVarDescription, data.Router.Name},
				{constants.RouterPortVarName, constants.RouterPortVarDescription, data.Router.Edge.Port},
				{constants.RouterListenerBindPortVarName, constants.RouterListenerBindPortVarDescription, data.Router.Edge.ListenerBindPort},
				{constants.RouterIdentityCertVarName, constants.RouterIdentityCertVarDescription, data.Router.IdentityCert},
				{constants.RouterIdentityServerCertVarName, constants.RouterIdentityServerCertVarDescription, data.Router.IdentityServerCert},
				{constants.RouterIdentityKeyVarName, constants.RouterIdentityKeyVarDescription, data.Router.IdentityKey},
				{constants.RouterIdentityCAVarName, constants.RouterIdentityCAVarDescription, data.Router.IdentityCA},
				{constants.RouterIPOverrideVarName, constants.RouterIPOverrideVarDescription, data.Router.Edge.IPOverride},
				{constants.RouterAdvertisedAddressVarName, constants.RouterAdvertisedAddressVarDescription, data.Router.Edge.AdvertisedHost},
				{constants.RouterResolverVarName, constants.RouterResolverVarDescription, data.Router.Edge.Resolver},
				{constants.RouterDnsSvcIpRangeVarName, constants.RouterDnsSvcIpRangeVarDescription, data.Router.Edge.DnsSvcIpRange},
				{constants.RouterCsrCVarName, constants.RouterCsrCVarDescription, data.Router.Edge.CsrC},
				{constants.RouterCsrSTVarName, constants.RouterCsrSTVarDescription, data.Router.Edge.CsrST},
				{constants.RouterCsrLVarName, constants.RouterCsrLVarDescription, data.Router.Edge.CsrL},
				{constants.RouterCsrOVarName, constants.RouterCsrOVarDescription, data.Router.Edge.CsrO},
				{constants.RouterCsrOUVarName, constants.RouterCsrOUVarDescription, data.Router.Edge.CsrOU},
				{constants.RouterCsrSansDnsVarName, constants.RouterCsrSansDnsVarDescription, data.Router.Edge.CsrSans},
			}

			// Setup logging
			var logOut *os.File
			// Figure out the correct comment prefix and variable declaration command
			if runtime.GOOS == "windows" {
				environmentOptions.OSCommentPrefix = "rem"
				if !environmentOptions.DisableOSVarDeclare {
					environmentOptions.OSVarDeclare = "SET"
				}
			} else {
				environmentOptions.OSCommentPrefix = "#"
				if !environmentOptions.DisableOSVarDeclare {
					environmentOptions.OSVarDeclare = "export"
				}
			}
			if environmentOptions.Verbose {
				logrus.SetLevel(logrus.DebugLevel)
				// Only print log to stdout if not printing config to stdout
				if strings.ToLower(environmentOptions.Output) != "stdout" {
					logOut = os.Stdout
				} else {
					logOut = os.Stderr
				}
				logrus.SetOutput(logOut)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			environmentOptions.Cmd = cmd
			environmentOptions.Args = args
			return environmentOptions.run()
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			// Reset log output after run completes
			logrus.SetOutput(os.Stdout)
		},
	}

	var sb strings.Builder

	sb.WriteString("Creates an env file for generating a controller or router config YAML." +
		"\nThe following can be set to override defaults:\n")
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.HomeVarName, constants.HomeVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.NetworkNameVarName, constants.NetworkNameVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.PkiCtrlCertVarName, constants.PkiCtrlCertVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.PkiCtrlServerCertVarName, constants.PkiCtrlServerCertVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.PkiCtrlKeyVarName, constants.PkiCtrlKeyVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.PkiCtrlCAVarName, constants.PkiCtrlCAVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlDatabaseFileVarName, constants.CtrlDatabaseFileVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlBindAddressVarName, constants.CtrlBindAddressVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlAdvertisedAddressVarName, constants.CtrlAdvertisedAddressVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlEdgeAltAdvertisedAddressVarName, constants.CtrlEdgeAltAdvertisedAddressVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlAdvertisedPortVarName, constants.CtrlAdvertisedPortVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlConsoleLocationVarName, constants.CtrlConsoleLocationVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlEdgeBindAddressVarName, constants.CtrlEdgeBindAddressVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlEdgeAdvertisedPortVarName, constants.CtrlEdgeAdvertisedPortVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.PkiSignerCertVarName, constants.PkiSignerCertVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.PkiSignerKeyVarName, constants.PkiSignerKeyVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlEdgeIdentityEnrollmentDurationVarName, constants.CtrlEdgeIdentityEnrollmentDurationVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlEdgeRouterEnrollmentDurationVarName, constants.CtrlEdgeRouterEnrollmentDurationVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlEdgeAdvertisedAddressVarName, constants.CtrlEdgeAdvertisedAddressVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlPkiEdgeCertVarName, constants.CtrlPkiEdgeCertVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlPkiEdgeServerCertVarName, constants.CtrlPkiEdgeServerCertVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlPkiEdgeKeyVarName, constants.CtrlPkiEdgeKeyVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.CtrlPkiEdgeCAVarName, constants.CtrlPkiEdgeCAVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.PkiAltServerCertVarName, constants.PkiAltServerCertVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.PkiAltServerKeyVarName, constants.PkiAltServerKeyVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterNameVarName, constants.RouterNameVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterPortVarName, constants.RouterPortVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterListenerBindPortVarName, constants.RouterListenerBindPortVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterIdentityCertVarName, constants.RouterIdentityCertVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterIdentityServerCertVarName, constants.RouterIdentityServerCertVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterIdentityKeyVarName, constants.RouterIdentityKeyVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterIdentityCAVarName, constants.RouterIdentityCAVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterIPOverrideVarName, constants.RouterIPOverrideVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterAdvertisedAddressVarName, constants.RouterAdvertisedAddressVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterResolverVarName, constants.RouterResolverVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterDnsSvcIpRangeVarName, constants.RouterDnsSvcIpRangeVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterCsrCVarName, constants.RouterCsrCVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterCsrSTVarName, constants.RouterCsrSTVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterCsrLVarName, constants.RouterCsrLVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterCsrOVarName, constants.RouterCsrOVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterCsrOUVarName, constants.RouterCsrOUVarDescription)
	fmt.Fprintf(&sb, "%-40s %-50s\n", constants.RouterCsrSansDnsVarName, constants.RouterCsrSansDnsVarDescription)

	cmd.Long = sb.String()

	environmentOptions.addFlags(cmd)
	environmentOptions.addCreateFlags(cmd)

	return cmd
}

// run implements the command
func (options *CreateConfigEnvironmentOptions) run() error {

	tmpl, err := template.New("environment-config").Parse(environmentConfigTemplate)
	if err != nil {
		return err
	}

	var f *os.File
	if strings.ToLower(options.Output) != "stdout" {
		// Check if the path exists, fail if it doesn't
		basePath := filepath.Dir(options.Output) + "/"
		if _, err := os.Stat(filepath.Dir(basePath)); os.IsNotExist(err) {
			logrus.Fatalf("Provided path: [%s] does not exist\n", basePath)
			return err
		}

		f, err = os.Create(options.Output)
		logrus.Debugf("Created output file: %s", options.Output)
		if err != nil {
			return errors.Wrapf(err, "unable to create config file: %s", options.Output)
		}
	} else {
		f = os.Stdout
	}
	defer func() { _ = f.Close() }()

	if err := tmpl.Execute(f, options); err != nil {
		return errors.Wrap(err, "unable to execute template")
	}

	logrus.Debugf("Environment configuration file generated successfully and written to: %s", options.Output)

	return nil
}
