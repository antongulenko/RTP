package load

// Protocol for generating controlled network load

import (
	"encoding/gob"
	"fmt"
	"time"

	"github.com/antongulenko/RTP/protocols"
)

var (
	Protocol     *loadProtocol
	MiniProtocol = protocols.NewMiniProtocolTransport(Protocol, protocols.UdpTransportB(2048))
)

const (
	codeLoad   = protocols.Code(100)
	PacketSize = 105 // Reported by tcpdump, size of LoadPacket with empty Payload. Varies between 105-107.
)

type LoadPacket struct {
	Seq       uint
	Payload   []byte
	Timestamp time.Time
}

func (packet *LoadPacket) String() string {
	return fmt.Sprintf("Load(Seq %v, %v byte payload)", packet.Seq, len(packet.Payload))
}

func (packet *LoadPacket) PrintReceived() {
	now := time.Now()
	latency := now.Sub(packet.Timestamp)
	fmt.Printf("%v: Seq %3d (latency %v)\n", now.Format("15:04:05.000000000"), packet.Seq, latency)
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
