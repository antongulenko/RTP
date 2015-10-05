package amp_balancer

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/protocols/balancer"
)

type ampBalancingHandler struct {
	amp.AmpProtocol
}

type ampBalancingSession struct {
	client       amp.CircuitBreaker
	receiverHost string
	receiverPort int
}

func NewAmpBalancingPlugin() *balancer.BalancingPlugin {
	return balancer.NewBalancingPlugin(new(ampBalancingHandler))
}

func (handler *ampBalancingHandler) NewClient(localAddr string) (protocols.CircuitBreaker, error) {
	client, err := amp.NewClient(localAddr)
	if err != nil {
		return nil, err
	}
	detector := protocols.NewPingFaultDetector(protocols.ExtendClient(client))
	return amp.NewCircuitBreaker(localAddr, detector)
}

func (handler *ampBalancingHandler) Protocol() protocols.Protocol {
	return handler
}

func (handler *ampBalancingHandler) NewSession(balancerSession *balancer.BalancingSession, param protocols.SessionParameter) (balancer.BalancingSessionHandler, error) {
	client, ok := balancerSession.PrimaryServer.Client.(amp.CircuitBreaker)
	if !ok {
		return nil, fmt.Errorf("Illegal client type for pcpBalancingHandler: %T", balancerSession.PrimaryServer.Client)
	}
	desc, ok := param.(*amp.StartStream)
	if !ok {
		return nil, fmt.Errorf("Illegal session parameter type for ampBalancingHandler: %T", param)
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

func (session *ampBalancingSession) HandleServerFault() (*balancer.BackendServer, error) {
	return nil, fmt.Errorf("Fault handling not implemented for AMP servers")
}
