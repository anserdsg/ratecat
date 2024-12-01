package proto_test

import (
	"testing"

	pb "github.com/anserdsg/ratecat/v1/proto"
	"github.com/gogo/protobuf/proto"
)

func BenchmarkProtoEncode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		header := pb.CmdHeader{Cmd: "Meow"}
		_, err := proto.Marshal(&header)
		if err != nil {
			b.Fatal(err)
		}

		cmd := pb.AllowCmd{Events: 100}
		_, err = proto.Marshal(&cmd)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtoDecode(b *testing.B) {
	header := pb.CmdHeader{Cmd: "Meow"}
	headerBytes, err := proto.Marshal(&header)
	if err != nil {
		b.Fatal(err)
	}
	cmd := pb.AllowCmd{Events: 100}
	cmdBytes, err := proto.Marshal(&cmd)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = proto.Unmarshal(headerBytes, &pb.CmdHeader{})
		_ = proto.Unmarshal(cmdBytes, &pb.AllowCmd{})
	}
}
