package amp

// "A Media Protocol"
// pseudo-protocol to initiate an RTP/RTCP media stream and controll other
// miscellaneous activities (load balancer, udp proxy). Transport: UDP.

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"

	"github.com/antongulenko/RTP/protocols"
)

const (
	ReceiveBuffer = 512
)

const (
	CodeStartSession = protocols.CodeOther + iota
	CodeStopSession
)

type AmpPacket struct {
	*protocols.Packet
}

func NewPacket(code uint, val interface{}) *AmpPacket {
	return &AmpPacket{
		&protocols.Packet{
			Code: code,
			Val:  val,
		},
	}
}

type StartSessionValue struct {
	MediaFile    string
	ReceiverHost string
	Port         int
}

type StopSessionValue struct {
	ReceiverHost string
	Port         int
}

func (packet *AmpPacket) StartSession() (res *StartSessionValue) {
	if packet.Code == CodeStartSession {
		res, _ = packet.Val.(*StartSessionValue)
	}
	return
}

func (packet *AmpPacket) StopSession() (res *StopSessionValue) {
	if packet.Code == CodeStopSession {
		res, _ = packet.Val.(*StopSessionValue)
	}
	return
}

func ReadPacket(reader io.Reader) (*AmpPacket, error) {
	packet, err := protocols.ReadPacket(reader, ampProtocolReader)
	if err != nil {
		return nil, err
	}
	return &AmpPacket{packet}, nil
}

func ReceivePacket(conn *net.UDPConn) (*AmpPacket, error) {
	packet, err := protocols.ReceivePacket(conn, ReceiveBuffer, ampProtocolReader)
	if err != nil {
		return nil, err
	}
	return &AmpPacket{packet}, nil
}

func (packet *AmpPacket) SendRequest(conn *net.UDPConn, addr *net.UDPAddr) (*protocols.Packet, error) {
	return packet.SendRawRequest(conn, addr, ReceiveBuffer, ampProtocolReader)
}

func (packet *AmpPacket) SendAmpRequest(conn *net.UDPConn, addr *net.UDPAddr) (reply *AmpPacket, err error) {
	var rawReply *protocols.Packet
	if rawReply, err = packet.SendRequest(conn, addr); err != nil {
		reply = &AmpPacket{rawReply}
	}
	return
}

func ampProtocolReader(code uint, dec *gob.Decoder) (interface{}, error) {
	switch code {
	case CodeStartSession:
		var val StartSessionValue
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding AMP StartSession value: %v", err)
		}
		return &val, nil
	case CodeStopSession:
		var val StopSessionValue
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding AMP StopSession value: %v", err)
		}
		return &val, nil
	default:
		return nil, fmt.Errorf("Unknown AMP code: %v", code)
	}
}
