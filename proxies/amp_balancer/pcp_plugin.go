package amp_balancer

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/protocols/pcp"
)

type pcpBalancingHandler struct {
	pcp.PcpProtocol
}

type pcpBalancingSession struct {
	client       pcp.CircuitBreaker
	receiverHost string
	receiverPort int
	proxyHost    string
	proxyPort    int
}

func NewPcpBalancingPlugin() *BalancingPlugin {
	return NewBalancingPlugin(new(pcpBalancingHandler))
}

func (handler *pcpBalancingHandler) NewClient(localAddr string) (protocols.CircuitBreaker, error) {
	return pcp.NewCircuitBreaker(localAddr)
}

func (handler *pcpBalancingHandler) Protocol() protocols.Protocol {
	return handler
}

func (handler *pcpBalancingHandler) NewSession(server *BackendServer, desc *amp.StartStream) (BalancingSession, error) {
	client, ok := server.Client.(pcp.CircuitBreaker)
	if !ok {
		return nil, fmt.Errorf("Illegal client type for pcpBalancingHandler: %T", server.Client)
	}
	resp, err := client.StartProxyPair(desc.ReceiverHost, desc.Port, desc.Port+1)
	if err != nil {
		return nil, err
	}
	session := &pcpBalancingSession{
		client:       client,
		receiverHost: desc.ReceiverHost,
		receiverPort: desc.Port,
		proxyHost:    resp.ProxyHost,
		proxyPort:    resp.ProxyPort1,
	}
	// Make sure the next plugin sends the data to the proxies instead of the actual client
	desc.ReceiverHost = resp.ProxyHost
	desc.Port = resp.ProxyPort1
	return session, nil
}

func (session *pcpBalancingSession) StopRemote() error {
	return session.client.StopProxyPair(session.proxyPort)
}

func (session *pcpBalancingSession) RedirectStream(newHost string, newPort int) error {
	return fmt.Errorf("RedirectRemote not implemented for pcp balancer plugin")
}
