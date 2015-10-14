package amp

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

type Handler interface {
	StopServer()
	StartStream(val *StartStream) error
	StopStream(val *StopStream) error
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
		CodeStartStream: state.handleStartStream,
		CodeStopStream:  state.handleStopStream,
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

func (server *serverState) handleStartStream(packet *protocols.Packet) *protocols.Packet {
	val := packet.Val
	if desc, ok := val.(*StartStream); ok {
		return server.ReplyCheck(server.handler.StartStream(desc))
	} else {
		return server.ReplyError(fmt.Errorf("Illegal value for AMP StartStream: %v", packet.Val))
	}
}

func (server *serverState) handleStopStream(packet *protocols.Packet) *protocols.Packet {
	val := packet.Val
	if desc, ok := val.(*StopStream); ok {
		return server.ReplyCheck(server.handler.StopStream(desc))
	} else {
		return server.ReplyError(fmt.Errorf("Illegal value for AMP StopStream: %v", packet.Val))
	}
}
