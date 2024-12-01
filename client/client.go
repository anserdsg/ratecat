package client

import (
	"context"
	"fmt"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/endec"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

var ErrLeaderNonexist = pb.NewCmdNotFoundError("The leader connection doesn't exist")

type Client interface {
	Dial() error
	Close()
	Meow() (*pb.MeowCmdResp, error)
	GetResource(resource string) (*pb.Resource, error)
	RegisterTokenBucket(resource string, rate float64, burst uint32, override bool) (uint32, error)
	Allow(resource string, events uint32) (bool, error)
}

type TCPClient struct {
	ctx      context.Context
	addrs    []string
	opts     *clientOptions
	connPool *ConnectionPool
	codec    *endec.Codec
	logger   *zap.SugaredLogger
}

func New(ctx context.Context, addrs []string, opts ...ClientOption) Client {
	client := &TCPClient{
		ctx:   ctx,
		addrs: addrs,
		codec: &endec.Codec{},
	}

	client.opts = &clientOptions{
		dialTimeout:   2 * time.Second,
		checkInterval: 10 * time.Second,
	}
	for _, opt := range opts {
		opt.apply(client.opts)
	}

	client.connPool = newConnectionPool(ctx, addrs, client.sendCmd, client.opts)

	if client.opts.logger == nil {
		client.opts.logger = zap.NewNop().Sugar()
	}
	client.logger = client.opts.logger

	return client
}

func (c *TCPClient) Dial() error {
	c.logger.Infow("TCPClient.Dial")
	err := c.connPool.establishConns()
	c.connPool.checkConns()
	return err
}

func (c *TCPClient) Close() {
	c.connPool.shutdown()
}

func (c *TCPClient) Meow() (*pb.MeowCmdResp, error) {
	conn := c.connPool.getLeaderConn()
	if conn == nil {
		return nil, ErrLeaderNonexist
	}

	return c.meowCmd(conn)
}

func (c *TCPClient) meowCmd(conn *Connection) (*pb.MeowCmdResp, error) {
	resp, err := conn.SendCmd("Meow", nil)
	if err != nil {
		return nil, err
	}

	respMsg := &pb.MeowCmdResp{}
	if err := proto.Unmarshal(resp, respMsg); err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, err)
	}

	return respMsg, nil
}

func (c *TCPClient) GetResource(resource string) (*pb.Resource, error) {
	conn := c.connPool.getResourceConn(resource)
	if conn == nil {
		return nil, fmt.Errorf("connection for resource %s doesnt exist", resource)
	}

	msg := &pb.GetResCmd{Resource: resource}
	resp, err := conn.SendCmd("GetRes", msg)
	if err != nil {
		return nil, err
	}

	respMsg := &pb.Resource{}
	if err := proto.Unmarshal(resp, respMsg); err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, err)
	}

	return respMsg, nil
}

func (c *TCPClient) RegisterTokenBucket(resource string, rate float64, burst uint32, override bool) (uint32, error) {
	conn := c.connPool.getLeaderConn()
	if conn == nil {
		return 0, ErrLeaderNonexist
	}

	respMsg, err := c.registerTokenBucketCmd(conn, resource, rate, burst, override)
	if err != nil {
		return 0, err
	}

	if conn.nodeID != respMsg.NodeId {
		newConn, err := c.connPool.get(respMsg.NodeId)
		if err != nil {
			return 0, err
		}

		c.connPool.bindResourceConn(newConn, resource)
	}

	return respMsg.NodeId, nil
}

func (c *TCPClient) registerTokenBucketCmd(conn *Connection, resource string, rate float64, burst uint32, override bool) (*pb.RegResCmdResp, *pb.CmdError) {
	msg := &pb.RegResCmd{
		Resource: resource,
		Type:     pb.LimiterType_TokenBucket,
		Override: override,
		Option: &pb.RegResCmd_TokenBucket{
			TokenBucket: &pb.TokenBucketParam{Rate: rate, Burst: burst},
		},
	}
	resp, err := conn.SendCmd("RegRes", msg)
	if err != nil {
		return nil, err
	}

	respMsg := &pb.RegResCmdResp{}
	if err := proto.Unmarshal(resp, respMsg); err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, err)
	}

	return respMsg, nil
}

func (c *TCPClient) Allow(resource string, events uint32) (bool, error) {
	conn := c.connPool.getResourceConn(resource)
	if conn == nil {
		return true, fmt.Errorf("connection for resource %s doesnt exist", resource)
	}

	respMsg, err := c.allowCmd(conn, resource, events)
	if err != nil {
		return true, err
	}

	if respMsg.Redirected {
		c.logger.Infow("Resource redirected", "node_id", respMsg.NodeId)
		newConn, err := c.connPool.get(respMsg.NodeId)
		if err != nil {
			return true, err
		}
		// bind resource to new connection and retry
		c.connPool.bindResourceConn(newConn, resource)
		return c.Allow(resource, events)
	}

	return respMsg.Ok, nil
}

func (c *TCPClient) allowCmd(conn *Connection, resource string, events uint32) (*pb.AllowCmdResp, *pb.CmdError) {
	msg := &pb.AllowCmd{Resource: resource, Events: events}
	resp, err := conn.SendCmd("Allow", msg)
	if err != nil {
		return nil, err
	}

	respMsg := &pb.AllowCmdResp{}
	if err := proto.Unmarshal(resp, respMsg); err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, err)
	}

	return respMsg, nil
}

func (c *TCPClient) sendCmd(conn *Connection, cmdName string, cmdMsg proto.Message) ([]byte, *pb.CmdError) {
	if !conn.Connected() {
		if err := c.Dial(); err != nil {
			return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, err)
		}
	}

	cmd, err := c.codec.EncodeCmdReq(cmdName, cmdMsg)
	if err != nil {
		c.logger.Errorw("codec.EncodeCmdReq fails", "error", err)
		return nil, err
	}

	_ = conn.SetDeadline(time.Now().Add(1 * time.Second))

	sent, wErr := conn.Write(cmd)
	if wErr != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, wErr)
	}
	if sent != len(cmd) {
		return nil, pb.NewCmdProcessFailError("Expect to send %d bytes, sent %d bytes\n", len(cmd), sent)
	}

	respCmd, err := c.codec.DecodeCmdResp(conn.Conn())
	if err != nil {
		return nil, err
	}

	return respCmd, nil
}
