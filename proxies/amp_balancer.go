package proxies

import (
	"fmt"
	"net"

	"github.com/antongulenko/RTP/protocols/amp"
)

type AmpBalancer struct {
	*amp.Server
	sessions map[string]*balancerSession

	Servers []*MediaServer
}

type MediaServer struct {
	Addr *net.UDPAddr
}

type balancerSession struct {
	client string
}

func NewAmpBalancer(local_addr string) (balancer *AmpBalancer, err error) {
	balancer = &AmpBalancer{
		sessions: make(map[string]*balancerSession),
		Servers:  make([]*MediaServer, 10),
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
	for _, session := range balancer.sessions {
		if err := balancer.cleanupSession(session); err != nil {
			balancer.LogError(err)
		}
	}
}

func (balancer *AmpBalancer) StartStream(desc *amp.StartStream) error {
	client := desc.Client()
	if _, ok := balancer.sessions[client]; ok {
		return fmt.Errorf("Session already running for client %v", client)
	}
	server := balancer.pickServer()
	if server == nil {
		return fmt.Errorf("No server available to handle your request")
	}
	session := &balancerSession{
		client: client,
	}
	err := session.startRemoteSession(desc)
	if err != nil {
		return fmt.Errorf("Error delegating request: %v", err)
	}
	balancer.sessions[client] = session
	return nil
}

func (balancer *AmpBalancer) pickServer() *MediaServer {
	// TODO implement load balancing
	if len(balancer.Servers) == 0 {
		return nil
	}
	return balancer.Servers[0]
}

func (balancer *AmpBalancer) StopStream(desc *amp.StopStream) error {
	client := desc.Client()
	session, ok := balancer.sessions[client]
	if !ok {
		return fmt.Errorf("No session running for client %v", client)
	}
	return balancer.cleanupSession(session)
}

func (balancer *AmpBalancer) cleanupSession(session *balancerSession) error {
	err := session.stopRemoteSession()
	delete(balancer.sessions, session.client)
	if err != nil {
		return fmt.Errorf("Error stopping remote session: %v", err)
	}
	return nil
}

func (session *balancerSession) startRemoteSession(desc *amp.StartStream) error {
	return nil
}

func (session *balancerSession) stopRemoteSession() error {
	return nil
}
