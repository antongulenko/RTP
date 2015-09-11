package proxies

import (
	"fmt"

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
	proxy.sessions.StopSessions()
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
	session.SessionBase = proxy.sessions.NewSession(port, session)
	return nil
}

func (proxy *PcpProxy) StopProxy(desc *pcp.StopProxy) error {
	port, err := desc.ListenPort()
	if err != nil {
		return err
	}
	return proxy.sessions.StopSession(port)
}

func (session *udpSession) Observees() []helpers.Observee {
	return []helpers.Observee{
		session.udp,
	}
}

func (session *udpSession) Start() {
	session.udp.Start()
	if session.proxy.ProxyStartedCallback != nil {
		session.proxy.ProxyStartedCallback(session.udp)
	}
}

func (session *udpSession) Cleanup() {
	if session.udp.Err != nil {
		session.CleanupErr = session.udp.Err
		session.proxy.LogError(fmt.Errorf("UDP proxy error: %v", session.udp.Err))
	}
	if session.proxy.ProxyStoppedCallback != nil {
		session.proxy.ProxyStoppedCallback(session.udp)
	}
}
