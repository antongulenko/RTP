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
	parent       *balancingSession
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

func (handler *pcpBalancingHandler) NewSession(containingSession *balancingSession, desc *amp.StartStream) (BalancingSession, error) {
	client, ok := containingSession.Server.Client.(pcp.CircuitBreaker)
	if !ok {
		return nil, fmt.Errorf("Illegal client type for pcpBalancingHandler: %T", containingSession.Server.Client)
	}
	proxyHost := client.Server().IP.String()
	// TODO the address for receiving traffic could be different from the protocol-API
	// Check the address of the sending session plugin..?
	resp, err := client.StartProxyPair(proxyHost, desc.ReceiverHost, desc.Port, desc.Port+1)
	if err != nil {
		return nil, err
	}
	session := &pcpBalancingSession{
		client:       client,
		parent:       containingSession,
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

func (session *pcpBalancingSession) BackgroundStopRemote() {
	client := session.client
	port := session.proxyPort
	go client.StopProxyPair(port) // Drops error
}

func (session *pcpBalancingSession) RedirectStream(newHost string, newPort int) error {
	return fmt.Errorf("RedirectStream not implemented for pcp balancer plugin")
}

func (session *pcpBalancingSession) HandleServerFault() (*BackendServer, error) {
	// Check preconditions for failover
	sending := session.parent.sendingSession
	sending2, ok := sending.(*balancingSession)
	ampSession, ok2 := sending2.BalancingSession.(*ampBalancingSession)
	if !ok || !ok2 {
		return nil, fmt.Errorf("Can only handle PCP fault when sending session is *ampBalancingSession. Instead have %v (%T)", sending, sending)
	}

	// Find and initialize backup server
	var usedBackup *BackendServer
	var pcpBackup pcp.CircuitBreaker
	var resp *pcp.StartProxyPairResponse
	for _, backup := range session.parent.BackupServers {
		var ok bool
		pcpBackup, ok = backup.Client.(pcp.CircuitBreaker)
		// TODO log errors that prevented a backup server from being used?
		if ok {
			var err error
			proxyHost := pcpBackup.Server().IP.String()
			// TODO The proxyHost could be different. See the comment above in NewSession.
			resp, err = pcpBackup.StartProxyPair(proxyHost, session.receiverHost, session.receiverPort, session.receiverPort+1)
			if err == nil {
				usedBackup = backup
				break
			}
		}
	}
	if usedBackup == nil {
		return nil, fmt.Errorf("No valid backup server for %s!", session.client)
	}
	session.client = pcpBackup
	session.proxyHost = resp.ProxyHost
	session.proxyPort = resp.ProxyPort1

	// Try to redirect the stream to the new proxy.
	err := ampSession.RedirectStream(resp.ProxyHost, resp.ProxyPort1)
	if err != nil {
		return nil, err
	}
	return usedBackup, nil
}
