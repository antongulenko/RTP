package protocols

import (
	"fmt"
	"net"
	"time"
)

// =============================== TCP Transport ===============================

type tcpTransportProvider struct {
	net        string
	bufferSize int
}

func TcpTransport() TransportProvider {
	return &tcpTransportProvider{"tcp4", 512}
}

func (trans *tcpTransportProvider) Resolve(addr string) (Addr, error) {
	return trans.newAddr(net.ResolveTCPAddr(trans.net, addr))
}

func (trans *tcpTransportProvider) ResolveIP(ip string) (Addr, error) {
	return trans.newAddr(net.ResolveTCPAddr(trans.net, net.JoinHostPort(ip, "0")))
}

func (trans *tcpTransportProvider) ResolveLocal(remote_addr string) (Addr, error) {
	server_addr, err := net.ResolveTCPAddr(trans.net, remote_addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTCP(trans.net, nil, server_addr)
	if err != nil {
		return nil, err
	}
	local_addr, ok := conn.LocalAddr().(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("Failed to convert to *net.TCPAddr: %v", conn.LocalAddr())
	}
	_ = conn.Close() // Drop error
	return trans.newAddr(local_addr, nil)
}

func (trans *tcpTransportProvider) Listen(local Addr, protocol Protocol) (Listener, error) {
	tcp, err := toTcpAddr(local)
	if err != nil {
		return nil, err
	}
	listener, err := net.ListenTCP(trans.net, tcp.tcp)
	if err != nil {
		return nil, err
	}
	localTcp, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("Could not convert Listen addr to *net.TCPAddr", listener.Addr())
	}
	return &tcpListener{
		trans:    trans,
		tcp:      listener,
		protocol: protocol,
		local:    tcpAddr{trans, localTcp},
	}, nil
}

func (trans *tcpTransportProvider) Dial(remote Addr, protocol Protocol) (Conn, error) {
	tcp, err := toTcpAddr(remote)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTCP(trans.net, nil, tcp.tcp)
	if err != nil {
		return nil, err
	}
	local, ok := conn.LocalAddr().(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("Could not convert LocalAddr to *net.TCPAddr: %v", conn.LocalAddr())
	}
	return &tcpConn{
		trans:    trans,
		tcp:      conn,
		protocol: protocol,
		local:    tcpAddr{trans, local},
		remote:   *tcp,
	}, nil
}

func (trans *tcpTransportProvider) newAddr(tcp *net.TCPAddr, err error) (*tcpAddr, error) {
	if err == nil {
		return &tcpAddr{trans, tcp}, nil
	} else {
		return nil, err
	}
}

// =============================== TCP Addr ===============================

type tcpAddr struct {
	trans *tcpTransportProvider
	tcp   *net.TCPAddr
}

func (addr *tcpAddr) String() string {
	return addr.tcp.String()
}

func (addr *tcpAddr) Network() string {
	return addr.tcp.Network()
}

func (addr *tcpAddr) IP() net.IP {
	return addr.tcp.IP
}

func toTcpAddr(addr Addr) (*tcpAddr, error) {
	if tcp, ok := addr.(*tcpAddr); ok {
		return tcp, nil
	} else {
		return nil, fmt.Errorf("Could not convert to *tcpAddr: %v", addr)
	}
}

// ============================== TCP Listener ==============================

type tcpListener struct {
	trans    *tcpTransportProvider
	tcp      *net.TCPListener
	local    tcpAddr
	protocol Protocol
}

func (listener *tcpListener) Accept() (Conn, error) {
	tcp, err := listener.tcp.AcceptTCP()
	if err != nil {
		return nil, err
	}
	remote, ok := tcp.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("Could not convert RemoteAddr to *net.TCPAddr: %v", tcp.RemoteAddr())
	}
	return &tcpConn{
		trans:    listener.trans,
		protocol: listener.protocol,
		local:    listener.local,
		tcp:      tcp,
		remote:   tcpAddr{listener.trans, remote},
	}, nil
}

func (listener *tcpListener) LocalAddr() Addr {
	return &listener.local
}

func (listener *tcpListener) Close() error {
	return listener.tcp.Close()
}

// =============================== TCP Conn ===============================

type tcpConn struct {
	trans    *tcpTransportProvider
	tcp      *net.TCPConn
	local    tcpAddr
	remote   tcpAddr
	protocol Protocol
}

func (conn *tcpConn) LocalAddr() Addr {
	return &conn.local
}

func (conn *tcpConn) RemoteAddr() Addr {
	return &conn.remote
}

func (conn *tcpConn) Close() error {
	return conn.tcp.Close()
}

func (conn *tcpConn) Send(packet *Packet, timeout time.Duration) error {
	if timeout > 0 {
		defer conn.resetTimeout()
		if err := conn.timeout(timeout); err != nil {
			return err
		}
	}
	return conn.doSend(packet)
}

func (conn *tcpConn) UnreliableSend(packet *Packet) error {
	conn.resetTimeout()
	return conn.doSend(packet)
}

func (conn *tcpConn) doSend(packet *Packet) error {
	b, err := Marshaller.MarshalPacket(packet)
	if err != nil {
		return err
	}
	_, err = conn.tcp.Write(b)
	// TODO check if everything was written?
	return err
}

func (conn *tcpConn) Receive(timeout time.Duration) (*Packet, error) {
	if timeout > 0 {
		defer conn.resetTimeout()
		if err := conn.timeout(timeout); err != nil {
			return nil, err
		}
	}
	buf := make([]byte, conn.trans.bufferSize)
	n, err := conn.tcp.Read(buf)
	// TODO check if data did not fit in buffer?
	if err != nil {
		return nil, fmt.Errorf("Error receiving: %v", err)
	}
	buf = buf[:n]
	return Marshaller.UnmarshalPacket(buf, conn.protocol)
}

func (conn *tcpConn) timeout(timeout time.Duration) error {
	return conn.tcp.SetDeadline(time.Now().Add(timeout))
}

func (conn *tcpConn) resetTimeout() {
	var zeroTime time.Time
	_ = conn.tcp.SetDeadline(zeroTime)
}
