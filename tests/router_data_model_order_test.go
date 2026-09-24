//go:build apitests

package tests

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hanzozt/channel/v4"
	"github.com/hanzozt/zt/v2/common/ctrlchan"
	"github.com/hanzozt/zt/v2/common/pb/edge_ctrl_pb"
	"github.com/hanzozt/zt/v2/controller/xt_smartrouting"
)

// Change sets the controller publishes back to back reach the router in order, so it holds
// every one, while its control channel holds a high-priority underlay beside the default one.
// The router applies change sets by index and drops one that arrives after a later one, so
// a burst split across the two underlays loses whatever is overtaken. A router renewing its
// subscription is sent every change set since its index in one such burst.
func Test_RouterDataModel_ChangeSetBurstArrivesInOrder(t *testing.T) {
	ctx := NewTestContext(t)
	defer ctx.Teardown()
	ctx.StartServer()
	ctx.RequireAdminManagementApiLogin()

	router := ctx.CreateEnrollAndStartEdgeRouter()
	requireHighPriorityUnderlay(ctx, router.GetNetworkControllers().AnyChannel())

	sender := ctx.EdgeController.AppEnv.Broker.GetRouterDataModel()
	const count = 200
	index := sender.CurrentIndex()
	for i := range count {
		index++
		sender.ApplyChangeSet(&edge_ctrl_pb.DataState_ChangeSet{
			Index: index,
			Changes: []*edge_ctrl_pb.DataState_Event{{
				Action: edge_ctrl_pb.DataState_Create,
				Model: &edge_ctrl_pb.DataState_Event_Identity{
					Identity: &edge_ctrl_pb.DataState_Identity{Id: fmt.Sprintf("burst-%d", i), Name: fmt.Sprintf("burst-%d", i)},
				},
			}},
		})
	}

	rdm := router.GetRouterDataModel()
	ctx.Req.Eventually(func() bool {
		return rdm.CurrentIndex() == index
	}, 5*time.Second, 20*time.Millisecond, "router should reach index %d", index)
	for i := range count {
		_, found := rdm.Identities.Get(fmt.Sprintf("burst-%d", i))
		ctx.Req.True(found, "router lost change set %d of %d", i+1, count)
	}
}

// Identities created back to back reach the router with their dial policy, and each dials
// within seconds, while the router's control channel holds a high-priority underlay beside
// the default one. The router applies data model change sets by index and drops one that
// arrives after a later one, so a change set that overtakes its predecessor loses it.
func Test_RouterDataModel_NewIdentitiesDial(t *testing.T) {
	ctx := NewTestContext(t)
	defer ctx.Teardown()
	ctx.StartServer()
	ctx.RequireAdminManagementApiLogin()

	service := ctx.AdminManagementSession.RequireNewServiceAccessibleToAll(xt_smartrouting.Name)
	router := ctx.CreateEnrollAndStartEdgeRouter()

	requireHighPriorityUnderlay(ctx, router.GetNetworkControllers().AnyChannel())

	_, host := ctx.AdminManagementSession.RequireCreateSdkContext()
	defer host.Close()
	listener, err := host.Listen(service.Name)
	ctx.Req.NoError(err)
	defer func() { _ = listener.Close() }()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	const count = 20
	ids := make(chan string, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids <- ctx.AdminManagementSession.RequireNewIdentityWithOtt(false).Id
		}()
	}
	wg.Wait()
	close(ids)

	rdm := router.GetRouterDataModel()
	for id := range ids {
		ctx.Req.Eventually(func() bool {
			access, err := rdm.GetServiceAccessPolicies(id, service.Id, edge_ctrl_pb.PolicyType_DialPolicy)
			return err == nil && len(access.Policies) > 0
		}, 2*time.Second, 20*time.Millisecond, "identity %s should reach the router with its dial policy", id)
	}

	for range 3 {
		_, dialer := ctx.AdminManagementSession.RequireCreateSdkContext()
		start := time.Now()
		conn, err := dialer.Dial(service.Name)
		ctx.Req.NoError(err, "a new identity should dial at once")
		t.Logf("new identity dialed in %v", time.Since(start))
		_ = conn.Close()
		dialer.Close()
	}
}

// requireHighPriorityUnderlay waits until the router's control channel holds a high-priority
// underlay beside its default one, which it dials a few seconds after connecting.
func requireHighPriorityUnderlay(ctx *TestContext, ch channel.Channel) {
	ctx.Req.Eventually(func() bool {
		multi, ok := ch.(interface{ GetUnderlayCountsByType() map[string]int })
		return ok && multi.GetUnderlayCountsByType()[ctrlchan.ChannelTypeHighPriority] > 0
	}, 15*time.Second, 100*time.Millisecond, "router should hold a high-priority control underlay")
}
