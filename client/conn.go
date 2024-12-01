package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/anserdsg/ratecat/v1/proto"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

type cmdFunc func(conn *Connection, cmdName string, cmdMsg proto.Message) ([]byte, *pb.CmdError)

type Connection struct {
	ctx       context.Context
	logger    *zap.SugaredLogger
	mu        sync.Mutex
	network   string
	conn      net.Conn
	connected bool
	cmdFunc   cmdFunc
	opts      *clientOptions

	nodeID uint32
	addr   *url.URL
	leader bool
	active bool
}

func newConnection(ctx context.Context, cmdFunc cmdFunc, opts *clientOptions) *Connection {
	c := &Connection{
		ctx:       ctx,
		connected: false,
		cmdFunc:   cmdFunc,
		opts:      opts,
		logger:    opts.logger,
	}

	return c
}

func (c *Connection) Dial(network string, addr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Debugw("Connection.Dial", "addr", addr)
	d := net.Dialer{Timeout: c.opts.dialTimeout}
	conn, err := d.DialContext(c.ctx, network, addr)
	if err != nil {
		return err
	}
	c.network = network
	c.conn = conn
	c.connected = true

	return nil
}

func (c *Connection) Conn() net.Conn {
	return c.conn
}

func (c *Connection) Connected() bool {
	return c.connected
}

func (c *Connection) SendCmd(cmdName string, cmdMsg proto.Message) ([]byte, *pb.CmdError) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmdFunc == nil {
		return nil, pb.NewCmdProcessFailError("The send command callback function is empty")
	}
	return c.cmdFunc(c, cmdName, cmdMsg)
}

func (c *Connection) Read(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *Connection) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *Connection) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Connection) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *Connection) meowCmd() (*pb.MeowCmdResp, error) {
	resp, err := c.SendCmd("Meow", nil)
	if err != nil {
		return nil, err
	}

	respMsg := &pb.MeowCmdResp{}
	dErr := proto.Unmarshal(resp, respMsg)
	if dErr != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_ProcessFail, dErr)
	}

	return respMsg, nil
}

type ConnectionPool struct {
	ctx        context.Context
	logger     *zap.SugaredLogger
	cancelFunc context.CancelFunc
	initAddrs  []string
	opts       *clientOptions
	sendCmd    cmdFunc

	mu      sync.Mutex
	pool    sync.Map
	resPool sync.Map

	running atomic.Bool
	stopCh  chan struct{}
}

func newConnectionPool(ctx context.Context, initAddrs []string, cmdFunc cmdFunc, opts *clientOptions) *ConnectionPool {
	pctx, cancel := context.WithCancel(ctx)
	return &ConnectionPool{
		ctx:        pctx,
		cancelFunc: cancel,
		initAddrs:  initAddrs,
		sendCmd:    cmdFunc,
		opts:       opts,
		logger:     opts.logger,
		stopCh:     make(chan struct{}, 1),
	}
}

func (p *ConnectionPool) checkConns() {
	go func() {
		p.running.Store(true)
		defer p.running.Store(false)
		defer func() { p.stopCh <- struct{}{} }()

		ticker := time.NewTicker(p.opts.checkInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = p.establishConns()
			case <-p.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (p *ConnectionPool) shutdown() {
	p.cancelFunc()

	if p.running.Load() {
		<-p.stopCh
	}

	p.iter(func(nodeID uint32, conn *Connection) bool {
		conn.Close()
		return true
	})
}

func (p *ConnectionPool) establishConns() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var connCount int

	respMsg, err := p.getServerInfo()
	if err != nil {
		return err
	}

	for _, node := range respMsg.Nodes {
		addr, err := url.Parse(node.ApiEndpoint)
		if err != nil {
			p.logger.Errorw("ratecat API endpoint url of node is invalid", "address", node.ApiEndpoint)
			return err
		}

		// update connection info
		if conn, err := p.get(node.Id); conn != nil && err == nil {
			conn.addr = addr
			conn.leader = node.Leader
			conn.active = node.Active
			connCount++
			p.logger.Debugw("Update node info", "id", node.Id, "addr", addr, "leader", node.Leader, "active", node.Active)
			continue
		}

		// create new connection
		conn := newConnection(p.ctx, p.sendCmd, p.opts)
		err = conn.Dial("tcp", addr.Host)
		if err != nil {
			p.logger.Errorw("conn.Dial(establishConns) fail", "addr", addr.Host, "error", err)
			continue
		}
		conn.addr = addr
		conn.nodeID = node.Id
		conn.leader = node.Leader
		conn.active = node.Active
		p.set(node.Id, conn)
		connCount++
	}

	// remove zombie connections
	p.iter(func(nodeID uint32, conn *Connection) bool {
		exist := false
		for _, node := range respMsg.Nodes {
			if node.Id == nodeID {
				exist = true
				break
			}
		}
		if !exist {
			p.pool.Delete(nodeID)
		}
		return true
	})

	if connCount == 0 {
		return errors.New("No connection established")
	}
	return nil
}

func (p *ConnectionPool) getServerInfo() (*pb.MeowCmdResp, error) {
	var respMsg *pb.MeowCmdResp
	var err error

	// retreive meow response from exisiting connection pool first
	p.pool.Range(func(k, v any) bool {
		conn, _ := v.(*Connection)
		if conn == nil {
			return true
		}
		for i := 0; i < 3; i++ {
			respMsg, err = conn.meowCmd()
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		return err != nil
	})
	if respMsg != nil {
		return respMsg, err
	}

	// retreive meow response from initial address list
	for _, addr := range p.initAddrs {
		conn := newConnection(p.ctx, p.sendCmd, p.opts)
		err = conn.Dial("tcp", addr)
		if err != nil {
			p.logger.Errorw("conn.Dial(getServerInfo) fail", "addr", addr, "error", err)
			continue
		}

		respMsg, err = conn.meowCmd()
		if err != nil {
			conn.Close()
			break
		}
		// add current connection into pool
		p.set(respMsg.NodeId, conn)
	}

	return respMsg, err
}

func (p *ConnectionPool) bindResourceConn(conn *Connection, name string) {
	p.resPool.Delete(name)
	p.resPool.Store(name, conn)
}

func (p *ConnectionPool) getResourceConn(name string) *Connection {
	v, ok := p.resPool.Load(name)
	if !ok {
		return p.getLeaderConn()
	}

	conn, _ := v.(*Connection)
	return conn
}

//nolint:unused
func (p *ConnectionPool) has(nodeID uint32) bool {
	_, ok := p.pool.Load(nodeID)
	return ok
}

//nolint:unused
func (p *ConnectionPool) hasAddr(addr string) bool {
	var result bool
	p.iter(func(nodeID uint32, conn *Connection) bool {
		if strings.Contains(conn.addr.String(), addr) {
			result = true
			return false
		}
		return true
	})

	return result
}

func (p *ConnectionPool) get(nodeID uint32) (*Connection, error) {
	v, ok := p.pool.Load(nodeID)
	if !ok {
		return nil, fmt.Errorf("Failed to get connection of node %d", nodeID)
	}

	conn, _ := v.(*Connection)
	return conn, nil
}

func (p *ConnectionPool) getLeaderConn() *Connection {
	var leader *Connection
	for i := 0; i < 3; i++ {
		p.iter(func(nodeID uint32, conn *Connection) bool {
			if conn.leader {
				leader = conn
				return false
			}
			return true
		})
		if leader != nil {
			break
		}
		p.logger.Warn("Failed to get leader connection")
		p.iter(func(nodeID uint32, conn *Connection) bool {
			p.logger.Debugw("conn info", "leader", conn.leader, "addr", conn.addr, "connected", conn.connected)
			return true
		})

		time.Sleep(100 * time.Millisecond)
		if err := p.establishConns(); err != nil {
			p.logger.Errorw("establishConns receive error", "error", err)
		}
	}

	if leader != nil {
		return leader
	}

	// find the first active connection
	p.iter(func(nodeID uint32, conn *Connection) bool {
		if conn.active {
			leader = conn
			return false
		}
		return true
	})

	return leader
}

func (p *ConnectionPool) set(nodeID uint32, conn *Connection) {
	conn.nodeID = nodeID
	p.pool.Store(nodeID, conn)
}

func (p *ConnectionPool) iter(f func(nodeID uint32, conn *Connection) bool) {
	p.pool.Range(func(k, v any) bool {
		nodeID, ok := k.(uint32)
		if !ok {
			return false
		}
		conn, ok := v.(*Connection)
		if !ok || conn == nil {
			p.logger.Errorw("ConnectionPool.Iter, conn is nil", "ok", ok)
			return false
		}

		return f(nodeID, conn)
	})
}
