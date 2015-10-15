package protocols

import (
	"net"
	"time"
)

var (
	DefaultTransport = TcpTransport()
)

type TransportProvider interface {
	Resolve(addr string) (Addr, error)
	ResolveIP(ip string) (Addr, error)
	ResolveLocal(remote_addr string) (Addr, error)
	Listen(local Addr, protocol Protocol) (Listener, error)
	Dial(remote Addr, protocol Protocol) (Conn, error)
	String() string
}

type Listener interface {
	Accept() (Conn, error)
	LocalAddr() Addr
	Close() error
}

type Conn interface {
	Send(packet *Packet, timeout time.Duration) error
	UnreliableSend(packet *Packet) error
	Receive(timeout time.Duration) (*Packet, error)

	RemoteAddr() Addr
	LocalAddr() Addr
	Close() error
}

type Addr interface {
	net.Addr
	IP() net.IP
}
