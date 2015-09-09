package amp

import (
	"fmt"
	"net"

	"github.com/antongulenko/RTP/protocols"
)

type AmpServer struct {
	*protocols.Server
	handler AmpHandler
}

type AmpHandler interface {
	StartSession(val *StartSessionValue) error
	StopSession(val *StopSessionValue) error
	StopServer()
}

func NewAmpServer(local_addr string, handler AmpHandler) (server *AmpServer, err error) {
	server = &AmpServer{handler: handler}
	server.Server, err = protocols.NewServer(local_addr, server)
	if err != nil {
		server = nil
	}
	return
}

func (server *AmpServer) StopServer() {
	server.handler.StopServer()
}

func (server *AmpServer) ReceivePacket(conn *net.UDPConn) (*protocols.Packet, error) {
	packet, err := ReceivePacket(conn)
	if err != nil {
		return nil, err
	}
	return packet.Packet, err
}

func (server *AmpServer) HandleRequest(request *protocols.Packet) {
	packet := &AmpPacket{request}
	switch packet.Code {
	case CodeStartSession:
		if desc := packet.StartSession(); desc == nil {
			server.ReplyError(packet.Packet, fmt.Errorf("Illegal value for AMP CodeStartSession: %v", packet.Val))
		} else {
			server.ReplyCheck(packet.Packet, server.handler.StartSession(desc))
		}
	case CodeStopSession:
		if desc := packet.StopSession(); desc == nil {
			server.ReplyError(packet.Packet, fmt.Errorf("Illegal value for AMP CodeStopSession: %v", packet.Val))
		} else {
			server.ReplyCheck(packet.Packet, server.handler.StopSession(desc))
		}
	default:
		server.LogError(fmt.Errorf("Received unexpected AMP code: %v", packet.Code))
	}
}
