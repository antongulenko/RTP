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

var (
	Protocol     *pcpProtocol
	MiniProtocol = protocols.NewMiniProtocol(Protocol)
)

const (
	codeStartProxy = protocols.Code(10 + iota)
	codeStopProxy
	codeStartProxyPair
	codeStopProxyPair
	codeStartProxyPairResponse
)

// ======================= Packets =======================

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
	ProxyHost     string
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

// ======================= Protocol =======================

type pcpProtocol struct {
}

func (*pcpProtocol) Name() string {
	return "PCP"
}

func (proto *pcpProtocol) Decoders() protocols.DecoderMap {
	return protocols.DecoderMap{
		codeStartProxy:             proto.decodeStartProxy,
		codeStopProxy:              proto.decodeStopProxy,
		codeStartProxyPair:         proto.decodeStartProxyPair,
		codeStopProxyPair:          proto.decodeStopProxyPair,
		codeStartProxyPairResponse: proto.decodeStartProxyPairResponse,
	}
}

func (proto *pcpProtocol) decodeStartProxy(decoder *gob.Decoder) (interface{}, error) {
	var val StartProxy
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding PCP StartProxy value: %v", err)
	}
	return &val, nil
}
func (proto *pcpProtocol) decodeStopProxy(decoder *gob.Decoder) (interface{}, error) {
	var val StopProxy
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding PCP StopProxy value: %v", err)
	}
	return &val, nil
}
func (proto *pcpProtocol) decodeStartProxyPair(decoder *gob.Decoder) (interface{}, error) {
	var val StartProxyPair
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding PCP StartProxyPair value: %v", err)
	}
	return &val, nil
}
func (proto *pcpProtocol) decodeStopProxyPair(decoder *gob.Decoder) (interface{}, error) {
	var val StopProxyPair
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding PCP StopProxyPair value: %v", err)
	}
	return &val, nil
}
func (proto *pcpProtocol) decodeStartProxyPairResponse(decoder *gob.Decoder) (interface{}, error) {
	var val StartProxyPairResponse
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding PCP StartProxyPairResponse value: %v", err)
	}
	return &val, nil
}
