package cluster_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/limit"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/anserdsg/ratecat/v1/internal/testutil"
	"github.com/stretchr/testify/require"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

func TestMain(m *testing.M) {
	err := config.Init()
	if err != nil {
		panic(err)
	}

	cfg := config.GetDefault()
	cfg.Env = "test"
	cfg.LogLevel = "info"
	cfg.MultiCore = true

	logger := logging.Init()
	if logger == nil {
		panic(errors.New("Failed to initial logger"))
	}

	exitCode := m.Run()
	os.Exit(exitCode)
}

func TestCluster_Single(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()

	resCount := 10000
	clusterInfos := testutil.InitClusterServers(t, ctx, 1)
	for _, info := range clusterInfos {
		os.RemoveAll(info.StateDir)
		require.NoError(info.StartAsync(ctx))
		defer os.RemoveAll(info.StateDir)
		defer info.Stop(ctx) //nolint:errcheck
	}

	require.True(clusterInfos.WaitClusterReady())

	for _, info := range clusterInfos {
		if !info.ResMgr.IsNodeLeader() {
			continue
		}

		start := time.Now()
		for i := 1; i <= resCount; i++ {
			limiter := limit.NewTokenBucketLimiter(float64(1000+i), uint32(10+i))
			res, err := info.ResMgr.PutResource(fmt.Sprintf("test%d", i), limiter)
			require.NoError(err)
			require.NotNil(res)
		}
		elapsed := time.Since(start)
		rps := float64((resCount * int(time.Second))) / float64(elapsed.Nanoseconds())
		fmt.Printf("PutResource rps=%.2f\n", rps)
		for i := 1; i <= resCount; i++ {
			res, ok := info.ResMgr.GetResource(fmt.Sprintf("test%d", i))
			require.True(ok)
			require.NotNil(res)
		}
	}
}

func TestCluster_Recover(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()

	resCount := 100
	clusterCount := 5
	clusterInfos := testutil.InitClusterServers(t, ctx, clusterCount)
	for _, info := range clusterInfos {
		os.RemoveAll(info.StateDir)
		require.NoError(info.StartAsync(ctx))
	}

	require.True(clusterInfos.WaitClusterReady())

	for _, info := range clusterInfos {
		if !info.ResMgr.IsNodeLeader() {
			continue
		}

		for i := 1; i <= resCount; i++ {
			limiter := limit.NewTokenBucketLimiter(float64(1000+i), uint32(10+i))
			res, err := info.ResMgr.PutResource(fmt.Sprintf("test%d", i), limiter)
			require.NoError(err)
			require.NotNil(res)
		}

		for i := resCount / 2; i <= resCount; i++ {
			err := info.ResMgr.RemoveResource(fmt.Sprintf("test%d", i))
			require.NoError(err)
		}
	}

	fmt.Printf("\n\n==== stop all servers ====\n\n")
	// stop all servers
	for _, info := range clusterInfos {
		require.NoError(info.Stop(ctx))
	}

	// restart all servers
	for _, info := range clusterInfos {
		require.NoError(info.StartAsync(ctx))
		defer os.RemoveAll(info.StateDir)
		defer info.Stop(ctx) //nolint:errcheck
	}

	require.True(clusterInfos.WaitClusterReady())

	// check resource in all clusters
	for _, info := range clusterInfos {
		for i := 1; i < resCount/2; i++ {
			resName := fmt.Sprintf("test%d", i)
			res, ok := info.ResMgr.GetResource(resName)
			require.True(ok)
			require.NotNil(res)
			limiter, ok := res.Limiter.ToLimiterPB().(*pb.TokenBucketLimiter)
			require.True(ok)
			require.NotNil(limiter)
			require.Equal(float64(1000+i), limiter.Rate)
			require.Equal(uint32(10+i), limiter.Burst)
		}
	}
}

func TestCluster_ForwardCmd(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	clusterCount := 3
	clusterInfos := testutil.InitClusterServers(nil, ctx, clusterCount)
	for _, info := range clusterInfos {
		os.RemoveAll(info.StateDir)
		require.NoError(info.StartAsync(ctx))
		defer os.RemoveAll(info.StateDir)
		defer info.Stop(ctx) //nolint:errcheck
	}

	require.True(clusterInfos.WaitClusterReady())

	limiter := limit.NewTokenBucketLimiter(100, 10)
	for i, info := range clusterInfos {
		for j := 0; j < 100; j++ {
			resName := fmt.Sprintf("test_%d_%d", i, j)
			res, err := info.ResMgr.PutResource(resName, limiter)
			require.NoError(err)
			require.NotNil(res)

			// check if resource exists in all nodes
			for _, c := range clusterInfos {
				cres, ok := c.ResMgr.GetResource(resName)
				require.True(ok)
				require.NotNil(cres)
				require.Equal(res.NodeID, cres.NodeID)
			}
		}
	}
}

func TestCluster_ResourceBalancing(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	clusterCount := 3
	clusterInfos := testutil.InitClusterServers(nil, ctx, clusterCount)
	for _, info := range clusterInfos {
		os.RemoveAll(info.StateDir)
		require.NoError(info.StartAsync(ctx))
		defer os.RemoveAll(info.StateDir)
		defer info.Stop(ctx) //nolint:errcheck
	}

	require.True(clusterInfos.WaitClusterReady())

	leader := clusterInfos.GetLeaderNode()
	limiter := limit.NewTokenBucketLimiter(100, 10)

	res1, err := leader.ResMgr.PutResource("test1", limiter)
	require.NoError(err)
	require.NotNil(res1)

	node1 := clusterInfos.GetNode(res1.NodeID)
	require.NotNil(node1)
	for i := 0; i < 10; i++ {
		resp, err := node1.CmdDisp.AllowPbCmd(&pb.AllowCmd{Resource: "test1", Events: 1})
		require.Nil(err)
		require.NotNil(resp)
	}
	node1.Metrics.AddAPICmdCountN(10)
	_ = node1.Metrics.ReportAPICmdRPS(time.Second)
	leader.ResMgr.UpdateNodeLoading()

	res2, err := leader.ResMgr.PutResource("test2", limiter)
	require.NoError(err)
	require.NotNil(res2)

	node2 := clusterInfos.GetNode(res2.NodeID)
	require.NotNil(node2)
	for i := 0; i < 20; i++ {
		resp, err := node2.CmdDisp.AllowPbCmd(&pb.AllowCmd{Resource: "test2", Events: 1})
		require.Nil(err)
		require.NotNil(resp)
	}
	// test if allow cmd forwareds to node 2
	resp, err := node1.CmdDisp.AllowPbCmd(&pb.AllowCmd{Resource: "test2", Events: 1})
	require.Nil(err)
	require.Equal(node2.ResMgr.CurNodeID(), resp.NodeId)

	node2.Metrics.AddAPICmdCountN(20)
	_ = node2.Metrics.ReportAPICmdRPS(time.Second)
	leader.ResMgr.UpdateNodeLoading()

	res3, err := leader.ResMgr.PutResource("test3", limiter)
	require.NoError(err)
	require.NotNil(res3)

	node3 := clusterInfos.GetNode(res3.NodeID)
	require.NotNil(node3)
	for i := 0; i < 30; i++ {
		resp, err := node3.CmdDisp.AllowPbCmd(&pb.AllowCmd{Resource: "test3", Events: 1})
		require.Nil(err)
		require.NotNil(resp)
	}
	node3.Metrics.AddAPICmdCountN(30)
	_ = node3.Metrics.ReportAPICmdRPS(time.Second)
	leader.ResMgr.UpdateNodeLoading()

	// test whether three resources are allocated on three different nodes
	require.NotEqual(res1.NodeID, res2.NodeID)
	require.NotEqual(res1.NodeID, res3.NodeID)
	require.NotEqual(res2.NodeID, res3.NodeID)
}

func TestCluster_NodeInfo(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	clusterCount := 3
	clusterInfos := testutil.InitClusterServers(nil, ctx, clusterCount)
	for _, info := range clusterInfos {
		os.RemoveAll(info.StateDir)
		require.NoError(info.StartAsync(ctx))
		defer os.RemoveAll(info.StateDir)
		defer info.Stop(ctx) //nolint:errcheck
	}

	require.True(clusterInfos.WaitClusterReady())

	for _, info := range clusterInfos {
		nodeInfo, err := info.ResMgr.GetCurNodeInfo()
		require.NoError(err)
		require.NotNil(nodeInfo)
		apiEnpointURL, err := config.ParseURL(nodeInfo.ApiEndpoint)
		require.NoError(err)
		require.Equal(info.APIServerHost, apiEnpointURL.Host)
	}
}

func TestCluster_MoveNodeResources(t *testing.T) {
	// logging.SetLevel(logging.ErrorLevel)
	require := require.New(t)
	ctx := context.Background()

	resCount := 30

	clusterCount := 7
	clusterInfos := testutil.InitClusterServers(nil, ctx, clusterCount)
	for _, info := range clusterInfos {
		os.RemoveAll(info.StateDir)
		require.NoError(info.StartAsync(ctx))
		defer os.RemoveAll(info.StateDir)
		defer info.Stop(ctx) //nolint:errcheck
	}

	require.True(clusterInfos.WaitClusterReady())

	leader := clusterInfos.GetLeaderNode()
	require.NotNilf(leader, "Failed to get leader node")

	memNode := clusterInfos.GetNonLeaderNode()
	require.NotNilf(leader, "Failed to get member node")

	limiter := limit.NewTokenBucketLimiter(100, 10)
	for i := 0; i < resCount; i++ {
		resName := fmt.Sprintf("item%d", i)
		res, err := leader.ResMgr.PutResourceToNode(resName, limiter, memNode.NodeID)
		require.NoError(err)
		require.NotNil(res)
	}
	leader.Metrics.AddAPICmdCountN(10)
	_ = leader.Metrics.ReportAPICmdRPS(time.Second)
	leader.ResMgr.UpdateNodeLoading()

	// stop and remove member node
	err := memNode.Stop(ctx)
	require.NoError(err)
	clusterInfos.RemoveClusterNode(memNode.NodeID)
	time.Sleep(2 * time.Second)

	for i := 0; i < resCount; i++ {
		resName := fmt.Sprintf("item%d", i)
		res, ok := leader.ResMgr.GetResource(resName)
		require.True(ok)
		require.NotNil(res)
		require.NotEqual(memNode.NodeID, res.NodeID)
	}

	// stop and remove leader node
	err = leader.Stop(ctx)
	require.NoError(err)
	clusterInfos.RemoveClusterNode(leader.NodeID)
	time.Sleep(2 * time.Second)

	newLeader := clusterInfos.GetLeaderNode()
	require.NotNilf(newLeader, "Failed to get new leader node")
	for i := 0; i < resCount; i++ {
		resName := fmt.Sprintf("item%d", i)
		res, ok := newLeader.ResMgr.GetResource(resName)
		require.True(ok)
		require.NotNil(res)
		require.NotEqualf(memNode.NodeID, res.NodeID, "The resource node %s id %d should not be member node id %d", resName, res.NodeID, memNode.NodeID)
		require.NotEqualf(leader.NodeID, res.NodeID, "The resource node %s id %d should not be leader node id %d", resName, res.NodeID, leader.NodeID)
	}
}

func TestCluster_Replicate(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	clusterCount := 3
	clusterInfos := testutil.InitClusterServers(nil, ctx, clusterCount)
	for _, info := range clusterInfos {
		os.RemoveAll(info.StateDir)
		info.ClusterServer.StartAsync()
		info.APIServer.StartAsync()
		defer os.RemoveAll(info.StateDir)
		defer info.APIServer.Stop(ctx)     //nolint:errcheck
		defer info.ClusterServer.Stop(ctx) //nolint:errcheck
	}

	require.True(clusterInfos.WaitClusterReady())

	leader := clusterInfos.GetLeaderNode()
	memNode := clusterInfos.GetNonLeaderNode()
	limiter := limit.NewTokenBucketLimiter(100, 10)

	for i := 0; i < 2; i++ {
		resName := fmt.Sprintf("item%d", i)
		res1, err := leader.ResMgr.PutResourceToNode(resName, limiter, memNode.NodeProber.CurNodeID())
		require.NoError(err)
		require.NotNil(res1)
	}

	_ = memNode.APIServer.Stop(ctx)
	_ = memNode.ClusterServer.Stop(ctx)

	fmt.Printf("==============\n")
	time.Sleep(2 * time.Second)
	fmt.Printf("==============\n")
	for i := 0; i < 3; i++ {
		resName := fmt.Sprintf("test_replicate%d", i)
		res1, err := leader.ResMgr.PutResource(resName, limiter)
		require.NoError(err)
		require.NotNil(res1)
	}

	time.Sleep(30 * time.Second)
}
