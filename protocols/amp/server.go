package amp

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

type Server struct {
	*protocols.Server
	*AmpProtocol
	handler Handler
}

type Handler interface {
	StopServer()

	StartStream(val *StartStream) error
	StopStream(val *StopStream) error
	RedirectStream(val *RedirectStream) error
	PauseStream(val *PauseStream) error
	ResumeStream(val *ResumeStream) error
}

func NewServer(local_addr string, handler Handler) (server *Server, err error) {
	if handler == nil {
		return nil, fmt.Errorf("Need non-nil amp.Handler")
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
	case CodeStartStream:
		if desc, ok := val.(*StartStream); ok {
			server.ReplyCheck(packet, server.handler.StartStream(desc))
		} else {
			server.ReplyError(packet, fmt.Errorf("Illegal value for AMP StartStream: %v", packet.Val))
		}
	case CodeStopStream:
		if desc, ok := val.(*StopStream); ok {
			server.ReplyCheck(packet, server.handler.StopStream(desc))
		} else {
			server.ReplyError(packet, fmt.Errorf("Illegal value for AMP StopStream: %v", packet.Val))
		}
	case CodeRedirectStream:
		if desc, ok := val.(*RedirectStream); ok {
			server.ReplyCheck(packet, server.handler.RedirectStream(desc))
		} else {
			server.ReplyError(packet, fmt.Errorf("Illegal value for AMP RedirectStream: %v", packet.Val))
		}
	case CodePauseStream:
		if desc, ok := val.(*PauseStream); ok {
			server.ReplyCheck(packet, server.handler.PauseStream(desc))
		} else {
			server.ReplyError(packet, fmt.Errorf("Illegal value for AMP PauseStream: %v", packet.Val))
		}
	case CodeResumeStream:
		if desc, ok := val.(*ResumeStream); ok {
			server.ReplyCheck(packet, server.handler.ResumeStream(desc))
		} else {
			server.ReplyError(packet, fmt.Errorf("Illegal value for AMP ResumeStream: %v", packet.Val))
		}
	default:
		server.LogError(fmt.Errorf("Received unexpected AMP code: %v", packet.Code))
	}
}
