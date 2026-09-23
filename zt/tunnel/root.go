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

package tunnel

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hanzozt/sdk-golang/zt/sdkinfo"
	"github.com/hanzozt/zt/v2/zt/cmd/common"
	"github.com/hanzozt/zt/v2/zt/util"

	"github.com/hanzozt/agent"
	"github.com/hanzozt/sdk-golang/zt"
	"github.com/hanzozt/zt/v2/common/version"
	"github.com/hanzozt/zt/v2/tunnel"
	"github.com/hanzozt/zt/v2/tunnel/dns"
	"github.com/hanzozt/zt/v2/tunnel/entities"
	"github.com/hanzozt/zt/v2/tunnel/intercept"
	"github.com/michaelquigley/pfxlog"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	svcPollRateFlag     = "svcPollRate"
	resolverCfgFlag     = "resolver"
	dnsSvcIpRangeFlag   = "dnsSvcIpRange"
	dnsUpstreamFlag     = "dnsUpstream"
	dnsUnanswerableFlag = "dnsUnanswerable"
)

var hostSpecificCmds []func() *cobra.Command

func NewTunnelCmd(legacy bool) *cobra.Command {
	var root = &cobra.Command{
		Use:              "tunnel",
		Short:            "ZT Tunnel",
		PersistentPreRun: rootPreRun,
		Hidden:           true,
	}

	root.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose mode")
	root.PersistentFlags().StringP("identity", "i", "", "Path to JSON file that contains an enrolled identity")
	root.PersistentFlags().String("identity-dir", "", "Path to directory file that contains one or more enrolled identities")
	root.PersistentFlags().String("controller", "", "Controller URL to log in to by IAM token, e.g. https://zt-api.hanzo.ai")
	root.PersistentFlags().String("token-command", "", "Command, run without a shell, that prints a Hanzo IAM access token to log in with instead of an identity file")
	root.MarkFlagsRequiredTogether("controller", "token-command")
	root.MarkFlagsMutuallyExclusive("token-command", "identity", "identity-dir")
	root.PersistentFlags().Uint(svcPollRateFlag, 15, "Set poll rate for service updates (seconds). Polling in proxy mode is disabled unless this value is explicitly set")
	root.PersistentFlags().StringP(resolverCfgFlag, "r", "udp://127.0.0.1:53", "Resolver configuration")
	root.PersistentFlags().String(dnsUpstreamFlag, "", "Upstream DNS server for recursive queries (e.g., udp://10.96.0.10:53 or tcp://8.8.8.8:53)")
	root.PersistentFlags().String(dnsUnanswerableFlag, "", "Disposition for unanswerable DNS queries (timeout|servfail|refused, default: refused)")
	root.PersistentFlags().StringVar(&logFormatter, "log-formatter", "", "Specify log formatter [json|pfxlog|text]")
	root.PersistentFlags().StringP(dnsSvcIpRangeFlag, "d", "100.64.0.1/10", "cidr to use when assigning IPs to unresolvable intercept hostnames")
	root.PersistentFlags().BoolVar(&cliAgentEnabled, "cli-agent", true, "Enable/disable CLI Agent (enabled by default)")
	root.PersistentFlags().StringVar(&cliAgentAddr, "cli-agent-addr", "", "Specify where CLI Agent should listen (ex: unix:/tmp/myfile.sock or tcp:127.0.0.1:10001)")
	root.PersistentFlags().StringVar(&cliAgentAlias, "cli-agent-alias", "", "Alias which can be used by zt agent commands to find this instance")
	root.PersistentFlags().BoolVar(&sdkFlowControl, "sdk-flow-control", true, "enables sdk flow control")
	root.PersistentFlags().Uint8Var(&maxDefaultConnections, "default-connections", 2, "sets the desired number of default connections")
	root.PersistentFlags().Uint8Var(&maxControlConnections, "control-connections", 1, "sets the desired number of control connections")
	root.AddCommand(NewHostCmd())
	root.AddCommand(NewProxyCmd())
	for _, cmdF := range hostSpecificCmds {
		cmd := cmdF()
		if cmd.Name() != "run" || legacy { // only include run in 'zt tunnel' tree
			root.AddCommand(cmdF())
		}
	}

	versionCmd := common.NewVersionCmd()
	versionCmd.Hidden = true
	versionCmd.Deprecated = "use 'zt version' instead of 'zt router version'"
	root.AddCommand(versionCmd)

	return root
}

var interceptor intercept.Interceptor
var logFormatter string
var cliAgentEnabled bool
var cliAgentAddr string
var cliAgentAlias string
var sdkFlowControl bool
var maxDefaultConnections uint8
var maxControlConnections uint8

func rootPreRun(cmd *cobra.Command, _ []string) {
	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		println("err")
	}
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	switch logFormatter {
	case "pfxlog":
		logrus.SetFormatter(pfxlog.NewFormatter(pfxlog.DefaultOptions().StartingToday()))
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: "2006-01-02T15:04:05.000Z"})
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	default:
		// let logrus do its own thing
	}
	util.LogReleaseVersionCheck()
}

func rootPostRun(cmd *cobra.Command, _ []string) {
	log := pfxlog.Logger()

	if cliAgentEnabled {
		// don't use the agent's shutdown handler. it calls os.Exit on SIGINT
		// which interferes with the servicePoller shutdown
		cleanup := false
		err := agent.Listen(agent.Options{
			Addr:            cliAgentAddr,
			ShutdownCleanup: &cleanup,
			AppAlias:        cliAgentAlias,
		})

		if err != nil {
			log.WithError(err).Error("failed to start CLI agent")
		}
	}

	sdkinfo.SetApplication("zt-tunnel", version.GetVersion())

	resolverConfig := cmd.Flag(resolverCfgFlag).Value.String()
	upstreamConfig := cmd.Flag(dnsUpstreamFlag).Value.String()
	unansweredDisposition, _ := cmd.Flags().GetString(dnsUnanswerableFlag)
	resolver, err := dns.NewResolver(resolverConfig, upstreamConfig, unansweredDisposition)
	if err != nil {
		log.WithError(err).Fatal("failed to start DNS resolver")
	}

	serviceListenerGroup := intercept.NewServiceListenerGroup(interceptor, resolver)
	dnsIpRange, _ := cmd.Flags().GetString(dnsSvcIpRangeFlag)
	if err := intercept.SetDnsInterceptIpRange(dnsIpRange); err != nil {
		log.Fatalf("invalid dns service IP range %s: %v", dnsIpRange, err)
	}

	if command := cmd.Flag("token-command").Value.String(); command != "" {
		controller := strings.TrimSuffix(cmd.Flag("controller").Value.String(), "/")
		creds, err := newIAM(command)
		if err != nil {
			log.Fatal(err)
		}
		if creds.CaPool, err = trust(controller); err != nil {
			log.Fatal(err)
		}
		log.Infof("logging in to %s by IAM token", controller)
		start(cmd, serviceListenerGroup, &zt.Config{ZtAPI: controller + "/edge/client/v1", Credentials: creds})
	} else if idDir := cmd.Flag("identity-dir").Value.String(); idDir != "" {
		files, err := os.ReadDir(idDir)
		if err != nil {
			log.Fatalf("failed to scan directory %s: %v", idDir, err)
		}

		for _, file := range files {
			if filepath.Ext(file.Name()) == ".json" {
				fn, err := filepath.Abs(filepath.Join(idDir, file.Name()))
				if err != nil {
					log.Fatalf("failed to listing file %s: %v", file.Name(), err)
				}
				go startIdentity(cmd, serviceListenerGroup, fn)
			}
		}
	} else {
		identityJson := cmd.Flag("identity").Value.String()
		startIdentity(cmd, serviceListenerGroup, identityJson)
	}

	serviceListenerGroup.WaitForShutdown()

	if cliAgentEnabled {
		agent.Close()
	}
}

func startIdentity(cmd *cobra.Command, serviceListenerGroup *intercept.ServiceListenerGroup, identityJson string) {
	log := pfxlog.Logger()

	log.Infof("loading identity: %v", identityJson)
	ztCfg, err := zt.NewConfigFromFile(identityJson)
	if err != nil {
		log.Fatalf("failed to load zt configuration from %s: %v", identityJson, err)
	}
	start(cmd, serviceListenerGroup, ztCfg)
}

func start(cmd *cobra.Command, serviceListenerGroup *intercept.ServiceListenerGroup, ztCfg *zt.Config) {
	log := pfxlog.Logger()

	ztCfg.ConfigTypes = []string{
		entities.ClientConfigV1,
		entities.ServerConfigV1,
		entities.InterceptV1,
		entities.HostConfigV1,
		entities.HostConfigV2,
		entities.ProxyV1,
	}

	ztCfg.MaxControlConnections = uint32(maxControlConnections)
	ztCfg.MaxDefaultConnections = uint32(maxDefaultConnections)

	serviceListener := serviceListenerGroup.NewServiceListener()
	svcPollRate, _ := cmd.Flags().GetUint(svcPollRateFlag)
	options := &zt.Options{
		RefreshInterval: time.Duration(svcPollRate) * time.Second,
		OnContextReady: func(ctx zt.Context) {
			serviceListener.HandleProviderReady(tunnel.NewContextProvider(ctx))
		},
		OnServiceUpdate: serviceListener.HandleServicesChange,
		EdgeRouterUrlFilter: func(url string) bool {
			return strings.HasPrefix(url, "tls:")
		},
	}

	rootPrivateContext, err := zt.NewContextWithOpts(ztCfg, options)

	if err != nil {
		pfxlog.Logger().WithError(err).Fatal("could not create zt sdk context")
	}

	for {
		if err = rootPrivateContext.Authenticate(); err != nil {
			log.WithError(err).Error("failed to authenticate")
			time.Sleep(30 * time.Second)
		} else {
			return
		}
	}
}
