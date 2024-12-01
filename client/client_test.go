package client_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/anserdsg/ratecat/v1/client"
	"github.com/anserdsg/ratecat/v1/internal/cmd"
	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/anserdsg/ratecat/v1/internal/metric"
	"github.com/anserdsg/ratecat/v1/internal/mgr"
	"github.com/anserdsg/ratecat/v1/internal/server/api"
	"github.com/anserdsg/ratecat/v1/internal/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	err := config.Init()
	if err != nil {
		panic(err)
	}

	cfg := config.GetDefault()
	cfg.Env = "test"
	cfg.LogLevel = "debug"

	logger := logging.Init()
	if logger == nil {
		panic(errors.New("Failed to initial logger"))
	}

	exitCode := m.Run()
	os.Exit(exitCode)
}

func initServer(ctx context.Context, cfg *config.Config) (*api.Server, error) {
	mgr := mgr.NewResourceManager(ctx, cfg, nil)
	metric := metric.NewMetrics(ctx, cfg, metric.NullMetric)
	disp := cmd.NewCmdDispatcher(ctx, cfg, mgr, metric)
	return api.NewServer(ctx, cfg, mgr, disp, nil, metric)
}

func TestCmds(t *testing.T) {
	require := require.New(t)

	cfg := config.GetDefault()
	_ = cfg.ApiListenUrl.Decode("tcp://0.0.0.0:14117")
	_ = cfg.ApiAdvertiseUrl.Decode("tcp://0.0.0.0:14117")

	ctx := context.Background()
	server, err := initServer(ctx, cfg)
	require.NoError(err)
	require.NotNil(server)

	server.StartAsync()
	server.WaitReady()
	defer server.Stop(ctx) //nolint:errcheck

	logger := zap.NewExample()
	client := client.New(ctx, []string{"127.0.0.1:14117"}, client.WithLogger(logger))
	err = client.Dial()
	require.NoError(err)
	defer client.Close()

	var burst uint32 = 2
	resNodeID, err := client.RegisterTokenBucket("test", 10, burst, false)
	require.NoError(err)
	require.NotZero(resNodeID)

	ok, err := client.Allow("test", burst+1)
	require.NoError(err)
	require.False(ok)

	ok, err = client.Allow("test", 1)
	require.NoError(err)
	require.True(ok)

	ok, err = client.Allow("test", 1)
	require.NoError(err)
	require.True(ok)

	ok, err = client.Allow("test", 1)
	require.NoError(err)
	require.False(ok)

	res, err := client.GetResource("test")
	require.NoError(err)
	require.NotNil(res)
}

func TestGoroutineClient(t *testing.T) {
	require := require.New(t)

	cfg := config.GetDefault()
	_ = cfg.ApiListenUrl.Decode("tcp://0.0.0.0:14117")
	_ = cfg.ApiAdvertiseUrl.Decode("tcp://0.0.0.0:14117")

	ctx := context.Background()
	server, err := initServer(ctx, cfg)
	require.NoError(err)
	require.NotNil(server)

	server.StartAsync()
	server.WaitReady()
	defer server.Stop(ctx) //nolint:errcheck

	logger := zap.NewExample()
	client := client.New(ctx, []string{"127.0.0.1:14117"}, client.WithLogger(logger))
	err = client.Dial()
	require.NoError(err)
	defer client.Close()

	var burst uint32 = 100
	resNodeID, err := client.RegisterTokenBucket("test", float64(burst), burst, false)
	require.NoError(err)
	require.NotZero(resNodeID)

	var wg sync.WaitGroup
	for i := 0; i < int(burst); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Meow()
			require.NoError(err)
			require.NotNil(resp)

			ok, err := client.Allow("test", 1)
			require.NoError(err)
			require.True(ok)
		}()
	}
	wg.Wait()
}

func TestCluster_AllowCmd(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()

	clusterCount := 3
	apiServerHosts := make([]string, 0, clusterCount)
	clusterInfos := testutil.InitClusterServers(t, ctx, clusterCount)
	for _, cluster := range clusterInfos {
		apiServerHosts = append(apiServerHosts, cluster.APIServerHost)
		os.RemoveAll(cluster.StateDir)
		require.NoError(cluster.StartAsync(ctx))
		defer os.RemoveAll(cluster.StateDir)
		defer cluster.Stop(ctx) //nolint:errcheck
	}

	require.True(clusterInfos.WaitClusterReady())

	logger := zap.NewExample()
	client := client.New(ctx, apiServerHosts, client.WithLogger(logger))

	err := client.Dial()
	require.NoError(err)
	defer client.Close()

	resNodeID, err := client.RegisterTokenBucket("test1", 1000, 100, true)
	require.NoError(err)
	require.NotZero(resNodeID)

	for i := 0; i < 10; i++ {
		ok, err := client.Allow("test1", 1)
		require.NoError(err)
		require.True(ok)
	}
	time.Sleep(2 * time.Second)
	resNodeID, err = client.RegisterTokenBucket("test2", 1000, 10, true)
	require.NoError(err)
	require.NotZero(resNodeID)

	ok, err := client.Allow("test2", 1)
	require.NoError(err)
	require.True(ok)

	ok, err = client.Allow("test2", 1)
	require.NoError(err)
	require.True(ok)
}

func TestCluster_RegResCmd(t *testing.T) {
	require := require.New(t)

	ctx := context.Background()

	clusterCount := 3
	apiServerHosts := make([]string, 0, clusterCount)
	clusterInfos := testutil.InitClusterServers(t, ctx, clusterCount)
	for _, cluster := range clusterInfos {
		apiServerHosts = append(apiServerHosts, cluster.APIServerHost)
		os.RemoveAll(cluster.StateDir)
		require.NoError(cluster.StartAsync(ctx))
		defer os.RemoveAll(cluster.StateDir)
		defer cluster.Stop(ctx) //nolint:errcheck
	}
	require.True(clusterInfos.WaitClusterReady())

	logger := zap.NewExample()
	client := client.New(ctx, apiServerHosts, client.WithLogger(logger))

	err := client.Dial()
	require.NoError(err)
	defer client.Close()

	testCount := 1000
	regResNodeIDs := make([]uint32, testCount)
	for i := 0; i < testCount; i++ {
		resName := fmt.Sprintf("test%d", i)
		resNodeID, err := client.RegisterTokenBucket(resName, 1000, 100, true)
		require.NoError(err)
		require.NotZero(resNodeID)
		regResNodeIDs[i] = resNodeID
	}

	for i := 0; i < testCount; i++ {
		resName := fmt.Sprintf("test%d", i)
		res, err := client.GetResource(resName)
		require.NoError(err)
		require.NotNil(res)
		require.Equal(regResNodeIDs[i], res.NodeId)
	}
}

func BenchmarkCluster_GetResCmd(b *testing.B) {
	ctx := context.Background()
	testutil.SetupBenchmarkEnv()

	fmt.Printf("init b.N=%d\n", b.N)
	clusterCount := 3
	apiServerHosts := make([]string, 0, clusterCount)
	clusterInfos := testutil.InitClusterServers(nil, ctx, clusterCount)
	for _, cluster := range clusterInfos {
		apiServerHosts = append(apiServerHosts, cluster.APIServerHost)
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

	logger := zap.NewExample()
	client := client.New(ctx, apiServerHosts, client.WithLogger(logger))

	err := client.Dial()
	if err != nil {
		b.FailNow()
	}
	defer client.Close()

	_, err = client.RegisterTokenBucket("test1", 1000, 10, true)
	if err != nil {
		b.FailNow()
	}

	b.ResetTimer()
	fmt.Printf("b.N=%d\n", b.N)
	for i := 0; i <= b.N; i++ {
		_, err := client.GetResource("test1")
		if err != nil {
			b.FailNow()
		}
	}
	b.StopTimer()
}

func BenchmarkCluster_RegResCmd(b *testing.B) {
	ctx := context.Background()
	testutil.SetupBenchmarkEnv()

	fmt.Printf("init b.N=%d\n", b.N)
	clusterCount := 3
	apiServerHosts := make([]string, 0, clusterCount)
	clusterInfos := testutil.InitClusterServers(nil, ctx, clusterCount)
	for _, cluster := range clusterInfos {
		apiServerHosts = append(apiServerHosts, cluster.APIServerHost)
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

	logger := zap.NewExample()
	client := client.New(ctx, apiServerHosts, client.WithLogger(logger))

	err := client.Dial()
	if err != nil {
		b.FailNow()
	}
	defer client.Close()

	b.ResetTimer()
	fmt.Printf("b.N=%d\n", b.N)
	for i := 0; i <= b.N; i++ {
		resName := fmt.Sprintf("test%d", i)
		_, err := client.RegisterTokenBucket(resName, 1000, 10, true)
		if err != nil {
			b.FailNow()
		}
	}
	b.StopTimer()
}

func BenchmarkStandalone_RegResCmd(b *testing.B) {
	testutil.SetupBenchmarkEnv()

	cfg := config.GetDefault()
	_ = cfg.ApiListenUrl.Decode("tcp://0.0.0.0:14117")
	_ = cfg.ApiAdvertiseUrl.Decode("tcp://0.0.0.0:14117")

	ctx := context.Background()
	server, err := initServer(ctx, cfg)
	if err != nil {
		b.FailNow()
	}

	server.StartAsync()
	server.WaitReady()
	defer server.Stop(ctx) //nolint:errcheck

	logger := zap.NewNop()
	client := client.New(ctx, []string{"127.0.0.1:14117"}, client.WithLogger(logger))
	err = client.Dial()
	if err != nil {
		b.FailNow()
	}
	defer client.Close()

	b.ResetTimer()
	for i := 0; i <= b.N; i++ {
		resName := fmt.Sprintf("test%d", i)
		_, err := client.RegisterTokenBucket(resName, 1000, 10, true)
		if err != nil {
			b.FailNow()
		}
	}
}

func BenchmarkStandalone_GetResCmd(b *testing.B) {
	testutil.SetupBenchmarkEnv()

	cfg := config.GetDefault()
	_ = cfg.ApiListenUrl.Decode("tcp://0.0.0.0:14117")
	_ = cfg.ApiAdvertiseUrl.Decode("tcp://0.0.0.0:14117")

	ctx := context.Background()
	server, err := initServer(ctx, cfg)
	if err != nil {
		b.FailNow()
	}

	server.StartAsync()
	server.WaitReady()
	defer server.Stop(ctx) //nolint:errcheck

	logger := zap.NewNop()
	client := client.New(ctx, []string{"127.0.0.1:14117"}, client.WithLogger(logger))
	err = client.Dial()
	if err != nil {
		b.FailNow()
	}
	defer client.Close()

	_, err = client.RegisterTokenBucket("test1", 1000, 10, true)
	if err != nil {
		b.FailNow()
	}

	b.ResetTimer()
	fmt.Printf("b.N=%d\n", b.N)
	for i := 0; i <= b.N; i++ {
		_, err := client.GetResource("test1")
		if err != nil {
			b.FailNow()
		}
	}
	b.StopTimer()
}
