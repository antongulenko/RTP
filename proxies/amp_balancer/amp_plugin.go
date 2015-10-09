package amp_balancer

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/protocols/amp_control"
	"github.com/antongulenko/RTP/protocols/balancer"
)

type ampBalancingHandler struct {
}

type ampBalancingSession struct {
	client         *amp.Client
	control_client *amp_control.Client
	receiverHost   string
	receiverPort   int
}

func NewAmpBalancingPlugin(make_detector balancer.FaultDetectorFactory) *balancer.BalancingPlugin {
	return balancer.NewBalancingPlugin(new(ampBalancingHandler), make_detector)
}

func (handler *ampBalancingHandler) NewClient(localAddr string, detector protocols.FaultDetector) (protocols.CircuitBreaker, error) {
	proto, err := protocols.NewProtocol("AMP+control", amp.Protocol, amp_control.Protocol)
	if err != nil {
		return nil, err
	}
	return protocols.NewCircuitBreakerOn(localAddr, proto, detector)
}

func (handler *ampBalancingHandler) Protocol() protocols.Protocol {
	return amp.MiniProtocol
}

func (handler *ampBalancingHandler) NewSession(balancerSession *balancer.BalancingSession, param protocols.SessionParameter) (balancer.BalancingSessionHandler, error) {
	desc, ok := param.(*amp.StartStream)
	if !ok {
		return nil, fmt.Errorf("Illegal session parameter type for ampBalancingHandler: %T", param)
	}
	breaker := balancerSession.PrimaryServer.Client
	client, err := amp.NewClient(breaker)
	if err != nil {
		return nil, err
	}
	control_client, err := amp_control.NewClient(breaker)
	if err != nil {
		return nil, err
	}

	err = client.StartStream(desc.ReceiverHost, desc.Port, desc.MediaFile)
	if err != nil {
		return nil, err
	}
	return &ampBalancingSession{
		client:         client,
		control_client: control_client,
		receiverHost:   desc.ReceiverHost,
		receiverPort:   desc.Port,
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
	err := session.control_client.RedirectStream(session.receiverHost, session.receiverPort, newHost, newPort)
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
