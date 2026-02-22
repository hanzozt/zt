/*
	(c) Copyright NetFoundry Inc.

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

package smoke

import (
	"embed"
	"os"
	"strings"
	"time"

	"github.com/michaelquigley/pfxlog"
	"github.com/hanzozt/fablab/kernel/lib/actions/component"
	"github.com/hanzozt/fablab/kernel/lib/binding"
	"github.com/hanzozt/fablab/kernel/lib/runlevel/0_infrastructure/aws_ssh_key"
	semaphore "github.com/hanzozt/fablab/kernel/lib/runlevel/0_infrastructure/semaphore"
	terraformInit "github.com/hanzozt/fablab/kernel/lib/runlevel/0_infrastructure/terraform"
	distribution "github.com/hanzozt/fablab/kernel/lib/runlevel/3_distribution"
	"github.com/hanzozt/fablab/kernel/lib/runlevel/3_distribution/rsync"
	awsSshKeyDispose "github.com/hanzozt/fablab/kernel/lib/runlevel/6_disposal/aws_ssh_key"
	"github.com/hanzozt/fablab/kernel/lib/runlevel/6_disposal/terraform"
	"github.com/hanzozt/fablab/kernel/model"
	"github.com/hanzozt/fablab/resources"
	"github.com/hanzozt/zt/zttest/models/smoke/actions"
	"github.com/hanzozt/zt/zttest/models/test_resources"
	"github.com/hanzozt/zt/zttest/ztlab"
	"github.com/hanzozt/zt/zttest/ztlab/actions/edge"
)

const ZitiEdgeTunnelVersion = "v1.5.10"
const ZitiCtrlVersion = ""
const ZitiRouterVersion = ""

//go:embed configs
var configResource embed.FS

func getUniqueId() string {
	if runId := os.Getenv("GITHUB_RUN_ID"); runId != "" {
		return "-" + runId + "." + os.Getenv("GITHUB_RUN_ATTEMPT")
	}
	return "-" + os.Getenv("USER")
}

var Model = &model.Model{
	Id: "smoketest",
	Scope: model.Scope{
		Defaults: model.Variables{
			"environment": "smoketest" + getUniqueId(),
			"credentials": model.Variables{
				"aws": model.Variables{
					"managed_key": true,
				},
				"ssh": model.Variables{
					"username": "ubuntu",
				},
				"edge": model.Variables{
					"username": "admin",
					"password": "admin",
				},
			},
		},
	},

	StructureFactories: []model.Factory{
		model.FactoryFunc(func(m *model.Model) error {
			if val, _ := m.GetBoolVariable("ha"); !val {
				for _, host := range m.SelectHosts("component.ha") {
					delete(host.Region.Hosts, host.Id)
				}
			}
			return nil
		}),
	},

	Factories: []model.Factory{
		model.FactoryFunc(func(m *model.Model) error {
			pfxlog.Logger().Infof("environment [%s]", m.MustStringVariable("environment"))
			m.AddActivationActions("stop", "bootstrap", "start")
			return nil
		}),
		model.FactoryFunc(func(m *model.Model) error {
			return m.ForEachHost("*", 1, func(host *model.Host) error {
				if strings.HasPrefix(host.Id, "ctrl") {
					host.InstanceType = "t3.medium"
				} else {
					host.InstanceType = "c5.large"
				}
				return nil
			})
		}),

		model.FactoryFunc(func(m *model.Model) error {
			zetPath, useLocalPath := m.GetStringVariable("local_zet_path")
			return m.ForEachComponent("*", 1, func(c *model.Component) error {
				if c.Type == nil {
					return nil
				}

				if zet, ok := c.Type.(*ztlab.ZitiEdgeTunnelType); ok {
					if useLocalPath {
						zet.Version = ""
						zet.LocalPath = zetPath
					} else {
						zet.Version = ZitiEdgeTunnelVersion
						zet.LocalPath = ""
					}
					zet.InitType(c)
					return nil
				}

				return nil
			})
		}),
	},

	Resources: model.Resources{
		resources.Configs:   resources.SubFolder(configResource, "configs"),
		resources.Terraform: test_resources.TerraformResources(),
	},

	Regions: model.Regions{
		"us-east-1": {
			Region: "us-east-1",
			Site:   "us-east-1a",
			Hosts: model.Hosts{
				"ctrl1": {
					Components: model.Components{
						"ctrl1": {
							Scope: model.Scope{Tags: model.Tags{"ctrl"}},
							Type: &ztlab.ControllerType{
								Version: ZitiCtrlVersion,
							},
						},
					},
				},
				"ctrl2": {
					Components: model.Components{
						"ctrl2": {
							Scope: model.Scope{Tags: model.Tags{"ctrl", "ha"}},
							Type: &ztlab.ControllerType{
								Version: ZitiCtrlVersion,
							},
						},
					},
				},
				"router-east-1": {
					Scope: model.Scope{Tags: model.Tags{"ert-client"}},
					Components: model.Components{
						"router-east-1": {
							Scope: model.Scope{Tags: model.Tags{"edge-router", "terminator", "tunneler", "client"}},
							Type: &ztlab.RouterType{
								Debug:   false,
								Version: ZitiRouterVersion,
							},
						},
						"zcat": {
							Scope: model.Scope{Tags: model.Tags{"sdk-app", "client"}},
							Type:  &ztlab.ZCatType{},
						},
					},
				},
				"router-east-2": {
					Components: model.Components{
						"router-east-2": {
							Scope: model.Scope{Tags: model.Tags{"edge-router", "initiator"}},
							Type: &ztlab.RouterType{
								Debug:   false,
								Version: ZitiRouterVersion,
							},
						},
					},
				},
				"zt-edge-tunnel-client": {
					Scope: model.Scope{Tags: model.Tags{"zet-client"}},
					Components: model.Components{
						"zt-edge-tunnel-client": {
							Scope: model.Scope{Tags: model.Tags{"sdk-app", "client", "zet"}},
							Type: &ztlab.ZitiEdgeTunnelType{
								Version:        ZitiEdgeTunnelVersion,
								VerbosityLevel: 3,
							},
						},
					},
				},
				"zt-tunnel-client": {
					Scope: model.Scope{Tags: model.Tags{"zt-tunnel-client"}},
					Components: model.Components{
						"zt-tunnel-client": {
							Scope: model.Scope{Tags: model.Tags{"zt-tunnel", "sdk-app", "client"}},
							Type:  &ztlab.ZitiTunnelType{},
						},
					},
				},
			},
		},
		"us-west-2": {
			Region: "us-west-2",
			Site:   "us-west-2b",
			Hosts: model.Hosts{
				"ctrl3": {
					Components: model.Components{
						"ctrl3": {
							Scope: model.Scope{Tags: model.Tags{"ctrl", "ha"}},
							Type: &ztlab.ControllerType{
								Version: ZitiCtrlVersion,
							},
						},
					},
				},

				"router-west": {
					Components: model.Components{
						"router-west": {
							Scope: model.Scope{Tags: model.Tags{"edge-router", "tunneler", "host", "ert-host"}},
							Type: &ztlab.RouterType{
								Debug:   false,
								Version: ZitiRouterVersion,
							},
						},
						"echo-server": {
							Scope: model.Scope{Tags: model.Tags{"sdk-app", "service"}},
							Type: &ztlab.EchoServerType{
								BindService: "echo",
								Verbose:     true,
							},
						},
						"iperf-server-ert": {
							Scope: model.Scope{Tags: model.Tags{"iperf", "service", "ert"}},
							Type:  &ztlab.IPerfServerType{},
						},
						"caddy-ert": {
							Scope: model.Scope{Tags: model.Tags{"caddy", "service"}},
							Type: &ztlab.CaddyType{
								Version: "v2.7.6",
							},
						},
					},
				},
				"zt-edge-tunnel-host": {
					Components: model.Components{
						"zt-edge-tunnel-host": {
							Scope: model.Scope{Tags: model.Tags{"sdk-app", "host", "zet-host", "zet"}},
							Type: &ztlab.ZitiEdgeTunnelType{
								Version:        ZitiEdgeTunnelVersion,
								VerbosityLevel: 3,
							},
						},
						"iperf-server-zet": {
							Scope: model.Scope{Tags: model.Tags{"iperf", "service", "zet"}},
							Type:  &ztlab.IPerfServerType{},
						},
						"caddy-zet": {
							Scope: model.Scope{Tags: model.Tags{"caddy", "service"}},
							Type: &ztlab.CaddyType{
								Version: "v2.7.6",
							},
						},
					},
				},
				"zt-tunnel-host": {
					Components: model.Components{
						"zt-tunnel-host": {
							Scope: model.Scope{Tags: model.Tags{"zt-tunnel", "sdk-app", "host", "zt-tunnel-host"}},
							Type: &ztlab.ZitiTunnelType{
								Mode:    ztlab.ZitiTunnelModeHost,
								Verbose: true,
							},
						},
						"iperf-server-zt": {
							Scope: model.Scope{Tags: model.Tags{"iperf", "service", "zt-tunnel"}},
							Type:  &ztlab.IPerfServerType{},
						},
						"caddy-zt": {
							Scope: model.Scope{Tags: model.Tags{"caddy", "service"}},
							Type: &ztlab.CaddyType{
								Version: "v2.7.6",
							},
						},
					},
				},
			},
		},
	},

	Actions: model.ActionBinders{
		"bootstrap": actions.NewBootstrapAction(),
		"start":     actions.NewStartAction(),
		"stop":      model.Bind(component.StopInParallel("*", 15)),
		"login":     model.Bind(edge.Login("#ctrl1")),
		"login2":    model.Bind(edge.Login("#ctrl2")),
		"login3":    model.Bind(edge.Login("#ctrl3")),
		"testZet": model.Bind(model.ActionFunc(func(run model.Run) error {
			out, err := TestFileDownload("zet", ClientCurl, "zet", true, FileSizes[0])
			pfxlog.Logger().WithField("test output", out).Info("test completed")
			return err
		})),
		"testZitiTunnel": model.Bind(model.ActionFunc(func(run model.Run) error {
			out, err := TestFileDownload("zt-tunnel", ClientCurl, "zt-tunnel", true, FileSizes[0])
			pfxlog.Logger().WithField("test output", out).Info("test completed")
			return err
		})),
		"testScenario3": model.Bind(model.ActionFunc(func(run model.Run) error {
			out, err := TestFileDownload("ert", ClientCurl, "zt-tunnel", false, FileSizes[2])
			pfxlog.Logger().WithField("test output", out).Info("test completed")
			return err
		})),
	},

	Infrastructure: model.Stages{
		aws_ssh_key.Express(),
		&terraformInit.Terraform{
			Retries: 3,
			ReadyCheck: &semaphore.ReadyStage{
				MaxWait: 90 * time.Second,
			},
		},
	},

	Distribution: model.Stages{
		distribution.DistributeSshKey("*"),
		rsync.RsyncStaged(),
	},

	Disposal: model.Stages{
		terraform.Dispose(),
		awsSshKeyDispose.Dispose(),
	},
}

func InitBootstrapExtensions() {
	model.AddBootstrapExtension(binding.AwsCredentialsLoader)
	model.AddBootstrapExtension(aws_ssh_key.KeyManager)
}
