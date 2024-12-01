package api

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/anserdsg/ratecat/v1/internal/cmd"
	"github.com/anserdsg/ratecat/v1/internal/config"
	"github.com/anserdsg/ratecat/v1/internal/endec"
	"github.com/anserdsg/ratecat/v1/internal/logging"
	"github.com/anserdsg/ratecat/v1/internal/metric"
	"github.com/gogo/protobuf/proto"
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/pool/goroutine"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

type EventHandler interface {
	gnet.EventHandler
	Context() context.Context
}

type tcpEventHandler struct {
	*gnet.BuiltinEventEngine
	ctx        context.Context
	cfg        *config.Config
	disp       *cmd.CmdDispatcher
	metric     *metric.Metrics
	workerPool *goroutine.Pool
	ready      chan bool
}

func newTCPEventHandler(
	ctx context.Context,
	cfg *config.Config,
	disp *cmd.CmdDispatcher,
	metric *metric.Metrics,
	ready chan bool,
) *tcpEventHandler {
	var workerPool *goroutine.Pool
	if cfg.WorkerPool {
		workerPool = goroutine.Default()
		logging.Infow("Create event worker pool", "capability", workerPool.Cap(), "free", workerPool.Free())
	}

	return &tcpEventHandler{ctx: ctx, cfg: cfg, disp: disp, workerPool: workerPool, metric: metric, ready: ready}
}

func (h *tcpEventHandler) Context() context.Context {
	return h.ctx
}

func (h *tcpEventHandler) OnBoot(eng gnet.Engine) gnet.Action {
	h.ready <- true
	return gnet.None
}

func (h *tcpEventHandler) OnClose(c gnet.Conn, err error) gnet.Action {
	logging.Debugw("Close connection", "addr", c.RemoteAddr(), "error", err)
	return gnet.None
}

func (h *tcpEventHandler) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	c.SetContext(&endec.Codec{})
	return
}

func (h *tcpEventHandler) OnTick() (delay time.Duration, action gnet.Action) {
	delay = time.Second
	err := h.metric.ReportAPICmdRPS(delay)
	if err != nil {
		logging.Warnw("Failed to send API command RPS metric", "error", err)
	}
	return
}

func (h *tcpEventHandler) OnTraffic(c gnet.Conn) gnet.Action {
	codec, ok := c.Context().(*endec.Codec)
	if !ok {
		return gnet.Close
	}
	var loopCount uint64 = 0
	for {
		atomic.AddUint64(&loopCount, 1)
		inboundBytes := c.InboundBuffered()
		if inboundBytes == 0 {
			break
		}
		if atomic.LoadUint64(&loopCount) > 1 {
			logging.Warnw("OnTraffic loop > 1", "loopCount", loopCount)
		}

		cmdHeader, bodyData, err := codec.DecodeCmdReq(c)
		if errors.Is(err, endec.ErrIncompletePacket) {
			h.respCmdError(c, codec, err.Error())
			return gnet.Close
		} else if errors.Is(err, endec.ErrInvalidMagicNum) {
			h.respCmdError(c, codec, err.Error())
			return gnet.Close
		} else if errors.Is(err, endec.ErrInvalidMsg) {
			h.respCmdError(c, codec, err.Error())
			return gnet.None
		}

		h.metric.AddAPICmdCount()

		if h.workerPool == nil {
			respMsgs := h.process(codec, cmdHeader, bodyData)
			var err error
			if h.cfg.MultiCore {
				err = c.AsyncWritev(respMsgs, nil)
			} else {
				_, err = c.Writev(respMsgs)
			}
			if err != nil {
				logging.Warnw("Failed to execute Writev", "error", err)
			}
		} else {
			cloned := make([]byte, len(bodyData))
			copy(cloned, bodyData)
			err := h.workerPool.Submit(func() {
				respMsgs := h.process(codec, cmdHeader, cloned)
				err := c.AsyncWritev(respMsgs, nil)
				if err != nil {
					logging.Warnw("Failed to execute AsyncWritev", "error", err)
				}
			})
			if err != nil {
				logging.Errorw("Failed to process command", "error", err)
				return gnet.Close
			}
		}
	}

	return gnet.None
}

func (h *tcpEventHandler) respCmdError(c gnet.Conn, codec *endec.Codec, msg string) {
	respMsg := &pb.CmdError{Msg: msg}
	resp, _ := proto.Marshal(respMsg)
	msgs := codec.EncodeCmdResp(endec.CmdRespErrorMagicNum, resp)
	_, err := c.Writev(msgs)
	if err != nil {
		logging.Warnw("Failed to execute Writev", "error", err)
	}
}

func (h *tcpEventHandler) process(codec *endec.Codec, header *pb.CmdHeader, data []byte) [][]byte {
	magicNum := endec.CmdRespSuccessMagicNum
	respMsg, err := h.disp.Process(header, data)
	if err != nil {
		magicNum = endec.CmdRespErrorMagicNum
		respMsg = &pb.CmdError{Msg: err.Error()}
	}
	if respMsg == nil {
		magicNum = endec.CmdRespErrorMagicNum
		respMsg = &pb.CmdError{Msg: "Empty response message"}
	}

	resp, encodeErr := proto.Marshal(respMsg)
	if encodeErr != nil {
		logging.Warnw("Failed to marshal response message", "error", err)
	}

	return codec.EncodeCmdResp(uint32(magicNum), resp)
}
