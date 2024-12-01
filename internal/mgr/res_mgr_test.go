package mgr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/limit"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	cpb "github.com/anserdsg/ratecat/v1/internal/cluster/pb"
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

func TestResourceManager_PutResource(t *testing.T) {
	require := require.New(t)

	mgr := NewResourceManager(context.Background(), config.GetDefault(), nil)
	for i := 0; i < 100_000; i++ {
		res, err := mgr.PutResource(fmt.Sprintf("item%d", i), limit.NewTokenBucketLimiter(1000, 10))
		require.NoError(err)
		require.NotNil(res)
	}

	for i := 0; i < 100_000; i++ {
		key := fmt.Sprintf("item%d", i)
		res, ok := mgr.GetResource(key)
		require.True(ok)
		require.NotNil(res)
		require.Equal(mgr.CurNodeID(), res.NodeID)
		require.Equal("TokenBucket", res.Limiter.Type())
	}
}

func TestResourceManager_PutAndRemoveResource(t *testing.T) {
	require := require.New(t)
	config.GetDefault().Cluster.Enabled = false
	mgr := NewResourceManager(context.Background(), config.GetDefault(), nil)
	for i := 0; i < 100_000; i++ {
		res, err := mgr.PutResource(fmt.Sprintf("item%d", i), limit.NewTokenBucketLimiter(1000, 10))
		require.NoError(err)
		require.NotNil(res)
	}

	for i := 0; i < 50_000; i++ {
		err := mgr.RemoveResource(fmt.Sprintf("item%d", i))
		require.NoError(err)
	}

	for i := 0; i < 50_000; i++ {
		require.False(mgr.HasResource(fmt.Sprintf("item%d", i)))
	}

	for i := 50_000; i < 100_000; i++ {
		require.True(mgr.HasResource(fmt.Sprintf("item%d", i)))
	}
}

func TestResourceManager_UpdateResourceNodeID(t *testing.T) {
	require := require.New(t)

	mgr := NewResourceManager(context.Background(), config.GetDefault(), nil)
	for i := 0; i < 100_000; i++ {
		res, err := mgr.PutResource(fmt.Sprintf("item%d", i), limit.NewTokenBucketLimiter(1000, 10))
		require.NoError(err)
		require.NotNil(res)
	}

	for i := 0; i < 100_000; i++ {
		key := fmt.Sprintf("item%d", i)
		err := mgr.UpdateResourceNodeId(key, uint32(i))
		require.NoError(err)
	}

	for i := 0; i < 100_000; i++ {
		key := fmt.Sprintf("item%d", i)
		res, ok := mgr.GetResource(key)
		require.True(ok)
		require.NotNil(res)
		require.Equal(uint32(i), res.NodeID, "i=%d key=%s res node_id=%d", i, key, res.NodeID)
	}
}

func TestResourceManager_Entry(t *testing.T) {
	require := require.New(t)

	_ = config.Init()
	mgr := NewResourceManager(context.Background(), config.GetDefault(), nil)
	require.NotNil(mgr)

	limiter := limit.NewTokenBucketLimiter(1000, 10)
	require.NotNil(limiter)

	entryPB, err := newEntryPBWithRes("test1", cpb.EntryAction_PutResource, 1, newResource(2, limiter))
	require.NoError(err)
	require.NotNil(entryPB)

	data, err := proto.Marshal(entryPB)
	require.NotNil(data)
	require.NoError(err)

	mgr.Apply(data)

	res, ok := mgr.GetResource("test1")
	require.NotNil(res)
	require.True(ok)

	require.Equal(uint32(2), res.NodeID)
	require.Equal(pb.LimiterType_TokenBucket, res.Limiter.PBType())
	require.Equal(pb.LimiterType_TokenBucket.String(), res.Limiter.Type())

	entryPB = newEntryPB("test1", cpb.EntryAction_RemoveResource, 1, 2)
	require.NotNil(entryPB)

	data, err = proto.Marshal(entryPB)
	require.NotNil(data)
	require.NoError(err)

	mgr.Apply(data)

	res, ok = mgr.GetResource("test1")
	require.Nil(res)
	require.False(ok)
}

func TestResourceManager_updateResourceNodeId(t *testing.T) {
	require := require.New(t)

	mgr := NewResourceManager(context.Background(), config.GetDefault(), nil)
	for i := 0; i < 1000; i++ {
		limiter := limit.NewTokenBucketLimiter(1000, 10)
		require.NotNil(limiter)
		resName := fmt.Sprintf("item%d", i)
		res := newResource(1, limiter)
		require.NotNil(res)
		mgr.putResource(resName, res)
	}

	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("item%d", i)
		mgr.updateResourceNodeId(name, 2)
		res, ok := mgr.GetResource(name)
		require.True(ok)
		require.NotNil(res)
		require.Equal(uint32(2), res.NodeID)
	}

	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("item%d", i)
		mgr.updateResourceNodeId(name, 3)
		res, ok := mgr.GetResource(name)
		require.True(ok)
		require.NotNil(res)
		require.Equal(uint32(3), res.NodeID)
	}
}

func TestResourceManager_removeResource(t *testing.T) {
	require := require.New(t)

	mgr := NewResourceManager(context.Background(), config.GetDefault(), nil)
	for i := 0; i < 1000; i++ {
		limiter := limit.NewTokenBucketLimiter(1000, 10)
		require.NotNil(limiter)
		resName := fmt.Sprintf("item%d", i)
		res := newResource(1, limiter)
		require.NotNil(res)
		mgr.putResource(resName, res)
	}

	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("item%d", i)
		mgr.removeResource(name)
		res, ok := mgr.GetResource(name)
		require.False(ok)
		require.Nil(res)
	}
}
