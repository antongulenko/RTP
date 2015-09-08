package proxies

import (
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/pcp"
)
import (
	"fmt"
	"net"
	"strconv"
)

type PcpProxy struct {
	*protocols.Server
	sessions map[int]*session
}

type session struct {
	*UdpProxy
	port int
}

func NewPcpProxy(pcpAddr string) (*PcpProxy, error) {
	proxy := &PcpProxy{
		sessions: make(map[int]*session),
	}
	var err error
	proxy.Server, err = protocols.NewServer(pcpAddr, proxy)
	if err != nil {
		return nil, err
	}
	return proxy, nil
}

func (proxy *PcpProxy) StopServer() {
	for port, _ := range proxy.sessions {
		proxy.cleanupSession(port)
	}
}

func (proxy *PcpProxy) ReceivePacket(conn *net.UDPConn) (*protocols.Packet, error) {
	packet, err := pcp.ReceivePacket(conn)
	if err != nil {
		return nil, err
	}
	return packet.Packet, err
}

func (proxy *PcpProxy) HandleRequest(request *protocols.Packet) {
	packet := &pcp.PcpPacket{request}
	switch packet.Code {
	case pcp.CodeStartProxySession:
		if desc := packet.StartProxySession(); desc == nil {
			proxy.ReplyError(packet.Packet, fmt.Errorf("Illegal value for PCP CodeStartProxySession: %v", packet.Val))
		} else {
			proxy.ReplyCheck(packet.Packet, proxy.startSession(desc))
		}
	case pcp.CodeStopProxySession:
		if desc := packet.StopProxySession(); desc == nil {
			proxy.ReplyError(packet.Packet, fmt.Errorf("Illegal value for PCP CodeStopProxySession: %v", packet.Val))
		} else {
			proxy.ReplyCheck(packet.Packet, proxy.stopSession(desc))
		}
	default:
		proxy.LogError(fmt.Errorf("Received unexpected PCP code: %v", packet.Code))
	}
}

func getPort(addr string) (int, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse ListenAddr: %v", err)
	}
	return strconv.Atoi(port)
}

func (proxy *PcpProxy) startSession(desc *pcp.StartProxySession) error {
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
	udp.Start()
	proxy.sessions[port] = &session{udp, port}
	return nil
}

func (proxy *PcpProxy) stopSession(desc *pcp.StopProxySession) error {
	port, err := getPort(desc.ListenAddr)
	if err != nil {
		return err
	}
	if _, ok := proxy.sessions[port]; !ok {
		return fmt.Errorf("No UDP proxy running for port %v", port)
	}
	proxy.cleanupSession(port)
	return nil
}

func (proxy *PcpProxy) cleanupSession(port int) {
	if udp, ok := proxy.sessions[port]; ok {
		udp.Close()
		if udp.UdpProxy.Err != nil {
			proxy.LogError(udp.UdpProxy.Err)
		}
		delete(proxy.sessions, port)
	}
}
