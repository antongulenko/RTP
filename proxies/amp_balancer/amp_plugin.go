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

func (handler *ampBalancingHandler) NewSession(containingSession *balancingSession, desc *amp.StartStream) (BalancingSession, error) {
	client, ok := containingSession.Server.Client.(amp.CircuitBreaker)
	if !ok {
		return nil, fmt.Errorf("Illegal client type for pcpBalancingHandler: %T", containingSession.Server.Client)
	}
	err := client.StartStream(desc.ReceiverHost, desc.Port, desc.MediaFile)
	if err != nil {
		return nil, err
	}
	return &ampBalancingSession{
		client:       client,
		receiverHost: desc.ReceiverHost,
		receiverPort: desc.Port,
	}, nil
}

func (session *ampBalancingSession) StopRemote() error {
	return session.client.StopStream(session.receiverHost, session.receiverPort)
}

func (session *ampBalancingSession) BackgroundStopRemote() {
	client := session.client
	host := session.receiverHost
	port := session.receiverPort
	go client.StopStream(host, port) // Drops error
}

func (session *ampBalancingSession) RedirectStream(newHost string, newPort int) error {
	err := session.client.RedirectStream(session.receiverHost, session.receiverPort, newHost, newPort)
	if err != nil {
		return err
	}
	session.receiverHost = newHost
	session.receiverPort = newPort
	return nil
}

func (session *ampBalancingSession) HandleServerFault() (*BackendServer, error) {
	return nil, fmt.Errorf("Fault handling not implemented for AMP servers")
}
