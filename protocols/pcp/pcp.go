package pcp

// "Proxy Control Protocol"
// For controlling the udp proxy

import (
	"encoding/gob"
	"fmt"
	"github.com/antongulenko/RTP/protocols"
	"io"
	"net"
)

const (
	ReceiveBuffer = 512
)

const (
	CodeStartProxySession = protocols.CodeOther + iota
	CodeStopProxySession
)

type PcpPacket struct {
	protocols.Packet
}

func NewPacket(code uint, val interface{}) *PcpPacket {
	return &PcpPacket{
		protocols.Packet{
			Code: code,
			Val:  val,
		},
	}
}

type StartProxySession struct {
	port     uint
	clientIP string
}

type StopProxySession StartProxySession

func (packet *PcpPacket) StartProxySession() (res *StartProxySession) {
	if packet.Code == CodeStartProxySession {
		res, _ = packet.Val.(*StartProxySession)
	}
	return
}

func (packet *PcpPacket) StopProxySession() (res *StopProxySession) {
	if packet.Code == CodeStopProxySession {
		res, _ = packet.Val.(*StopProxySession)
	}
	return
}

func ReadPacket(reader io.Reader) (*PcpPacket, error) {
	packet, err := protocols.ReadPacket(reader, pcpProtocolReader)
	if err != nil {
		return nil, err
	}
	return &PcpPacket{*packet}, nil
}

func ReceivePacket(conn *net.UDPConn) (*PcpPacket, error) {
	packet, err := protocols.ReceivePacket(conn, ReceiveBuffer, pcpProtocolReader)
	if err != nil {
		return nil, err
	}
	return &PcpPacket{*packet}, nil
}

func (packet *PcpPacket) SendPcpRequest(conn *net.UDPConn, addr *net.UDPAddr) (pcpReply *PcpPacket, err error) {
	var reply *protocols.Packet
	if reply, err := packet.SendReqest(conn, addr, ReceiveBuffer, pcpProtocolReader); err == nil {
		pcpReply = &PcpPacket{*reply}
	}
	return
}

func pcpProtocolReader(code uint, dec *gob.Decoder) (interface{}, error) {
	switch code {
	case CodeStartProxySession:
		var val StartProxySession
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding PCP StartProxySession value: %v", err)
		}
		return &val, nil
	case CodeStopProxySession:
		var val StopProxySession
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding PCP StopProxySession value: %v", err)
		}
		return &val, nil
	default:
		return nil, fmt.Errorf("Unknown PCP code: %v", code)
	}
}
