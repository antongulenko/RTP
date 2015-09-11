package pcp

// "Proxy Control Protocol"
// For controlling remote udp proxies

import (
	"encoding/gob"
	"fmt"
	"net"
	"strconv"

	"github.com/antongulenko/RTP/protocols"
)

const (
	CodeStartProxy = protocols.CodeOther + iota
	CodeStopProxy
)

type ProxyDescription struct {
	ListenAddr string
	TargetAddr string
}

type StartProxy struct {
	ProxyDescription
}

type StopProxy struct {
	ProxyDescription
}

func (desc *ProxyDescription) ListenPort() (int, error) {
	_, port, err := net.SplitHostPort(desc.ListenAddr)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse ListenAddr: %v", err)
	}
	return strconv.Atoi(port)
}

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
