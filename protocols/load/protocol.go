package load

// Protocol for generating controlled network load

import (
	"encoding/gob"
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

var (
	Protocol     *loadProtocol
	MiniProtocol = protocols.NewMiniProtocolTransport(Protocol, protocols.UdpTransport())
)

const (
	codeLoad   = protocols.Code(100)
	PacketSize = 45 // Reported by tcpdump
)

type LoadPacket struct {
	Seq uint
}

type loadProtocol struct {
}

func (*loadProtocol) Name() string {
	return "Load"
}

func (proto *loadProtocol) Decoders() protocols.DecoderMap {
	return protocols.DecoderMap{
		codeLoad: proto.decodeLoad,
	}
}

func (proto *loadProtocol) decodeLoad(decoder *gob.Decoder) (interface{}, error) {
	var val LoadPacket
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding LoadPacket value: %v", err)
	}
	return &val, nil
}
