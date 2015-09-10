package amp

// "A Media Protocol"
// Mini-protocol to initiate an RTP/RTCP media stream.

import (
	"encoding/gob"
	"fmt"
	"net"
	"strconv"

	"github.com/antongulenko/RTP/protocols"
)

const (
	CodeStartStream = protocols.CodeOther + iota
	CodeStopStream
)

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

type ampProtocol struct {
}

func (*ampProtocol) Name() string {
	return "AMP"
}

func (*ampProtocol) DefaultBufferSize() uint {
	return 512
}

func (*ampProtocol) DecodeValue(code uint, dec *gob.Decoder) (interface{}, error) {
	switch code {
	case CodeStartStream:
		var val StartStream
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding AMP StartStream value: %v", err)
		}
		return &val, nil
	case CodeStopStream:
		var val StopStream
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding AMP StopStream value: %v", err)
		}
		return &val, nil
	default:
		return nil, fmt.Errorf("Unknown AMP code: %v", code)
	}
}
