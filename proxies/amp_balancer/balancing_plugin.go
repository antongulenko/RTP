package amp_balancer

import (
	"fmt"
	"net"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

type BalancingPlugin struct {
	Servers []*BackendServer
	handler BalancingHandler
}

type BalancingHandler interface {
	NewClient(localAddr string) (protocols.CircuitBreaker, error)
	NewSession(server *BackendServer, desc *amp.StartStream) (BalancingSession, error) // Modify desc if necessary, do not store it.
	Protocol() protocols.Protocol
}

type BackendServer struct {
	Addr      *net.UDPAddr
	LocalAddr *net.UDPAddr
	Client    protocols.CircuitBreaker
}

type BalancingSession interface {
	StopRemote() error
	RedirectStream(newHost string, newPort int) error
}

type balancingSession struct {
	// Small wrapper for implementing the PluginSession interface
	BalancingSession
	server *BackendServer
}

func NewBalancingPlugin(handler BalancingHandler) *BalancingPlugin {
	return &BalancingPlugin{
		handler: handler,
		Servers: make([]*BackendServer, 0, 10),
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
	client.SetStateChangedCallback(stateCallback)
	client.Start()
	plugin.Servers = append(plugin.Servers, &BackendServer{
		Addr:      serverAddr,
		LocalAddr: localAddr,
		Client:    client,
	})
	return nil
}

func (plugin *BalancingPlugin) pickServer(client string) *BackendServer {
	// TODO implement load balancing
	for _, server := range plugin.Servers {
		if server.Client.Online() {
			return server
		}
	}
	return nil
}

func (plugin *BalancingPlugin) NewSession(desc *amp.StartStream) (PluginSession, error) {
	clientAddr := desc.Client()
	server := plugin.pickServer(clientAddr)
	if server == nil {
		return nil, fmt.Errorf("No %s server available to handle your request", plugin.handler.Protocol().Name())
	}
	session, err := plugin.handler.NewSession(server, desc)
	if err != nil {
		return nil, fmt.Errorf("Failed to create %s session: %s", plugin.handler.Protocol().Name(), err)
	}
	return &balancingSession{
		BalancingSession: session,
		server:           server,
	}, nil
}

func (plugin *BalancingPlugin) Stop(containingServer *protocols.Server) {
	for _, server := range plugin.Servers {
		if err := server.Client.Close(); err != nil {
			containingServer.LogError(fmt.Errorf("Error closing connection to %s: %v", server.Client, err))
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
	return session.StopRemote()
}
