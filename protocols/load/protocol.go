package load

// Protocol for generating controlled network load

import (
	"encoding/gob"
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

var (
	Protocol     *loadProtocol
	MiniProtocol = protocols.NewMiniProtocolTransport(Protocol, protocols.UdpTransportB(2048))
)

const (
	codeLoad   = protocols.Code(100)
	PacketSize = 57 // Reported by tcpdump, size of LoadPacket with empty Payload. Varies between 55-57.
)

type LoadPacket struct {
	Seq     uint
	Payload []byte
}

type loadProtocol struct {
}

func (packet *LoadPacket) Size() uint {
	return PacketSize + uint(len(packet.Payload))
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
