package amp_balancer

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

type ampBalancingHandler struct {
	amp.AmpProtocol
}

type ampBalancingSession struct {
	client       amp.CircuitBreaker
	receiverHost string
	receiverPort int
}

func NewAmpBalancingPlugin() *BalancingPlugin {
	return NewBalancingPlugin(new(ampBalancingHandler))
}

func (handler *ampBalancingHandler) NewClient(localAddr string) (protocols.CircuitBreaker, error) {
	return amp.NewCircuitBreaker(localAddr)
}

func (handler *ampBalancingHandler) Protocol() protocols.Protocol {
	return handler
}

func (handler *ampBalancingHandler) NewSession(server *BackendServer, desc *amp.StartStream) (BalancingSession, error) {
	client, ok := server.Client.(amp.CircuitBreaker)
	if !ok {
		return nil, fmt.Errorf("Illegal client type for pcpBalancingHandler: %T", server.Client)
	}
	err := client.StartStream(desc.ReceiverHost, desc.Port, desc.MediaFile)
	if err != nil {
		return nil, err
	}
	return &ampBalancingSession{
		client: client,
	}, nil
}

func (session *ampBalancingSession) StopRemote() error {
	return session.client.StopStream(session.receiverHost, session.receiverPort)
}

func (session *ampBalancingSession) RedirectStream(newHost string, newPort int) error {
	return fmt.Errorf("RedirectRemote not implemented for amp balancer plugin")
}
