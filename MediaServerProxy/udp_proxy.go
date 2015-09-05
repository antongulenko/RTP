package proxy

import (
	"log"
	"net"
	"sync"
)

const (
	proto    = "udp"
	buf_size = 4096
)

type UdpProxy struct {
	listenConn  *net.UDPConn
	listenAddr  *net.UDPAddr
	targetConn  *net.UDPConn
	targetAddr  *net.UDPAddr
	closeOnce   sync.Once
	Err         error
	Closed      bool
	ListenPort  int
	ProxyClosed <-chan interface{}
	proxyClosed chan interface{}
}

func NewUdpProxy(listenAddr, targetAddr string) (*UdpProxy, error) {
	var listenUDP, targetUDP *net.UDPAddr
	var err error
	if listenUDP, err = net.ResolveUDPAddr(proto, listenAddr); err != nil {
		return nil, err
	}
	if targetUDP, err = net.ResolveUDPAddr(proto, targetAddr); err != nil {
		return nil, err
	}

	listenConn, err := net.ListenUDP(proto, listenUDP)
	if err != nil {
		return nil, err
	}
	// TODO http://play.golang.org/p/ygGFr9oLpW
	// for per-UDP-packet addressing in case on proxy handles multiple connections
	targetConn, err := net.DialUDP(proto, nil, targetUDP)
	if err != nil {
		listenConn.Close()
		return nil, err
	}

	closedChan := make(chan interface{}, 1)
	return &UdpProxy{
		listenConn:  listenConn,
		listenAddr:  listenUDP,
		targetConn:  targetConn,
		targetAddr:  targetUDP,
		proxyClosed: closedChan,
		ProxyClosed: closedChan,
	}, nil
}

func (proxy *UdpProxy) doclose(err error) {
	proxy.closeOnce.Do(func() {
		proxy.listenConn.Close()
		proxy.targetConn.Close()
		proxy.Err = err
		proxy.Closed = true
		proxy.proxyClosed <- nil
	})
}

func (proxy *UdpProxy) Close() {
	proxy.doclose(nil)
}

func (proxy *UdpProxy) readPackets() {
	for {
		buf := make([]byte, buf_size)
		nbytes, _ /*sourceAddr*/, err := proxy.listenConn.ReadFrom(buf)
		if err != nil {
			proxy.doclose(err)
			return
		}
		go func() {
			_ /*sentbytes*/, err := proxy.targetConn.Write(buf[:nbytes])
			if err != nil {
				proxy.doclose(err)
				return
			}
		}()
	}
}

func (proxy *UdpProxy) Start() {
	log.Printf("UDP proxy started from %v to %v\n", proxy.listenAddr, proxy.targetAddr)
	go proxy.readPackets()
}
