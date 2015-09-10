package pcp

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

type Server struct {
	*protocols.Server
	*pcpProtocol
	handler Handler
}

type Handler interface {
	StartProxy(val *StartProxy) error
	StopProxy(val *StopProxy) error
	StopServer()
}

func NewServer(local_addr string, handler Handler) (server *Server, err error) {
	if handler == nil {
		return nil, fmt.Errorf("Need non-nil pcp.Handler")
	}
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
	default:
		server.LogError(fmt.Errorf("Received unexpected Pcp code: %v", packet.Code))
	}
}
