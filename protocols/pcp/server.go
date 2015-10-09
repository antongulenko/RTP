package pcp

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

type Handler interface {
	StartProxy(val *StartProxy) error
	StopProxy(val *StopProxy) error
	StartProxyPair(val *StartProxyPair) (*StartProxyPairResponse, error)
	StopProxyPair(val *StopProxyPair) error
	StopServer()
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
		codeStartProxy:     state.handleStartProxy,
		codeStopProxy:      state.handleStopProxy,
		codeStartProxyPair: state.handleStartProxyPair,
		codeStopProxyPair:  state.handleStopProxyPair,
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

func (server *serverState) handleStartProxy(packet *protocols.Packet) {
	if desc, ok := packet.Val.(*StartProxy); ok {
		server.ReplyCheck(packet, server.handler.StartProxy(desc))
	} else {
		server.ReplyError(packet, fmt.Errorf("Illegal value for Pcp StartProxy: %v", packet.Val))
	}
}

func (server *serverState) handleStopProxy(packet *protocols.Packet) {
	if desc, ok := packet.Val.(*StopProxy); ok {
		server.ReplyCheck(packet, server.handler.StopProxy(desc))
	} else {
		server.ReplyError(packet, fmt.Errorf("Illegal value for Pcp StopProxy: %v", packet.Val))
	}
}

func (server *serverState) handleStartProxyPair(packet *protocols.Packet) {
	if desc, ok := packet.Val.(*StartProxyPair); ok {
		reply, err := server.handler.StartProxyPair(desc)
		if err == nil {
			server.Reply(packet, codeStartProxyPairResponse, reply)
		} else {
			server.ReplyError(packet, err)
		}
	} else {
		server.ReplyError(packet, fmt.Errorf("Illegal value for Pcp StartProxyPair: %v", packet.Val))
	}
}

func (server *serverState) handleStopProxyPair(packet *protocols.Packet) {
	if desc, ok := packet.Val.(*StopProxyPair); ok {
		server.ReplyCheck(packet, server.handler.StopProxyPair(desc))
	} else {
		server.ReplyError(packet, fmt.Errorf("Illegal value for Pcp StopProxyPair: %v", packet.Val))
	}
}
