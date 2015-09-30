package proxies

import (
	"fmt"
	"net"
	"strconv"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/pcp"
)

type PcpProxy struct {
	*pcp.Server
	sessions protocols.Sessions

	ProxyStartedCallback func(proxy *UdpProxy)
	ProxyStoppedCallback func(proxy *UdpProxy)
}

type udpSession struct {
	*protocols.SessionBase

	udp   *UdpProxy
	udp2  *UdpProxy
	port  int
	proxy *PcpProxy
}

func NewPcpProxy(pcpAddr string) (proxy *PcpProxy, err error) {
	proxy = &PcpProxy{
		sessions: make(protocols.Sessions),
	}
	proxy.Server, err = pcp.NewServer(pcpAddr, proxy)
	if err != nil {
		proxy = nil
	}
	return
}

func (proxy *PcpProxy) StopServer() {
	if err := proxy.sessions.DeleteSessions(); err != nil {
		proxy.LogError(fmt.Errorf("Error stopping sessions: %v", err))
	}
}

func (proxy *PcpProxy) StartProxy(desc *pcp.StartProxy) error {
	port, err := desc.ListenPort()
	if err != nil {
		return err
	}
	if _, ok := proxy.sessions[port]; ok {
		return fmt.Errorf("UDP proxy already running for port %v", port)
	}

	udp, err := NewUdpProxy(desc.ListenAddr, desc.TargetAddr)
	if err != nil {
		return err
	}
	session := &udpSession{
		udp:   udp,
		port:  port,
		proxy: proxy,
	}
	proxy.sessions.StartSession(port, session)
	return nil
}

func (proxy *PcpProxy) StopProxy(desc *pcp.StopProxy) error {
	port, err := desc.ListenPort()
	if err != nil {
		return err
	}
	return proxy.sessions.DeleteSession(port)
}

func (proxy *PcpProxy) StartProxyPair(val *pcp.StartProxyPair) (*pcp.StartProxyPairResponse, error) {
	target1 := net.JoinHostPort(val.ReceiverHost, strconv.Itoa(val.ReceiverPort1))
	target2 := net.JoinHostPort(val.ReceiverHost, strconv.Itoa(val.ReceiverPort2))
	udp1, udp2, err := NewUdpProxyPair(val.ProxyHost, target1, target2)
	if err != nil {
		return nil, err
	}

	port1, port2 := udp1.listenAddr.Port, udp2.listenAddr.Port
	port := port1
	if _, ok := proxy.sessions[port]; ok {
		// This should not happen due to the NewUdpProxyPair algorithm
		return nil, fmt.Errorf("Session already exists for one of the proxies on port %v or %v", port1, port2)
	}

	session := &udpSession{
		udp:   udp1,
		udp2:  udp2,
		port:  port,
		proxy: proxy,
	}
	proxy.sessions.StartSession(port, session)
	return &pcp.StartProxyPairResponse{
		ProxyHost:  val.ProxyHost,
		ProxyPort1: port1,
		ProxyPort2: port2,
	}, nil
}

func (proxy *PcpProxy) StopProxyPair(val *pcp.StopProxyPair) error {
	return proxy.sessions.DeleteSession(val.ProxyPort1)
}

func (session *udpSession) Observees() []helpers.Observee {
	result := []helpers.Observee{session.udp}
	if session.udp2 != nil {
		result = append(result, session.udp2)
	}
	return result
}

func (session *udpSession) Start(base *protocols.SessionBase) {
	session.SessionBase = base
	session.udp.Start()
	if session.udp2 != nil {
		session.udp2.Start()
	}
	if session.proxy.ProxyStartedCallback != nil {
		session.proxy.ProxyStartedCallback(session.udp)
		if session.udp2 != nil {
			session.proxy.ProxyStartedCallback(session.udp2)
		}
	}
}

func (session *udpSession) Cleanup() {
	var errors helpers.MultiError
	if session.udp.Err != nil {
		errors = append(errors, fmt.Errorf("UDP proxy %v error: %v", session.udp, session.udp.Err))
	}
	if session.udp2 != nil && session.udp2.Err != nil {
		errors = append(errors, fmt.Errorf("UDP proxy %v error: %v", session.udp2, session.udp2.Err))
	}
	session.CleanupErr = errors.NilOrError()
	if session.proxy.ProxyStoppedCallback != nil {
		session.proxy.ProxyStoppedCallback(session.udp)
		if session.udp2 != nil {
			session.proxy.ProxyStoppedCallback(session.udp2)
		}
	}
}
