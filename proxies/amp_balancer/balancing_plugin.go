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

type BackendServerSlice []*BackendServer

// Implement sort.Interface
func (slice BackendServerSlice) Len() int {
	return len(slice)
}
func (slice BackendServerSlice) Less(i, j int) bool {
	return slice[i].Load < slice[j].Load
}
func (slice BackendServerSlice) Swap(i, j int) {
	tmp := slice[i]
	slice[i], slice[j] = slice[j], tmp
}

func (slice BackendServerSlice) pickServer(client string) (primary *BackendServer, backups BackendServerSlice) {
	// Lowest loaded server for the primary, next lowest loaded servers for backups
	sort.Sort(slice)
	backups = make(BackendServerSlice, 0, num_backup_servers)
	for i := 0; i < len(slice) && len(backups) < num_backup_servers; i++ {
		if server := slice[i]; server.Client.Online() {
			if primary == nil {
				primary = server
			} else {
				backups = append(backups, server)
			}
		}
	}
	return
}

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

type BackendServer struct {
	Addr           *net.UDPAddr
	LocalAddr      *net.UDPAddr
	Client         protocols.CircuitBreaker
	Sessions       map[*balancingSession]bool
	BackupSessions uint
	Load           float64 // 1 per session + backup_session_weight per backup session
}

type BalancingSession interface {
	StopRemote() error
	RedirectStream(newHost string, newPort int) error
	HandleServerFault() error
}

type balancingSession struct {
	// Small wrapper for implementing the PluginSession interface
	BalancingSession
	Client            string
	Server            *BackendServer
	BackupServers     BackendServerSlice
	containingSession *ampServerSession
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
	server.Sessions[wrapper] = true
	server.Load++
	for _, backup := range backups {
		backup.BackupSessions++
		backup.Load += backup_session_weight
	}
	return wrapper, nil
}

func (plugin *BalancingPlugin) Stop(containingServer *protocols.Server) (errors []error) {
	for _, server := range plugin.Servers {
		if err := server.Client.Close(); err != nil {
			errors = append(errors, fmt.Errorf("Error closing connection to %s: %v", server.Client, err))
		}
	}
	return
}

func (plugin *BalancingPlugin) serverStateChanged(key interface{}) {
	server, ok := key.(*BackendServer)
	if !ok {
		plugin.Server.LogError(fmt.Errorf("Could not handle server fault: Failed to convert %v (%T) to *BackendServer", key, key))
		return
	}
	if err := server.Client.Error(); err != nil {
		// Server fault detected!
		if len(server.Sessions) == 0 {
			plugin.Server.LogError(fmt.Errorf("Backend server %v is down, but no clients are affected (%v)", server.Client, err))
			return
		}
		for session := range server.Sessions {
			go func() {
				if err := session.HandleServerFault(); err != nil {
					plugin.Server.LogError(fmt.Errorf("Could not handle server fault for session %v: %v", session.Client, err))
				}
			}()
		}
	}
}

func (session *balancingSession) Start() {
	// Nothing to do. BalancingSession.NewSession() fully starts the session.
}

func (session *balancingSession) Observees() []helpers.Observee {
	return nil // No observees
}

func (session *balancingSession) Cleanup() error {
	delete(session.Server.Sessions, session)
	session.Server.Load--
	for _, backup := range session.BackupServers {
		backup.BackupSessions--
		backup.Load -= backup_session_weight
	}
	return session.StopRemote()
}
