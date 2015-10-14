package protocols

import (
	"fmt"
	"net"
	"time"
)

// =============================== UDP Transport ===============================

const (
	DefaultRetries = 3
)

type udpTransportProvider struct {
	net        string
	bufferSize int
}

func UdpTransport() TransportProvider {
	return &udpTransportProvider{"udp4", 512}
}

func (trans *udpTransportProvider) Resolve(addr string) (Addr, error) {
	return trans.newAddr(net.ResolveUDPAddr(trans.net, addr))
}

func (trans *udpTransportProvider) ResolveIP(ip string) (Addr, error) {
	return trans.newAddr(net.ResolveUDPAddr(trans.net, net.JoinHostPort(ip, "0")))
}

func (trans *udpTransportProvider) doResolveLocal(server_addr *net.UDPAddr) (*net.UDPAddr, error) {
	// TODO find simpler way to get the local address for a remote server
	conn, err := net.DialUDP(trans.net, nil, server_addr)
	if err != nil {
		return nil, err
	}
	local_addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("Failed to convert to *net.UDPAddr: %v", conn.LocalAddr())
	}
	_ = conn.Close() // Drop error
	return local_addr, nil
}

func (trans *udpTransportProvider) ResolveLocal(remote_addr string) (Addr, error) {
	server_addr, err := net.ResolveUDPAddr(trans.net, remote_addr)
	if err != nil {
		return nil, err
	}
	return trans.newAddr(trans.doResolveLocal(server_addr))
}

func (trans *udpTransportProvider) Listen(local Addr, protocol Protocol) (Listener, error) {
	udp, err := toUdpAddr(local)
	if err != nil {
		return nil, err
	}
	udpConn, err := net.ListenUDP(trans.net, udp.udp)
	conn, err := trans.newConn(udpConn, nil, protocol, err)
	if err != nil {
		return nil, err
	}
	return &udpListener{conn}, nil
}

func (trans *udpTransportProvider) Dial(remote Addr, protocol Protocol) (Conn, error) {
	udp, err := toUdpAddr(remote)
	if err != nil {
		return nil, err
	}
	local, err := trans.doResolveLocal(udp.udp)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP(trans.net, local)
	return trans.newConn(conn, udp, protocol, err)
}

func (trans *udpTransportProvider) newAddr(udp *net.UDPAddr, err error) (*udpAddr, error) {
	if err == nil {
		return &udpAddr{trans, udp}, nil
	} else {
		return nil, err
	}
}

func (trans *udpTransportProvider) newConn(udp *net.UDPConn, remote_addr *udpAddr, protocol Protocol, err error) (*udpConn, error) {
	if err != nil {
		return nil, err
	}
	local, ok := udp.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("Could not convert LocalAddr to *net.UDPAddr: %v", udp.LocalAddr())
	}
	return &udpConn{
		trans:    trans,
		udp:      udp,
		protocol: protocol,
		local:    udpAddr{trans, local},
		remote:   remote_addr,
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

// ============================== UDP Listener ==============================

type udpListener struct {
	conn *udpConn
}

func (listener *udpListener) Accept() (Conn, error) {
	packet, err := listener.conn.Receive(time.Duration(0))
	if err != nil {
		return nil, err
	}
	return &udpAcceptedConn{
		listener: listener,
		packet:   packet,
	}, nil
}

func (listener *udpListener) Close() error {
	return listener.conn.Close()
}

func (listener *udpListener) LocalAddr() Addr {
	return listener.conn.LocalAddr()
}

// ============================ UDP accepted Conn ============================

type udpAcceptedConn struct {
	listener *udpListener
	packet   *Packet
	received bool
	closed   bool
}

func (conn *udpAcceptedConn) Send(packet *Packet, timeout time.Duration) error {
	if err := conn.checkClosed(); err != nil {
		return err
	}
	return conn.listener.conn.doSend(packet, conn.packet.SourceAddr, timeout)
}

func (conn *udpAcceptedConn) UnreliableSend(packet *Packet) error {
	if err := conn.checkClosed(); err != nil {
		return err
	}
	return conn.listener.conn.doUnreliableSend(packet, conn.packet.SourceAddr)
}

func (conn *udpAcceptedConn) Receive(timeout time.Duration) (*Packet, error) {
	if err := conn.checkClosed(); err != nil {
		return nil, err
	}
	if conn.received {
		return nil, fmt.Errorf("Can only receive once from a udpAcceptedConn")
	}
	return conn.packet, nil
}

func (conn *udpAcceptedConn) RemoteAddr() Addr {
	return conn.packet.SourceAddr
}

func (conn *udpAcceptedConn) LocalAddr() Addr {
	return conn.listener.conn.LocalAddr()
}

func (conn *udpAcceptedConn) Close() error {
	if err := conn.checkClosed(); err != nil {
		return err
	}
	conn.closed = true
	return nil
}

func (conn *udpAcceptedConn) checkClosed() error {
	if conn.closed {
		return fmt.Errorf("Already closed")
	}
	return nil
}

// =============================== UDP Conn ===============================

type udpConn struct {
	trans    *udpTransportProvider
	udp      *net.UDPConn
	local    udpAddr
	remote   *udpAddr
	protocol Protocol
	retries  int
}

func (conn *udpConn) LocalAddr() Addr {
	return &conn.local
}

func (conn *udpConn) RemoteAddr() Addr {
	if conn.remote == nil {
		panic("oups")
	}
	return conn.remote
}

func (conn *udpConn) Close() error {
	return conn.udp.Close()
}

func (conn *udpConn) Send(packet *Packet, timeout time.Duration) error {
	if conn.remote == nil {
		return fmt.Errorf("Cannot send on this connection")
	}
	return conn.doSend(packet, conn.remote, timeout)
}

func (conn *udpConn) doSend(packet *Packet, addr Addr, timeout time.Duration) error {
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

func (conn *udpConn) UnreliableSend(packet *Packet) error {
	if conn.remote == nil {
		return fmt.Errorf("Cannot send on this connection")
	}
	return conn.doUnreliableSend(packet, conn.remote)
}

func (conn *udpConn) doUnreliableSend(packet *Packet, addr Addr) error {
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
	// TODO check if Ack was received...
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
	// TODO write Ack mechansim
	return nil
}

func (conn *udpConn) receiveAck(expectedAddr *net.UDPAddr) error {
	// TODO write Ack mechansim
	return nil
}
