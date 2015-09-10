package pcp

// "Proxy Control Protocol"
// For controlling remote udp proxies

import (
	"encoding/gob"
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

const (
	CodeStartProxy = protocols.CodeOther + iota
	CodeStopProxy
)

type StartProxy struct {
	ListenAddr string
	TargetAddr string
}

type StopProxy StartProxy

type pcpProtocol struct {
}

func (*pcpProtocol) Name() string {
	return "PCP"
}

func (*pcpProtocol) DefaultBufferSize() uint {
	return 512
}

func (*pcpProtocol) DecodeValue(code uint, dec *gob.Decoder) (interface{}, error) {
	switch code {
	case CodeStartProxy:
		var val StartProxy
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding PCP StartProxy value: %v", err)
		}
		return &val, nil
	case CodeStopProxy:
		var val StopProxy
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding PCP StopProxy value: %v", err)
		}
		return &val, nil
	default:
		return nil, fmt.Errorf("Unknown PCP code: %v", code)
	}
}
