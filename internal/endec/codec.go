package endec

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"

	"github.com/gogo/protobuf/proto"
	"github.com/panjf2000/gnet/v2"

	pb "github.com/anserdsg/ratecat/v1/proto"
)

const (
	CmdReqMagicNum         = 0x1015
	CmdRespSuccessMagicNum = 0x1016
	CmdRespErrorMagicNum   = 0x1017
	CmdRespInvalidMagicNum = 0x1018

	MagicNumSize     = 2
	TotalLenSize     = 4
	CmdMsgLenSize    = 4
	HeaderMsgLenSize = 2
	ReqHeaderSize    = MagicNumSize + TotalLenSize + HeaderMsgLenSize
	RespHeaderSize   = MagicNumSize + CmdMsgLenSize

	MaxRespDataSize = 4096
)

var (
	ErrFail             = errors.New("Invalid message")
	ErrInvalidMsg       = errors.New("Invalid message")
	ErrInvalidMagicNum  = errors.New("Invalid magic number")
	ErrIncompletePacket = errors.New("Incomplete packet")
)

var (
	cmdReqMagicNumBytes         []byte
	cmdRespSuccessMagicNumBytes []byte
	cmdRespErrorMagicNumBytes   []byte
)

func init() {
	// initialize magic number bytes for further comparison
	cmdReqMagicNumBytes = make([]byte, MagicNumSize)
	binary.LittleEndian.PutUint16(cmdReqMagicNumBytes, uint16(CmdReqMagicNum))
	cmdRespSuccessMagicNumBytes = make([]byte, MagicNumSize)
	binary.LittleEndian.PutUint16(cmdRespSuccessMagicNumBytes, uint16(CmdRespSuccessMagicNum))
	cmdRespErrorMagicNumBytes = make([]byte, MagicNumSize)
	binary.LittleEndian.PutUint16(cmdRespErrorMagicNumBytes, uint16(CmdRespErrorMagicNum))
}

type Codec struct{}

// Command request format:
// magicNum:     2 bytes, uint16(little endian)
// totalLen:     4 bytes, uint32(little endian)
// headerMsgLen: 2 bytes, uint16(little endian)
// headerMsg:    [headerMsgLen] bytes, protobuf
// cmdMsg:       [totalLen - headerMsgLen] bytes, protobuf
func (c Codec) EncodeCmdReq(cmdName string, msg proto.Message) ([]byte, *pb.CmdError) {
	headerMsg, err := proto.Marshal(&pb.CmdHeader{Cmd: cmdName})
	if err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_InvalidMsg, err)
	}
	headerMsgLen := len(headerMsg)

	cmdMsgLen := 0
	var cmdMsg []byte
	if msg != nil {
		cmdMsg, err = proto.Marshal(msg)
		if err != nil {
			return nil, pb.WrapCmdError(pb.ErrorCode_InvalidMsg, err)
		}
		cmdMsgLen = len(cmdMsg)
	}

	totalLen := headerMsgLen + cmdMsgLen
	cmd := make([]byte, ReqHeaderSize+totalLen)
	copy(cmd[:MagicNumSize], cmdReqMagicNumBytes)
	binary.LittleEndian.PutUint32(cmd[MagicNumSize:], uint32(totalLen))
	binary.LittleEndian.PutUint16(cmd[MagicNumSize+TotalLenSize:], uint16(headerMsgLen))
	copy(cmd[ReqHeaderSize:], headerMsg)
	if cmdMsgLen > 0 {
		copy(cmd[ReqHeaderSize+headerMsgLen:], cmdMsg)
	}

	return cmd, nil
}

func (c *Codec) DecodeCmdReq(conn gnet.Conn) (*pb.CmdHeader, []byte, *pb.CmdError) {
	buf, _ := conn.Next(ReqHeaderSize)
	if len(buf) < ReqHeaderSize {
		return nil, nil, pb.NewCmdIncompletePacketError("The buffer size %d less than %d", len(buf), ReqHeaderSize)
	}

	if !bytes.Equal(cmdReqMagicNumBytes, buf[:MagicNumSize]) {
		magicNum := binary.LittleEndian.Uint16(buf[:MagicNumSize])
		return nil, nil, pb.NewCmdInvalidMagicNumError("Invalid magic number: %X", magicNum)
	}

	totalLen := int(binary.LittleEndian.Uint32(buf[MagicNumSize:]))
	if totalLen < HeaderMsgLenSize {
		return nil, nil, pb.NewCmdIncompletePacketError("The total message size %d is less than %d", totalLen, HeaderMsgLenSize)
	}

	headerMsgLen := int(binary.LittleEndian.Uint16(buf[MagicNumSize+TotalLenSize:]))
	if headerMsgLen < 1 {
		return nil, nil, pb.NewCmdIncompletePacketError("The header message size %d is less than 1 byte", headerMsgLen)
	}

	data, err := conn.Next(totalLen)
	if len(data) < totalLen {
		return nil, nil, pb.WrapCmdError(pb.ErrorCode_IncompletePacket, err)
	}

	cmdHeader := &pb.CmdHeader{}
	err = proto.Unmarshal(data[:headerMsgLen], cmdHeader)
	if err != nil {
		return nil, nil, pb.WrapCmdError(pb.ErrorCode_DecodeFail, err)
	}

	return cmdHeader, data[headerMsgLen:], nil
}

/*
Command response format:
magicNum   2 bytes, uint16(little endian)
cmdMsgLen: 4 bytes, uint32(little endian)
cmdMsg:    [cmdMsgLen] bytes, protobuf
*/
func (c *Codec) DecodeCmdResp(conn net.Conn) ([]byte, *pb.CmdError) {
	header := make([]byte, RespHeaderSize)
	n, err := conn.Read(header)
	if n < RespHeaderSize {
		return nil, pb.NewCmdIncompletePacketError("The header message size %d is less than %d byte", n, RespHeaderSize)
	}
	if err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_IncompletePacket, err)
	}

	var magicNum uint16
	if bytes.Equal(cmdRespSuccessMagicNumBytes, header[:MagicNumSize]) {
		magicNum = CmdRespSuccessMagicNum
	} else if bytes.Equal(cmdRespErrorMagicNumBytes, header[:MagicNumSize]) {
		magicNum = CmdRespErrorMagicNum
	} else {
		magicNum = binary.LittleEndian.Uint16(header[:MagicNumSize])
		return nil, pb.NewCmdInvalidMagicNumError("Invalid magic number: %X", magicNum)
	}

	cmdMsgLen := int(binary.LittleEndian.Uint32(header[MagicNumSize:]))
	if cmdMsgLen > MaxRespDataSize {
		return nil, pb.NewCmdInvalidMsgError("The message size %d is larger than max response size %d", cmdMsgLen, MaxRespDataSize)
	}

	cmdMsg := make([]byte, cmdMsgLen)
	n, err = conn.Read(cmdMsg)
	if n < cmdMsgLen {
		return nil, pb.NewCmdIncompletePacketError("The message size %d is less than %d byte", n, cmdMsgLen)
	}
	if err != nil {
		return nil, pb.WrapCmdError(pb.ErrorCode_IncompletePacket, err)
	}

	if magicNum == CmdRespErrorMagicNum {
		errResp := &pb.CmdError{}
		err := proto.Unmarshal(cmdMsg, errResp)
		if err != nil {
			return nil, pb.WrapCmdError(pb.ErrorCode_DecodeFail, err)
		}
		return nil, errResp
	}

	return cmdMsg, nil
}

func (c *Codec) EncodeCmdResp(respMagicNum uint32, resp []byte) [][]byte {
	var respHeader [MagicNumSize + TotalLenSize]byte
	switch respMagicNum {
	case CmdRespSuccessMagicNum:
		copy(respHeader[:MagicNumSize], cmdRespSuccessMagicNumBytes)
	case CmdRespErrorMagicNum:
		copy(respHeader[:MagicNumSize], cmdRespErrorMagicNumBytes)
	}

	if resp == nil {
		return [][]byte{respHeader[:]}
	}

	binary.LittleEndian.PutUint32(respHeader[MagicNumSize:], uint32(len(resp)))
	return [][]byte{respHeader[:], resp}
}
