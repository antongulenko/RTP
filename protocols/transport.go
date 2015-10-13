package protocols

import (
	"bytes"
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
	Send(packet *Packet, addr Addr, timeout time.Duration) error
	UnreliableSend(packet *Packet, addr Addr) error
	Receive(timeout time.Duration) (*Packet, error)

	LocalAddr() Addr
	Close() error
}

type Addr interface {
	net.Addr
	IP() net.IP
}

// =============================== UDP Transport ===============================

const (
	DefaultRetries = 3
)

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
		retries:  DefaultRetries,
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

const (
	AckData = "\0101*Ack*"
)

var (
	AckBytes = []byte(AckData)
)

type udpConn struct {
	trans    *udpTransportProvider
	udp      *net.UDPConn
	local    udpAddr
	protocol Protocol
	retries  int
}

func (conn *udpConn) LocalAddr() Addr {
	return &conn.local
}

func (conn *udpConn) Close() error {
	return conn.udp.Close()
}

func (conn *udpConn) Send(packet *Packet, addr Addr, timeout time.Duration) error {
	udpAddr, b, err := conn.prepareSend(packet, addr)
	if err != nil {
		return err
	}
	receiveTimeout := timeout / time.Duration(conn.retries)
	if receiveTimeout == 0 {
		receiveTimeout = time.Duration(1)
	}
	defer conn.resetTimeout()

	var ackErr error
	for i := 0; i < conn.retries; i++ {
		if err := conn.send(b, udpAddr.udp); err != nil {
			return fmt.Errorf("Error sending to %v: %v", addr, err)
		}
		if err := conn.timeout(timeout); err != nil {
			return err
		}
		ackErr = conn.receiveAck(udpAddr.udp)
		if ackErr == nil {
			return nil
		}
	}
	return fmt.Errorf("Gave up sending to %v after %v retries. Error: %v", addr, conn.retries, ackErr)
}

func (conn *udpConn) UnreliableSend(packet *Packet, addr Addr) error {
	udpAddr, b, err := conn.prepareSend(packet, addr)
	if err != nil {
		return err
	}
	return conn.send(b, udpAddr.udp)
}

func (conn *udpConn) prepareSend(packet *Packet, addr Addr) (udp *udpAddr, b []byte, err error) {
	udp, err = toUdpAddr(addr)
	if err != nil {
		return
	}
	b, err = Marshaller.MarshalPacket(packet)
	return
}

func (conn *udpConn) Receive(timeout time.Duration) (*Packet, error) {
	if timeout > 0 {
		defer conn.resetTimeout()
		if err := conn.timeout(timeout); err != nil {
			return nil, err
		}
	}
	buf, addr, err := conn.receive()
	if err != nil {
		return nil, fmt.Errorf("Error receiving: %v", err)
	}
	if bytes.Equal(AckBytes, buf) {
		return nil, fmt.Errorf("Received standalone Ack")
	}
	packet, err := Marshaller.UnmarshalPacket(buf, conn.protocol)
	if err != nil {
		return nil, err
	}
	if err := conn.sendAck(addr); err == nil {
		packet.SourceAddr = &udpAddr{conn.trans, addr}
		return packet, nil
	} else {
		return nil, err
	}
}

func (conn *udpConn) timeout(timeout time.Duration) error {
	return conn.udp.SetDeadline(time.Now().Add(timeout))
}

func (conn *udpConn) resetTimeout() {
	var zeroTime time.Time
	_ = conn.udp.SetDeadline(zeroTime)
}

func (conn *udpConn) send(b []byte, addr *net.UDPAddr) error {
	_, err := conn.udp.WriteToUDP(b, addr)
	// TODO check if everything was sent?
	return err
}

func (conn *udpConn) receive() ([]byte, *net.UDPAddr, error) {
	buf := make([]byte, conn.trans.bufferSize)
	n, addr, err := conn.udp.ReadFromUDP(buf)
	// TODO check if data did not fit in buffer?
	return buf[:n], addr, err
}

func (conn *udpConn) sendAck(addr *net.UDPAddr) error {
	//	return conn.send(AckBytes, addr)
	return nil
}

func (conn *udpConn) receiveAck(expectedAddr *net.UDPAddr) error {
	//	buf, addr, err := conn.receive()
	//	if err != nil {
	//		return fmt.Errorf("Error receiving Ack: %v", err)
	//	}
	//	if bytes.Equal(AckBytes, buf) {
	//		if addr == expectedAddr {
	//			return nil
	//		} else {
	//			return fmt.Errorf("Received Ack from %v instead of %v", addr, expectedAddr)
	//		}
	//	} else {
	//		return fmt.Errorf("Received non-Ack packet from %v", addr)
	//	}
	return nil
}
