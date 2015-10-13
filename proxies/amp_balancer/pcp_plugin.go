package amp_balancer

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/protocols/balancer"
	"github.com/antongulenko/RTP/protocols/pcp"
)

type pcpBalancingHandler struct {
}

type pcpBalancingSession struct {
	balancingSession *balancer.BalancingSession
	client           *pcp.Client
	receiverHost     string
	receiverPort     int
	proxyHost        string
	proxyPort        int
}

func NewPcpBalancingPlugin(make_detector balancer.FaultDetectorFactory) *balancer.BalancingPlugin {
	return balancer.NewBalancingPlugin(new(pcpBalancingHandler), make_detector)
}

func (handler *pcpBalancingHandler) NewClient(localAddr string, detector protocols.FaultDetector) (protocols.CircuitBreaker, error) {
	return protocols.NewCircuitBreakerOn(localAddr, pcp.MiniProtocol, detector)
}

func (handler *pcpBalancingHandler) Protocol() protocols.Protocol {
	return pcp.MiniProtocol
}

func (handler *pcpBalancingHandler) NewSession(balancingSession *balancer.BalancingSession, param protocols.SessionParameter) (balancer.BalancingSessionHandler, error) {
	desc, ok := param.(*amp.StartStream)
	if !ok {
		return nil, fmt.Errorf("Illegal session parameter type for pcpBalancingHandler: %T", param)
	}
	client, err := pcp.NewClient(balancingSession.PrimaryServer.Client)
	if err != nil {
		return nil, err
	}

	proxyHost := client.Server().IP().String()
	// TODO the address for receiving traffic could be different from the protocol-API
	// Check the address of the sending session plugin..?
	resp, err := client.StartProxyPair(proxyHost, desc.ReceiverHost, desc.Port, desc.Port+1)
	if err != nil {
		return nil, err
	}
	session := &pcpBalancingSession{
		client:           client,
		balancingSession: balancingSession,
		receiverHost:     desc.ReceiverHost,
		receiverPort:     desc.Port,
		proxyHost:        resp.ProxyHost,
		proxyPort:        resp.ProxyPort1,
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

func (session *pcpBalancingSession) HandleServerFault() (*balancer.BackendServer, error) {
	// Check preconditions for failover
	sending := session.balancingSession.SendingSession
	sending2, ok := sending.(*balancer.BalancingSession)
	ampSession, ok2 := sending2.Handler.(*ampBalancingSession)
	if !ok || !ok2 {
		return nil, fmt.Errorf("Can only handle PCP fault when sending session is *ampBalancingSession. Instead have %v (%T)", sending, sending)
	}

	// Find and initialize backup server
	var usedBackup *balancer.BackendServer
	var pcpBackup *pcp.Client
	var resp *pcp.StartProxyPairResponse
	for _, backup := range session.balancingSession.BackupServers {
		var err error
		pcpBackup, err = pcp.NewClient(backup.Client)
		// TODO log errors that prevented a backup server from being used?
		if err == nil {
			var err error
			proxyHost := pcpBackup.Server().IP().String()
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
