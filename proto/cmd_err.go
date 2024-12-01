package proto

import (
	fmt "fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (e *CmdError) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

func (e *CmdError) Err() error {
	return e
}

func (e *CmdError) StatusErr() error {
	if e == nil {
		return nil
	}
	return status.Error(codes.Code(e.Code), e.Msg)
}

func NewCmdError(code ErrorCode, msg string) *CmdError {
	return &CmdError{Code: code, Msg: msg}
}

func WrapCmdError(code ErrorCode, err error) *CmdError {
	return &CmdError{Code: code, Msg: err.Error()}
}

// Helper functions
func msgf(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

func NewCmdUnknownError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_Unknown, Msg: msgf(msg, args...)}
}

func NewCmdNotFoundError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_NotFound, Msg: msgf(msg, args...)}
}

func NewCmdAlreadyExistsError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_AlreadyExists, Msg: msgf(msg, args...)}
}

func NewCmdOutOfRangeError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_OutOfRange, Msg: msgf(msg, args...)}
}

func NewCmdIncompletePacketError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_IncompletePacket, Msg: msgf(msg, args...)}
}

func NewCmdInvalidMsgError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_InvalidMsg, Msg: msgf(msg, args...)}
}

func NewCmdInvalidMagicNumError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_InvalidMagicNum, Msg: msgf(msg, args...)}
}

func NewCmdInvalidLimiterError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_InvalidLimiter, Msg: msgf(msg, args...)}
}

func NewCmdEncodeFailError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_EncodeFail, Msg: msgf(msg, args...)}
}

func NewCmdDecodeFailError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_DecodeFail, Msg: msgf(msg, args...)}
}

func NewCmdProcessFailError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_ProcessFail, Msg: msgf(msg, args...)}
}

func NewCmdForwardFailError(msg string, args ...any) *CmdError {
	return &CmdError{Code: ErrorCode_ForwardFail, Msg: msgf(msg, args...)}
}
