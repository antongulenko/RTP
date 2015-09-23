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
	resp, err := client.StartProxyPair(desc.ReceiverHost, desc.Port, desc.Port+1)
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

func (session *pcpBalancingSession) RedirectStream(newHost string, newPort int) error {
	return fmt.Errorf("RedirectStream not implemented for pcp balancer plugin")
}

func (session *pcpBalancingSession) HandleServerFault() error {
	// 1. Fencing: Stop the original node just to be sure.
	// TODO more reliable fencing.
	// TODO log the error?
	go session.client.StopProxyPair(session.proxyPort)

	// 2. Find and initialize backup server
	var usedBackup *BackendServer
	var pcpBackup pcp.CircuitBreaker
	var resp *pcp.StartProxyPairResponse
	for _, backup := range session.parent.BackupServers {
		var ok bool
		pcpBackup, ok = backup.Client.(pcp.CircuitBreaker)
		// TODO log errors that prevented a backup server from being used?
		if ok {
			var err error
			resp, err = pcpBackup.StartProxyPair(session.receiverHost, session.receiverPort, session.receiverPort+1)
			if err == nil {
				usedBackup = backup
				break
			}
		}
	}
	if usedBackup == nil {
		return fmt.Errorf("No valid backup server to failover %s session!", session.client)
	}

	// Try to redirect the stream to the new proxy.
	err := session.parent.containingSession.RedirectStream(session.parent, resp.ProxyHost, resp.ProxyPort1)
	if err != nil {
		return err
	}

	session.proxyHost = resp.ProxyHost
	session.proxyPort = resp.ProxyPort1
	session.client = pcpBackup

	// Remove the used backup server from the list of backups
	// TODO manage this list, add new backups
	session.parent.Server = usedBackup
	var newBackups = make([]*BackendServer, 0, len(session.parent.BackupServers)-1)
	for _, backup := range session.parent.BackupServers {
		if backup != usedBackup {
			newBackups = append(newBackups, backup)
		}
	}
	return nil
}
