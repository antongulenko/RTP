package heartbeat

import (
	"fmt"
	"time"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
)

// ======================= Receiving heartbeats =======================

type HeartbeatServerHandler interface {
	HeartbeatReceived(source protocols.Addr, beat *HeartbeatPacket)
}

func RegisterServerHandler(server *protocols.Server, handler HeartbeatServerHandler) error {
	if err := server.Protocol().CheckIncludesFragment(Protocol.Name()); err != nil {
		return err
	}
	return server.RegisterHandlers(protocols.ServerHandlerMap{
		codeHeartbeat: func(packet *protocols.Packet) {
			val := packet.Val
			if beat, ok := val.(*HeartbeatPacket); ok {
				handler.HeartbeatReceived(packet.SourceAddr, beat)
			} else {
				server.LogError(fmt.Errorf("Heartbeat received with wrong payload: (%T) %v", val, val))
			}
		},
	})
}

// ======================= Sending heartbeats =======================

type serverState struct {
	*protocols.Server
	heartbeatRunning  *helpers.OneshotCondition
	heartbeatReceiver protocols.Addr
	heartbeatTimeout  time.Duration
	heartbeatSeq      uint64
}

func (proto *heartbeatProtocol) ServerHandlers(server *protocols.Server) protocols.ServerHandlerMap {
	state := &serverState{
		Server:           server,
		heartbeatRunning: helpers.NewOneshotCondition(),
	}
	return protocols.ServerHandlerMap{
		codeConfigureHeartbeat: state.handleConfigureHeartbeat,
	}
}

func (server *serverState) handleConfigureHeartbeat(request *protocols.Packet) {
	val := request.Val
	if conf, ok := val.(*ConfigureHeartbeatPacket); ok {
		server.ReplyCheck(request, server.configureHeartbeat(conf.TargetServer, conf.Timeout))
	} else {
		err := fmt.Errorf("ConfigureHeartbeat received with wrong payload: (%T) %v", val, val)
		server.ReplyError(request, err)
	}
}

func (server *serverState) configureHeartbeat(receiver string, timeout time.Duration) error {
	var addr protocols.Addr
	if receiver != "" {
		var err error
		addr, err = protocols.Transport.Resolve(receiver)
		if err != nil {
			return fmt.Errorf("Failed to resolve heartbeat-receiver %s: %v", receiver, err)
		}
	}

	// TODO potential race condition. This should only be called once.
	// TODO once started the sendHeartbeats routine will keep spinning even if heartbeats are disabled again
	server.heartbeatTimeout = timeout
	server.heartbeatReceiver = addr
	server.heartbeatSeq = 0
	if addr != nil && timeout > 0 {
		// Not really an error.
		server.LogError(fmt.Errorf("Sending heartbeats to %s every %v", server.heartbeatReceiver, timeout))
		server.heartbeatRunning.Enable(server.sendHeartbeats)
	} else {
		server.LogError(fmt.Errorf("Stopped sending heartbeats"))
	}
	return nil
}

func (server *serverState) sendHeartbeats() {
	server.Wg.Add(1)
	go func() {
		defer server.Wg.Done()
		for !server.Stopped {
			receiver := server.heartbeatReceiver
			timeout := server.heartbeatTimeout
			if receiver != nil && timeout != 0 {
				packet := &protocols.Packet{
					Code: codeHeartbeat,
					Val: HeartbeatPacket{
						TimeSent: time.Now(),
						Seq:      server.heartbeatSeq,
					},
				}
				// Special routine for sending heartbeats to allow using the server port as source address
				server.heartbeatSeq++
				err := server.SendPacket(packet, receiver)
				if server.Stopped {
					break
				}
				if err != nil {
					server.LogError(fmt.Errorf("Error sending heartbeat to %v: %v", receiver, err))
				}
			}
			if timeout == 0 {
				timeout = 1 * time.Second
			}
			time.Sleep(timeout)
		}
	}()
}
