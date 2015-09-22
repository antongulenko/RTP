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
	NewBalancingSessionHandler(session *BalancingSession) BalancingSessionHandler
	Protocol() protocols.Protocol
}

type BackendServer struct {
	Addr      *net.UDPAddr
	LocalAddr *net.UDPAddr
	Client    protocols.CircuitBreaker
}

type BalancingSession struct {
	Server  *BackendServer
	Desc    *amp.StartStream
	handler BalancingSessionHandler
}

type BalancingSessionHandler interface {
	StartRemote() error
	StopRemote() error
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
	session := &BalancingSession{
		Server: server,
		Desc:   desc,
	}
	session.handler = plugin.handler.NewBalancingSessionHandler(session)
	if session.handler == nil {
		return nil, fmt.Errorf("Failed to create BalancingSessionHandler")
	}
	if err := session.handler.StartRemote(); err != nil {
		return nil, fmt.Errorf("Error delegating %s request: %v", plugin.handler.Protocol().Name(), err)
	}
	return session, nil
}

func (plugin *BalancingPlugin) Stop(containingServer *protocols.Server) {
	for _, server := range plugin.Servers {
		if err := server.Client.Close(); err != nil {
			containingServer.LogError(fmt.Errorf("Error closing connection to %s: %v", server.Client, err))
		}
	}
}

func (session *BalancingSession) Start() {
	// Nothing to do. StartRemote() starts the sessions and is called in NewSession
}

func (session *BalancingSession) Observees() []helpers.Observee {
	return nil // No observees
}

func (session *BalancingSession) Cleanup() error {
	return session.handler.StopRemote()
}
