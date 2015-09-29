package amp_balancer

import (
	"fmt"
	"net"
	"sort"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

const (
	num_backup_servers    = 1
	backup_session_weight = 0.1
)

type BalancingPlugin struct {
	Server  *ExtendedAmpServer
	Servers BackendServerSlice
	handler BalancingHandler
}

type BalancingHandler interface {
	NewClient(localAddr string) (protocols.CircuitBreaker, error)
	NewSession(containingSession *balancingSession, desc *amp.StartStream) (BalancingSession, error) // Modify desc if necessary, do not store it.
	Protocol() protocols.Protocol
}

type BalancingSession interface {
	StopRemote() error
	BackgroundStopRemote()
	RedirectStream(newHost string, newPort int) error
	HandleServerFault() (*BackendServer, error)
}

type balancingSession struct {
	BalancingSession
	Client            string
	Server            *BackendServer
	BackupServers     BackendServerSlice
	containingSession *ampServerSession
	sendingSession    PluginSession
	failoverError     error
}

func NewBalancingPlugin(handler BalancingHandler) *BalancingPlugin {
	return &BalancingPlugin{
		handler: handler,
		Servers: make(BackendServerSlice, 0, 10),
	}
}

func (plugin *BalancingPlugin) AddBackendServer(addr string, stateCallback protocols.CircuitBreakerCallback) error {
	serverAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return err
	}
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return fmt.Errorf("Failed to convert to net.UDPAddr: %v", conn.LocalAddr())
	}
	client, err := plugin.handler.NewClient(localAddr.IP.String())
	if err != nil {
		return err
	}
	err = client.SetServer(serverAddr.String())
	if err != nil {
		return err
	}
	server := &BackendServer{
		Addr:      serverAddr,
		LocalAddr: localAddr,
		Client:    client,
		Sessions:  make(map[*balancingSession]bool),
		Plugin:    plugin,
	}
	plugin.Servers = append(plugin.Servers, server)
	sort.Sort(plugin.Servers)
	client.AddStateChangedCallback(stateCallback, client)
	client.AddStateChangedCallback(plugin.serverStateChanged, server)
	client.Start()
	return nil
}

func (plugin *BalancingPlugin) Start(server *ExtendedAmpServer) {
	plugin.Server = server
}

func (plugin *BalancingPlugin) NewSession(containingSession *ampServerSession, desc *amp.StartStream) (PluginSession, error) {
	clientAddr := desc.Client()
	server, backups := plugin.Servers.pickServer(clientAddr)
	if server == nil {
		return nil, fmt.Errorf("No %s server available to handle your request", plugin.handler.Protocol().Name())
	}
	wrapper := &balancingSession{
		Server:            server,
		containingSession: containingSession,
		Client:            clientAddr,
		BackupServers:     backups,
	}
	var err error
	wrapper.BalancingSession, err = plugin.handler.NewSession(wrapper, desc)
	if err != nil {
		return nil, fmt.Errorf("Failed to create %s session: %s", plugin.handler.Protocol().Name(), err)
	}
	server.registerSession(wrapper)
	return wrapper, nil
}

func (plugin *BalancingPlugin) Stop() error {
	var errors helpers.MultiError
	for _, server := range plugin.Servers {
		if err := server.Client.Close(); err != nil {
			errors = append(errors, fmt.Errorf("Error closing connection to %s: %v", server.Client, err))
		}
	}
	return errors.NilOrError()
}

func (plugin *BalancingPlugin) serverStateChanged(key interface{}) {
	server, ok := key.(*BackendServer)
	if !ok {
		plugin.Server.LogError(fmt.Errorf("Could not handle server fault: Failed to convert %v (%T) to *BackendServer", key, key))
		return
	}
	server.handleStateChanged()
}

func (session *balancingSession) StopContainingSession() error {
	return session.containingSession.server.StopSession(session.Client)
}

func (session *balancingSession) LogServerError(err error) {
	session.containingSession.server.LogError(err)
}

func (session *balancingSession) Start(sendingSession PluginSession) {
	session.sendingSession = sendingSession
	// Nothing else to do. BalancingSession.NewSession() fully starts the session.
}

func (session *balancingSession) Observees() []helpers.Observee {
	return nil // No observees
}

func (session *balancingSession) Cleanup() error {
	session.Server.unregisterSession(session)
	if session.failoverError == nil {
		return session.StopRemote()
	} else {
		// Failover for the session failed previously. Don't try to close it anymore.
		return session.failoverError
	}
}
