package heartbeat

import (
	"fmt"
	"time"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/golib"
)

// ======================= Receiving heartbeats =======================

type HeartbeatServerHandler interface {
	HeartbeatReceived(beat *HeartbeatPacket)
}

func RegisterServerHandler(server *protocols.Server, handler HeartbeatServerHandler) error {
	if err := server.Protocol().CheckIncludesFragment(Protocol.Name()); err != nil {
		return err
	}
	return server.RegisterHandlers(protocols.ServerHandlerMap{
		codeHeartbeat: func(packet *protocols.Packet) *protocols.Packet {
			val := packet.Val
			if beat, ok := val.(*HeartbeatPacket); ok {
				handler.HeartbeatReceived(beat)
			} else {
				server.LogError(fmt.Errorf("Heartbeat received with wrong payload: (%T) %v", val, val))
			}
			return nil
		},
	})
}

// ======================= Sending heartbeats =======================

type serverState struct {
	*protocols.Server
	token            int64
	heartbeatClient  protocols.Client
	heartbeatRunning *golib.OneshotCondition
	heartbeatTimeout time.Duration
	heartbeatSeq     uint64
}

func (proto *heartbeatProtocol) ServerHandlers(server *protocols.Server) protocols.ServerHandlerMap {
	state := &serverState{
		Server:           server,
		heartbeatRunning: golib.NewOneshotCondition(),
	}
	return protocols.ServerHandlerMap{
		codeConfigureHeartbeat: state.handleConfigureHeartbeat,
	}
}

func (server *serverState) handleConfigureHeartbeat(request *protocols.Packet) *protocols.Packet {
	val := request.Val
	if conf, ok := val.(*ConfigureHeartbeatPacket); ok {
		return server.ReplyCheck(server.configureHeartbeat(conf.TargetServer, conf.Token, conf.Timeout))
	} else {
		err := fmt.Errorf("ConfigureHeartbeat received with wrong payload: (%T) %v", val, val)
		return server.ReplyError(err)
	}
}

func (server *serverState) configureHeartbeat(receiver string, token int64, timeout time.Duration) error {
	if server.heartbeatClient == nil {
		client, err := protocols.NewClientFor(receiver, server.Protocol())
		if err != nil {
			return err
		}
		client.SetTimeout(500 * time.Millisecond) // TODO this is arbitrary
		server.heartbeatClient = client
	}

	// TODO potential race condition. This entire thing should only be called once.
	// TODO once started the sendHeartbeats routine will keep spinning even if heartbeats are disabled again
	server.heartbeatSeq = 0
	err := server.heartbeatClient.SetServer(receiver)

	if err == nil && timeout > 0 && token != 0 {
		// Not really an error.
		server.LogError(fmt.Errorf("Sending heartbeats to %s every %v", server.heartbeatClient.Server(), timeout))
		server.heartbeatTimeout = timeout
		server.token = token
		server.heartbeatRunning.Enable(server.sendHeartbeats)
	} else {
		server.heartbeatTimeout = 0
		server.token = 0
		server.LogError(fmt.Errorf("Stopped sending heartbeats"))
		if err != nil {
			return fmt.Errorf("Failed to resolve heartbeat-receiver %s: %v", receiver, err)
		}
	}
	return nil
}

func (server *serverState) sendHeartbeats() {
	server.Wg.Add(1)
	go func() {
		defer server.Wg.Done()
		for !server.Stopped {
			timeout := server.heartbeatTimeout
			token := server.token
			if timeout != 0 && token != 0 {
				packet := &HeartbeatPacket{
					Token:    token,
					TimeSent: time.Now(),
					Seq:      server.heartbeatSeq,
				}
				server.heartbeatSeq++
				err := server.heartbeatClient.Send(codeHeartbeat, packet)
				if server.Stopped {
					break
				}
				if err != nil {
					server.LogError(fmt.Errorf("Error sending heartbeat to %v: %v", server.heartbeatClient.Server(), err))
				}
				server.heartbeatClient.ResetConnection()
			} else {
				// TODO can take up to 1 second until we start sending heartbeats
				timeout = 1 * time.Second
			}
			time.Sleep(timeout)
		}
	}()
}
