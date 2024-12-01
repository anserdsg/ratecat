package proto_test

import (
	"testing"

	pb "github.com/anserdsg/ratecat/v1/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/status"
)

func TestCmdError_Convert(t *testing.T) {
	require := require.New(t)

	var err *pb.CmdError

	require.Nil(err.Err())
	require.Nil(err.StatusErr())

	err = pb.NewCmdError(pb.ErrorCode_AlreadyExists, pb.ErrorCode_AlreadyExists.String())
	require.NotNil(err)
	require.IsType(&pb.CmdError{}, err)
	require.Equal(pb.ErrorCode_AlreadyExists.String(), err.Error())
	require.Equal(pb.ErrorCode_AlreadyExists.String(), err.Err().Error())

	for _, v := range pb.ErrorCode_value {
		code := pb.ErrorCode(v)
		err := pb.NewCmdError(code, code.String())
		require.NotNil(err)
		st, ok := status.FromError(err.StatusErr())
		require.True(ok)
		require.IsType(&status.Status{}, st)
		require.Equal(uint32(code), uint32(st.Code()))
		if code != pb.ErrorCode_OK {
			require.Equal(code.String(), st.Message())
		}
	}
}
