package main

import (
	"embed"
	_ "embed"
	"os"
	"path"
	"strings"
	"time"

	"github.com/hanzozt/fablab"
	"github.com/hanzozt/fablab/kernel/lib/actions"
	"github.com/hanzozt/fablab/kernel/lib/actions/component"
	"github.com/hanzozt/fablab/kernel/lib/actions/host"
	"github.com/hanzozt/fablab/kernel/lib/actions/semaphore"
	"github.com/hanzozt/fablab/kernel/lib/binding"
	"github.com/hanzozt/fablab/kernel/lib/runlevel/0_infrastructure/aws_ssh_key"
	"github.com/hanzozt/fablab/kernel/lib/runlevel/0_infrastructure/semaphore"
	"github.com/hanzozt/fablab/kernel/lib/runlevel/0_infrastructure/terraform"
	distribution "github.com/hanzozt/fablab/kernel/lib/runlevel/3_distribution"
	"github.com/hanzozt/fablab/kernel/lib/runlevel/3_distribution/rsync"
	aws_ssh_key2 "github.com/hanzozt/fablab/kernel/lib/runlevel/6_disposal/aws_ssh_key"
	"github.com/hanzozt/fablab/kernel/lib/runlevel/6_disposal/terraform"
	"github.com/hanzozt/fablab/kernel/model"
	"github.com/hanzozt/fablab/resources"
	"github.com/hanzozt/zt/v2/controller/db"
	"github.com/hanzozt/zt/zttest/models/test_resources"
	"github.com/hanzozt/zt/zttest/ztlab"
	"github.com/hanzozt/zt/zttest/ztlab/actions/edge"
	"github.com/michaelquigley/pfxlog"
	"go.etcd.io/bbolt"
)

func getDbFile() string {
	dbFile := os.Getenv("ZT_DB")
	if dbFile == "" {
		pfxlog.Logger().Fatal("required env var ZT_DB not set")
	}
	return dbFile
}

//go:embed configs
var configResource embed.FS

type scaleStrategy struct{}

func (self scaleStrategy) IsScaled(entity model.Entity) bool {
	return entity.GetType() == model.EntityTypeHost && entity.GetScope().HasTag("scaled")
}

func (self scaleStrategy) GetEntityCount(entity model.Entity) uint32 {
	if entity.GetType() == model.EntityTypeHost && entity.GetScope().HasTag("scaled") {
		return 4
	}
	return 1
}

type dbStrategy struct{}

func (d dbStrategy) ProcessDbModel(tx *bbolt.Tx, m *model.Model, builder *ztlab.ZitiDbBuilder) error {
	return builder.CreateEdgeRouterHosts(tx, m, d)
}

func (d dbStrategy) GetDbFile(*model.Model) string {
	return getDbFile()
}

func (d dbStrategy) GetSite(router *db.EdgeRouter) (string, bool) {
	for _, attr := range router.RoleAttributes {
		if strings.Contains(attr, "Hosted") {
			return "us-west-2b", true
		}
	}
	return "us-west-1c", true
}

func (d dbStrategy) PostProcess(router *db.EdgeRouter, c *model.Component) {
	if router.IsTunnelerEnabled {
		c.Scope.Tags = append(c.Scope.Tags, "tunneler")
	}
	c.Scope.Tags = append(c.Scope.Tags, "edge-router")
	c.Scope.Tags = append(c.Scope.Tags, "pre-created")
	c.Host.InstanceType = "c5.large"
}

var m = &model.Model{
	Id: "router-test",
	Scope: model.Scope{
		Defaults: model.Variables{
			"environment": "router-test",
			"credentials": model.Variables{
				"ssh": model.Variables{
					"username": "ubuntu",
				},
				"edge": model.Variables{
					"username": "admin",
					"password": "admin",
				},
			},
			"metrics": model.Variables{
				"influxdb": model.Variables{
					"url": "http://localhost:8086",
					"db":  "zt",
				},
			},
		},
	},
	StructureFactories: []model.Factory{
		model.NewScaleFactoryWithDefaultEntityFactory(scaleStrategy{}),
		&ztlab.ZitiDbBuilder{Strategy: dbStrategy{}},
	},
	Resources: model.Resources{
		resources.Configs:   resources.SubFolder(configResource, "configs"),
		resources.Binaries:  os.DirFS(path.Join(os.Getenv("GOPATH"), "bin")),
		resources.Terraform: test_resources.TerraformResources(),
	},
	Regions: model.Regions{
		"us-east-1": {
			Region: "us-east-1",
			Site:   "us-east-1a",
			Hosts: model.Hosts{
				"ctrl": {
					InstanceType: "c5.large",
					Components: model.Components{
						"ctrl": {
							Type: &ztlab.ControllerType{},
						},
					},
				},
			},
		},
	},

	Actions: model.ActionBinders{
		"bootstrap": model.ActionBinder(func(m *model.Model) model.Action {
			workflow := actions.Workflow()

			workflow.AddAction(component.Stop("*"))
			workflow.AddAction(host.GroupExec("*", 25, "rm -f logs/*"))

			workflow.AddAction(component.Start("#ctrl"))
			workflow.AddAction(semaphore.Sleep(2 * time.Second))

			workflow.AddAction(edge.Login("#ctrl"))

			workflow.AddAction(edge.ReEnrollEdgeRouters(".pre-created", 2))
			return workflow
		}),
		"stop": model.Bind(component.StopInParallel("*", 15)),
		"clean": model.Bind(actions.Workflow(
			component.StopInParallel("*", 15),
			host.GroupExec("*", 25, "rm -f logs/*"),
		)),
		"login": model.Bind(edge.Login("#ctrl")),
	},

	Infrastructure: model.Stages{
		aws_ssh_key.Express(),
		terraform_0.Express(),
		semaphore_0.Ready(90 * time.Second),
	},

	Distribution: model.Stages{
		distribution.DistributeSshKey("*"),
		distribution.Locations("*", "logs"),
		rsync.RsyncStaged(),
		rsync.NewRsyncHost("#ctrl", getDbFile(), "/home/ubuntu/fablab/ctrl.db"),
	},

	Disposal: model.Stages{
		terraform.Dispose(),
		aws_ssh_key2.Dispose(),
	},
}

func main() {
	m.AddActivationActions("stop", "bootstrap")

	model.AddBootstrapExtension(binding.AwsCredentialsLoader)
	model.AddBootstrapExtension(aws_ssh_key.KeyManager)

	fablab.InitModel(m)
	fablab.Run()
}
