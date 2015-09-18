package amp_balancer

import (
	"fmt"
	"net"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

type AmpPlugin struct {
	Servers []*MediaServer
}

type AmpPluginSession struct {
	server *MediaServer

	receiverHost string
	receiverPort int
	mediaFile    string
}

type MediaServer struct {
	Addr      *net.UDPAddr
	LocalAddr *net.UDPAddr
	Client    amp.CircuitBreaker
}

func NewAmpPlugin() *AmpPlugin {
	return &AmpPlugin{
		Servers: make([]*MediaServer, 0, 10),
	}
}

func (plugin *AmpPlugin) AddMediaServer(addr string, stateCallback protocols.CircuitBreakerCallback) error {
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
	client, err := amp.NewCircuitBreaker(localAddr.IP.String())
	if err != nil {
		return err
	}
	err = client.SetServer(serverAddr.String())
	if err != nil {
		return err
	}
	client.SetStateChangedCallback(stateCallback)
	client.Start()
	plugin.Servers = append(plugin.Servers, &MediaServer{
		Addr:      serverAddr,
		LocalAddr: localAddr,
		Client:    client,
	})
	return nil
}

func (plugin *AmpPlugin) pickServer(client string) *MediaServer {
	// TODO implement load balancing
	for _, server := range plugin.Servers {
		if server.Client.Online() {
			return server
		}
	}
	return nil
}

func (plugin *AmpPlugin) NewSession(desc *amp.StartStream) (PluginSession, error) {
	clientAddr := desc.Client()
	server := plugin.pickServer(clientAddr)
	if server == nil {
		return nil, fmt.Errorf("No server available to handle your request")
	}
	session := &AmpPluginSession{
		server:       server,
		receiverHost: desc.ReceiverHost,
		receiverPort: desc.Port,
		mediaFile:    desc.MediaFile,
	}
	if err := session.startRemote(); err != nil {
		return nil, fmt.Errorf("Error delegating request: %v", err)
	}
	return session, nil
}

func (plugin *AmpPlugin) Stop(containingServer *protocols.Server) {
	for _, server := range plugin.Servers {
		if err := server.Client.Close(); err != nil {
			containingServer.LogError(fmt.Errorf("Error closing connection to %v: %v", server.Addr, err))
		}
	}
}

func (session *AmpPluginSession) Start() {
	// Nothing to do. startRemote() starts the sessions must already be called in NewSession
}

func (session *AmpPluginSession) Observees() []helpers.Observee {
	return nil // No observees
}

func (session *AmpPluginSession) startRemote() error {
	return session.server.Client.StartStream(session.receiverHost, session.receiverPort, session.mediaFile)
}

func (session *AmpPluginSession) Cleanup() error {
	return session.server.Client.StopStream(session.receiverHost, session.receiverPort)
}
