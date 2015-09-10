package proxies

import (
	"fmt"
	"net"
	"strconv"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols/pcp"
)

type PcpProxy struct {
	*pcp.Server
	sessions map[int]*udpSession

	ProxyStartedCallback func(proxy *UdpProxy)
	ProxyStoppedCallback func(proxy *UdpProxy)
}

type udpSession struct {
	udp     *UdpProxy
	port    int
	stopped *helpers.OneshotCondition
}

func NewPcpProxy(pcpAddr string) (proxy *PcpProxy, err error) {
	proxy = &PcpProxy{
		sessions: make(map[int]*udpSession),
	}
	proxy.Server, err = pcp.NewServer(pcpAddr, proxy)
	if err != nil {
		proxy = nil
	}
	return
}

func (proxy *PcpProxy) StopServer() {
	for _, session := range proxy.sessions {
		proxy.cleanupSession(session)
	}
}

func getPort(addr string) (int, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse ListenAddr: %v", err)
	}
	return strconv.Atoi(port)
}

func (proxy *PcpProxy) StartProxy(desc *pcp.StartProxy) error {
	port, err := getPort(desc.ListenAddr)
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
		udp:     udp,
		port:    port,
		stopped: helpers.NewOneshotCondition(),
	}
	proxy.sessions[port] = session
	udp.Start()
	go func() {
		<-udp.Observe(proxy.Wg)
		proxy.cleanupSession(session)
	}()
	if proxy.ProxyStartedCallback != nil {
		proxy.ProxyStartedCallback(udp)
	}
	return nil
}

func (proxy *PcpProxy) StopProxy(desc *pcp.StopProxy) error {
	port, err := getPort(desc.ListenAddr)
	if err != nil {
		return err
	}
	if session, ok := proxy.sessions[port]; !ok {
		return fmt.Errorf("No UDP proxy running for port %v", port)
	} else {
		proxy.cleanupSession(session)
	}
	return nil
}

func (proxy *PcpProxy) cleanupSession(session *udpSession) {
	session.stopped.Enable(func() {
		session.udp.Stop()
		if session.udp.Err != nil {
			proxy.LogError(fmt.Errorf("UDP proxy error: %v", session.udp.Err))
		}
		if proxy.ProxyStoppedCallback != nil {
			proxy.ProxyStoppedCallback(session.udp)
		}
		delete(proxy.sessions, session.port)
	})
}
