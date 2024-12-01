package cmd

import (
	"context"
	"time"

	pb "github.com/anserdsg/ratecat/v1/proto"

	"github.com/gogo/protobuf/proto"

	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/anserdsg/ratecat/v1/internal/metric"
	"github.com/anserdsg/ratecat/v1/internal/mgr"
)

type CommandHandler func(body []byte) (proto.Message, *pb.CmdError)

type CmdDispatcher struct {
	ctx      context.Context
	cfg      *config.Config
	resMgr   *mgr.ResourceManager
	metric   *metric.Metrics
	initTime time.Time
	cmds     map[string]CommandHandler
}

func NewCmdDispatcher(ctx context.Context, cfg *config.Config, rs *mgr.ResourceManager, mt *metric.Metrics) *CmdDispatcher {
	c := &CmdDispatcher{
		ctx:      ctx,
		cfg:      cfg,
		metric:   mt,
		resMgr:   rs,
		initTime: time.Now(),
	}
	c.cmds = map[string]CommandHandler{
		"Meow":   c.Meow,
		"Allow":  c.Allow,
		"GetRes": c.GetResource,
		"RegRes": c.RegisterResource,
	}

	return c
}

func (c *CmdDispatcher) Process(header *pb.CmdHeader, body []byte) (proto.Message, *pb.CmdError) {
	logging.Debugw("Process command", "cmd", header.Cmd)

	handler, ok := c.cmds[header.Cmd]
	if !ok {
		return nil, pb.NewCmdProcessFailError("Invalid command: %s", header.Cmd)
	}
	return handler(body)
}
