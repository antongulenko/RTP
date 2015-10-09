package amp

// "A Media Protocol"
// Mini-protocol to initiate and control an RTP/RTCP media stream.

import (
	"encoding/gob"
	"fmt"
	"net"
	"strconv"

	"github.com/antongulenko/RTP/protocols"
)

var (
	Protocol     *ampProtocol
	MiniProtocol = protocols.NewMiniProtocol(Protocol)
)

const (
	CodeStartStream = protocols.Code(15 + iota)
	CodeStopStream
)

// ======================= Packets =======================

type ClientDescription struct {
	ReceiverHost string
	Port         int
}

type StartStream struct {
	ClientDescription
	MediaFile string
}

type StopStream struct {
	ClientDescription
}

func (client *ClientDescription) Client() string {
	return net.JoinHostPort(client.ReceiverHost, strconv.Itoa(client.Port))
}

// ======================= Protocol =======================

type ampProtocol struct {
}

func (*ampProtocol) Name() string {
	return "AMP"
}

func (proto *ampProtocol) Decoders() protocols.DecoderMap {
	return protocols.DecoderMap{
		CodeStartStream: proto.decodeStartStream,
		CodeStopStream:  proto.decodeStopStream,
	}
}

func (proto *ampProtocol) decodeStartStream(decoder *gob.Decoder) (interface{}, error) {
	var val StartStream
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding AMP StartStream value: %v", err)
	}
	return &val, nil
}
func (proto *ampProtocol) decodeStopStream(decoder *gob.Decoder) (interface{}, error) {
	var val StopStream
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding AMP StopStream value: %v", err)
	}
	return &val, nil
}
