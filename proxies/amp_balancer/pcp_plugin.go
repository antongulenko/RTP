package amp_balancer

import (
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/pcp"
)

type pcpBalancingPluginHandler struct {
	pcp.PcpProtocol
}

type pcpBalancingSessionHandler struct {
	*BalancingSession
	client    pcp.CircuitBreaker
	proxyHost string
	proxyPort int
}

func NewPcpBalancingPlugin() *BalancingPlugin {
	return NewBalancingPlugin(new(pcpBalancingPluginHandler))
}

func (handler *pcpBalancingPluginHandler) NewClient(localAddr string) (protocols.CircuitBreaker, error) {
	return pcp.NewCircuitBreaker(localAddr)
}

func (handler *pcpBalancingPluginHandler) Protocol() protocols.Protocol {
	return handler
}

func (handler *pcpBalancingPluginHandler) NewBalancingSessionHandler(session *BalancingSession) BalancingSessionHandler {
	client, ok := session.Server.Client.(pcp.CircuitBreaker)
	if !ok {
		return nil
	}
	return &pcpBalancingSessionHandler{
		BalancingSession: session,
		client:           client,
	}
}

func (handler *pcpBalancingSessionHandler) StartRemote() error {
	// TODO the proxy host should be specified
	resp, err := handler.client.StartProxyPair(handler.Desc.ReceiverHost, handler.Desc.Port, handler.Desc.Port+1)
	if err != nil {
		return err
	}
	handler.proxyHost = resp.ProxyHost
	handler.proxyPort = resp.ProxyPort1
	return nil
}

func (handler *pcpBalancingSessionHandler) StopRemote() error {
	return handler.client.StopProxyPair(handler.proxyPort)
}
