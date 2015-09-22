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
	CodeStartProxyPair
	CodeStopProxyPair
	CodeStartProxyPairResponse
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

type StartProxyPair struct {
	ReceiverHost  string
	ReceiverPort1 int
	ReceiverPort2 int
}

type StopProxyPair struct {
	ProxyPort1 int
}

type StartProxyPairResponse struct {
	ProxyHost  string
	ProxyPort1 int
	ProxyPort2 int
}

func (desc *ProxyDescription) ListenPort() (int, error) {
	_, port, err := net.SplitHostPort(desc.ListenAddr)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse ListenAddr: %v", err)
	}
	return strconv.Atoi(port)
}

type PcpProtocol struct {
}

func (*PcpProtocol) Name() string {
	return "PCP"
}

func (*PcpProtocol) DefaultBufferSize() uint {
	return 512
}

func (*PcpProtocol) DecodeValue(code uint, dec *gob.Decoder) (interface{}, error) {
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
	case CodeStartProxyPair:
		var val StartProxyPair
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding PCP StartProxyPair value: %v", err)
		}
		return &val, nil
	case CodeStopProxyPair:
		var val StopProxyPair
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding PCP StopProxyPair value: %v", err)
		}
		return &val, nil
	case CodeStartProxyPairResponse:
		var val StartProxyPairResponse
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding PCP StartProxyPairResponse value: %v", err)
		}
		return &val, nil
	default:
		return nil, fmt.Errorf("Unknown PCP code: %v", code)
	}
}
