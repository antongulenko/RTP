package proxies

import (
	"fmt"
	"net"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

type AmpBalancer struct {
	*amp.Server
	sessions protocols.Sessions

	Servers []*MediaServer

	SessionStartedCallback func(client string)
	SessionStoppedCallback func(client string)
}

type MediaServer struct {
	Addr      *net.UDPAddr
	LocalAddr *net.UDPAddr
}

type balancerSession struct {
	base       *protocols.SessionBase
	clientAddr string
	client     *amp.Client
	balancer   *AmpBalancer
	server     *MediaServer

	startDesc *amp.StartStream
}

func NewAmpBalancer(local_addr string) (balancer *AmpBalancer, err error) {
	balancer = &AmpBalancer{
		sessions: make(protocols.Sessions),
		Servers:  make([]*MediaServer, 0, 10),
	}
	balancer.Server, err = amp.NewServer(local_addr, balancer)
	if err != nil {
		balancer = nil
	}
	return
}

func (balancer *AmpBalancer) AddMediaServer(addr string) error {
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
	balancer.Servers = append(balancer.Servers, &MediaServer{
		Addr:      serverAddr,
		LocalAddr: localAddr,
	})
	return nil
}

func (balancer *AmpBalancer) StopServer() {
	balancer.sessions.StopSessions()
}

func (balancer *AmpBalancer) StartStream(desc *amp.StartStream) error {
	client := desc.Client()
	if _, ok := balancer.sessions[client]; ok {
		return fmt.Errorf("Session already running for client %v", client)
	}
	session, err := balancer.newSession(desc)
	if err != nil {
		return err
	}
	session.base = balancer.sessions.NewSession(client, session)
	return nil
}

func (balancer *AmpBalancer) newSession(desc *amp.StartStream) (*balancerSession, error) {
	clientAddr := desc.Client()
	server := balancer.pickServer(clientAddr)
	if server == nil {
		return nil, fmt.Errorf("No server available to handle your request")
	}
	client, err := amp.NewClient(server.LocalAddr.IP.String())
	if err != nil {
		return nil, err
	}
	err = client.SetServer(server.Addr.String())
	if err != nil {
		return nil, err
	}
	client.Timeout = protocols.DefaultTimeout / 2 // Timeout before our client times out
	session := &balancerSession{
		clientAddr: clientAddr,
		balancer:   balancer,
		server:     server,
		client:     client,
	}
	err = session.startRemoteSession(desc)
	if err != nil {
		return nil, fmt.Errorf("Error delegating request: %v", err)
	}
	return session, nil
}

func (balancer *AmpBalancer) pickServer(client string) *MediaServer {
	// TODO implement load balancing
	if len(balancer.Servers) == 0 {
		return nil
	}
	return balancer.Servers[0]
}

func (balancer *AmpBalancer) StopStream(desc *amp.StopStream) error {
	return balancer.sessions.StopSession(desc.Client())
}

func (session *balancerSession) Observees() []helpers.Observee {
	return nil // No observees
}

func (session *balancerSession) Start() {
	if session.balancer.SessionStartedCallback != nil {
		session.balancer.SessionStartedCallback(session.clientAddr)
	}
}

func (session *balancerSession) Cleanup() {
	err := session.stopRemoteSession()
	if err != nil {
		session.base.CleanupErr = fmt.Errorf("Error stopping remote session: %v", err)
	}
	if session.balancer.SessionStoppedCallback != nil {
		session.balancer.SessionStoppedCallback(session.clientAddr)
	}
}

func (session *balancerSession) startRemoteSession(desc *amp.StartStream) error {
	if session.startDesc != nil {
		return fmt.Errorf("Session for %s has already been started!", session.clientAddr)
	}
	session.startDesc = desc
	return session.client.StartStream(desc.ReceiverHost, desc.Port, desc.MediaFile)
}

func (session *balancerSession) stopRemoteSession() error {
	return session.client.StopStream(session.startDesc.ReceiverHost, session.startDesc.Port)
}
