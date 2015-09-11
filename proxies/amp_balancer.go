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
	Addr *net.UDPAddr
}

type balancerSession struct {
	base     *protocols.SessionBase
	client   string
	balancer *AmpBalancer
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
	balancer.Servers = append(balancer.Servers, &MediaServer{
		Addr: serverAddr,
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
	client := desc.Client()
	server := balancer.pickServer(client)
	if server == nil {
		return nil, fmt.Errorf("No server available to handle your request")
	}
	session := &balancerSession{
		client:   client,
		balancer: balancer,
	}
	err := session.startRemoteSession(desc)
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
	return nil
}

func (session *balancerSession) Start() {
	if session.balancer.SessionStoppedCallback != nil {
		session.balancer.SessionStoppedCallback(session.client)
	}
}

func (session *balancerSession) Cleanup() {
	err := session.stopRemoteSession()
	if err != nil {
		session.base.CleanupErr = fmt.Errorf("Error stopping remote session: %v", err)
	}
	if session.balancer.SessionStoppedCallback != nil {
		session.balancer.SessionStoppedCallback(session.client)
	}
}

func (session *balancerSession) startRemoteSession(desc *amp.StartStream) error {
	return nil
}

func (session *balancerSession) stopRemoteSession() error {
	return nil
}
