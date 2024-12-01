package cluster_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/anserdsg/ratecat/v1/internal/limit"
	"github.com/anserdsg/ratecat/v1/internal/testutil"
)

func BenchmarkCluster_GetNodeMembers(b *testing.B) {
	ctx := context.Background()
	testutil.SetupBenchmarkEnv()

	clusterCount := 3
	clusterInfos := testutil.InitClusterServers(nil, ctx, clusterCount)
	for _, cluster := range clusterInfos {
		os.RemoveAll(cluster.StateDir)
		err := cluster.StartAsync(ctx)
		if err != nil {
			b.FailNow()
		}
		defer os.RemoveAll(cluster.StateDir)
		defer cluster.Stop(ctx) //nolint:errcheck
	}

	ready := clusterInfos.WaitClusterReady()
	if !ready {
		b.FailNow()
	}

	leader := clusterInfos.GetLeaderNode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		members := leader.ResMgr.GetNodeMembers()
		if members == nil || len(members) != clusterCount {
			b.FailNow()
		}
	}
	b.StopTimer()
}

func BenchmarkCluster_PutResource(b *testing.B) {
	ctx := context.Background()
	testutil.SetupBenchmarkEnv()

	clusterCount := 3
	clusterInfos := testutil.InitClusterServers(nil, ctx, clusterCount)
	for _, cluster := range clusterInfos {
		os.RemoveAll(cluster.StateDir)
		err := cluster.StartAsync(ctx)
		if err != nil {
			b.FailNow()
		}
		defer os.RemoveAll(cluster.StateDir)
		defer cluster.Stop(ctx) //nolint:errcheck
	}

	ready := clusterInfos.WaitClusterReady()
	if !ready {
		b.FailNow()
	}

	leader := clusterInfos.GetLeaderNode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter := limit.NewTokenBucketLimiter(float64(1000+i), uint32(10+i))
		_, err := leader.ResMgr.PutResource(fmt.Sprintf("benchmark_%d", i), limiter)
		if err != nil {
			b.FailNow()
		}
	}
	b.StopTimer()
}

func BenchmarkCluster_MeowCmd(b *testing.B) {
	ctx := context.Background()
	testutil.SetupBenchmarkEnv()

	clusterCount := 3
	clusterInfos := testutil.InitClusterServers(nil, ctx, clusterCount)
	for _, cluster := range clusterInfos {
		os.RemoveAll(cluster.StateDir)
		err := cluster.StartAsync(ctx)
		if err != nil {
			b.FailNow()
		}
		defer os.RemoveAll(cluster.StateDir)
		defer cluster.Stop(ctx) //nolint:errcheck
	}

	ready := clusterInfos.WaitClusterReady()
	if !ready {
		b.FailNow()
	}

	leader := clusterInfos.GetLeaderNode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := leader.CmdDisp.Meow(nil)
		if err != nil {
			b.FailNow()
		}
	}
	b.StopTimer()
}
