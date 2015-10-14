package amp_control

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

type Handler interface {
	StopServer()
	RedirectStream(val *RedirectStream) error
	PauseStream(val *PauseStream) error
	ResumeStream(val *ResumeStream) error
}

func RegisterServer(server *protocols.Server, handler Handler) error {
	if err := server.Protocol().CheckIncludesFragment(Protocol.Name()); err != nil {
		return err
	}
	state := &serverState{
		Server:  server,
		handler: handler,
	}
	if err := server.RegisterHandlers(protocols.ServerHandlerMap{
		CodeRedirectStream: state.handleRedirectStream,
		CodePauseStream:    state.handlePauseStream,
		CodeResumeStream:   state.handleResumeStream,
	}); err != nil {
		return err
	}
	server.RegisterStopHandler(state.stopServer)
	return nil
}

type serverState struct {
	*protocols.Server
	handler Handler
}

func (server *serverState) stopServer() {
	server.handler.StopServer()
}

func (server *serverState) handleRedirectStream(packet *protocols.Packet) *protocols.Packet {
	val := packet.Val
	if desc, ok := val.(*RedirectStream); ok {
		return server.ReplyCheck(server.handler.RedirectStream(desc))
	} else {
		return server.ReplyError(fmt.Errorf("Illegal value for AMPcontrol RedirectStream: %v", packet.Val))
	}
}

func (server *serverState) handlePauseStream(packet *protocols.Packet) *protocols.Packet {
	val := packet.Val
	if desc, ok := val.(*PauseStream); ok {
		return server.ReplyCheck(server.handler.PauseStream(desc))
	} else {
		return server.ReplyError(fmt.Errorf("Illegal value for AMPcontrol PauseStream: %v", packet.Val))
	}
}

func (server *serverState) handleResumeStream(packet *protocols.Packet) *protocols.Packet {
	val := packet.Val
	if desc, ok := val.(*ResumeStream); ok {
		return server.ReplyCheck(server.handler.ResumeStream(desc))
	} else {
		return server.ReplyError(fmt.Errorf("Illegal value for AMPcontrol ResumeStream: %v", packet.Val))
	}
}
