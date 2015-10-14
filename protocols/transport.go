package protocols

import (
	"fmt"
	"net"
	"time"
)

var (
	Transport TransportProvider = udpTransport
)

type TransportProvider interface {
	Resolve(addr string) (Addr, error)
	ResolveIP(ip string) (Addr, error)
	ResolveLocal(remote_addr string) (Addr, error)
	Listen(local Addr, protocol Protocol) (Conn, error)
	Dial(remote Addr, protocol Protocol) (Conn, error)
}

type Conn interface {
	Send(packet *Packet, addr Addr) error
	UnreliableSend(packet *Packet, addr Addr) error
	Receive() (*Packet, error)

	LocalAddr() Addr
	Close() error
	SetDeadline(t time.Time) error
}

type Addr interface {
	net.Addr
	IP() net.IP
}

// =============================== UDP Transport ===============================

var (
	udpTransport = &udpTransportProvider{"udp4", 512}
)

type udpTransportProvider struct {
	net        string
	bufferSize int
}

func (trans *udpTransportProvider) Resolve(addr string) (Addr, error) {
	return trans.newAddr(net.ResolveUDPAddr(trans.net, addr))
}

func (trans *udpTransportProvider) ResolveIP(ip string) (Addr, error) {
	return trans.newAddr(net.ResolveUDPAddr(trans.net, net.JoinHostPort(ip, "0")))
}

func (trans *udpTransportProvider) ResolveLocal(remote_addr string) (Addr, error) {
	server_addr, err := net.ResolveUDPAddr(trans.net, remote_addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP(trans.net, nil, server_addr)
	if err != nil {
		return nil, err
	}
	local_addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("Failed to convert to *net.UDPAddr: %v", conn.LocalAddr())
	}
	_ = conn.Close() // Drop error
	return trans.newAddr(local_addr, nil)
}

func (trans *udpTransportProvider) Listen(local Addr, protocol Protocol) (Conn, error) {
	udp, err := toUdpAddr(local)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP(trans.net, udp.udp)
	return trans.newConn(conn, protocol, err)
}

func (trans *udpTransportProvider) Dial(remote Addr, protocol Protocol) (Conn, error) {
	udp, err := toUdpAddr(remote)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP(trans.net, nil, udp.udp)
	return trans.newConn(conn, protocol, err)
}

func (trans *udpTransportProvider) newAddr(udp *net.UDPAddr, err error) (*udpAddr, error) {
	if err == nil {
		return &udpAddr{trans, udp}, nil
	} else {
		return nil, err
	}
}

func (trans *udpTransportProvider) newConn(udp *net.UDPConn, protocol Protocol, err error) (*udpConn, error) {
	if err != nil {
		return nil, err
	}
	local, ok := udp.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("Could not convert to *net.UDPAddr: %v", udp.LocalAddr())
	}
	return &udpConn{
		trans:    trans,
		udp:      udp,
		protocol: protocol,
		local:    udpAddr{trans, local},
	}, nil
}

// =============================== UDP Addr ===============================

type udpAddr struct {
	trans *udpTransportProvider
	udp   *net.UDPAddr
}

func (addr *udpAddr) String() string {
	return addr.udp.String()
}

func (addr *udpAddr) Network() string {
	return addr.udp.Network()
}

func (addr *udpAddr) IP() net.IP {
	return addr.udp.IP
}

func toUdpAddr(addr Addr) (*udpAddr, error) {
	if udp, ok := addr.(*udpAddr); ok {
		return udp, nil
	} else {
		return nil, fmt.Errorf("Could not convert to *udpAddr: %v", addr)
	}
}

// =============================== UDP Conn ===============================

type udpConn struct {
	trans    *udpTransportProvider
	udp      *net.UDPConn
	local    udpAddr
	protocol Protocol
}

func (conn *udpConn) Send(packet *Packet, addr Addr) error {
	// TODO implement retry etc.
	return conn.UnreliableSend(packet, addr)
}

func (conn *udpConn) UnreliableSend(packet *Packet, addr Addr) error {
	udpAddr, err := toUdpAddr(addr)
	if err != nil {
		return err
	}
	b, err := Marshaller.MarshalPacket(packet)
	if err != nil {
		return err
	}
	_, err = conn.udp.WriteToUDP(b, udpAddr.udp)
	return err
}

func (conn *udpConn) Receive() (*Packet, error) {
	buf := make([]byte, conn.trans.bufferSize)
	n, addr, err := conn.udp.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	// TODO check if not enough data received?
	packet, err := Marshaller.UnmarshalPacket(buf[:n], conn.protocol)
	if err != nil {
		return nil, err
	}
	packet.SourceAddr = &udpAddr{conn.trans, addr}
	return packet, nil
}

func (conn *udpConn) LocalAddr() Addr {
	return &conn.local
}

func (conn *udpConn) Close() error {
	return conn.udp.Close()
}

func (conn *udpConn) SetDeadline(t time.Time) error {
	return conn.udp.SetDeadline(t)
}
