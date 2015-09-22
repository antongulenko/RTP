package amp_balancer

import (
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

type ampBalancingPluginHandler struct {
	amp.AmpProtocol
}

type ampBalancingSessionHandler struct {
	*BalancingSession
	client amp.CircuitBreaker
}

func NewAmpBalancingPlugin() *BalancingPlugin {
	return NewBalancingPlugin(new(ampBalancingPluginHandler))
}

func (handler *ampBalancingPluginHandler) NewClient(localAddr string) (protocols.CircuitBreaker, error) {
	return amp.NewCircuitBreaker(localAddr)
}

func (handler *ampBalancingPluginHandler) Protocol() protocols.Protocol {
	return handler
}

func (handler *ampBalancingPluginHandler) NewBalancingSessionHandler(session *BalancingSession) BalancingSessionHandler {
	client, ok := session.Server.Client.(amp.CircuitBreaker)
	if !ok {
		return nil
	}
	return &ampBalancingSessionHandler{
		BalancingSession: session,
		client:           client,
	}
}

func (handler *ampBalancingSessionHandler) StartRemote() error {
	return handler.client.StartStream(handler.Desc.ReceiverHost, handler.Desc.Port, handler.Desc.MediaFile)
}

func (handler *ampBalancingSessionHandler) StopRemote() error {
	return handler.client.StopStream(handler.Desc.ReceiverHost, handler.Desc.Port)
}
