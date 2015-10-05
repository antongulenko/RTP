package pcp

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

type Server struct {
	*protocols.Server
	PcpProtocolImpl
	handler Handler
}

type Handler interface {
	StartProxy(val *StartProxy) error
	StopProxy(val *StopProxy) error
	StartProxyPair(val *StartProxyPair) (*StartProxyPairResponse, error)
	StopProxyPair(val *StopProxyPair) error
	StopServer()
}

func NewServer(local_addr string, handler Handler) (server *Server, err error) {
	server = &Server{handler: handler}
	server.Server, err = protocols.NewServer(local_addr, server)
	if err != nil {
		server = nil
	}
	return
}

func (server *Server) StopServer() {
	server.handler.StopServer()
}

func (server *Server) HandleRequest(packet *protocols.Packet) {
	val := packet.Val
	switch packet.Code {
	case CodeStartProxy:
		if desc, ok := val.(*StartProxy); ok {
			server.ReplyCheck(packet, server.handler.StartProxy(desc))
		} else {
			server.ReplyError(packet, fmt.Errorf("Illegal value for Pcp StartProxy: %v", packet.Val))
		}
	case CodeStopProxy:
		if desc, ok := val.(*StopProxy); ok {
			server.ReplyCheck(packet, server.handler.StopProxy(desc))
		} else {
			server.ReplyError(packet, fmt.Errorf("Illegal value for Pcp StopProxy: %v", packet.Val))
		}
	case CodeStartProxyPair:
		if desc, ok := val.(*StartProxyPair); ok {
			reply, err := server.handler.StartProxyPair(desc)
			if err == nil {
				server.Reply(packet, CodeStartProxyPairResponse, reply)
			} else {
				server.ReplyError(packet, err)
			}
		} else {
			server.ReplyError(packet, fmt.Errorf("Illegal value for Pcp StartProxyPair: %v", packet.Val))
		}
	case CodeStopProxyPair:
		if desc, ok := val.(*StopProxyPair); ok {
			server.ReplyCheck(packet, server.handler.StopProxyPair(desc))
		} else {
			server.ReplyError(packet, fmt.Errorf("Illegal value for Pcp StopProxyPair: %v", packet.Val))
		}
	case CodeStartProxyPairResponse:
		server.LogError(fmt.Errorf("Received standalone StartProxyPairResponse message"))
	default:
		server.LogError(fmt.Errorf("Received unexpected Pcp code: %v", packet.Code))
	}
}
