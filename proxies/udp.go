package proxies

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/stats"
)

const (
	buf_size    = 4096
	buf_packets = 128
)

type UdpProxy struct {
	listenConn  *net.UDPConn
	listenAddr  *net.UDPAddr
	targetConn  *net.UDPConn
	targetAddr  *net.UDPAddr
	proxyClosed *helpers.OneshotCondition
	packets     chan []byte

	Closed bool
	Err    error
	Stats  *stats.Stats
}

func NewUdpProxy(listenAddr, targetAddr string) (*UdpProxy, error) {
	var listenUDP, targetUDP *net.UDPAddr
	var err error
	if listenUDP, err = net.ResolveUDPAddr("udp", listenAddr); err != nil {
		return nil, err
	}
	if targetUDP, err = net.ResolveUDPAddr("udp", targetAddr); err != nil {
		return nil, err
	}

	listenConn, err := net.ListenUDP("udp", listenUDP)
	if err != nil {
		return nil, err
	}
	// TODO http://play.golang.org/p/ygGFr9oLpW
	// for per-UDP-packet addressing in case on proxy handles multiple connections
	targetConn, err := net.DialUDP("udp", nil, targetUDP)
	if err != nil {
		listenConn.Close()
		return nil, err
	}

	return &UdpProxy{
		listenConn:  listenConn,
		listenAddr:  listenUDP,
		targetConn:  targetConn,
		targetAddr:  targetUDP,
		packets:     make(chan []byte, buf_packets),
		proxyClosed: helpers.NewOneshotCondition(),
		Stats:       stats.NewStats("UDP Proxy " + listenAddr),
	}, nil
}

func NewUdpProxyPair(listenHost, target1, target2 string, startPort, maxPort int) (proxy1 *UdpProxy, proxy2 *UdpProxy, err error) {
	for {
		addr1 := net.JoinHostPort(listenHost, strconv.Itoa(startPort))
		proxy1, err = NewUdpProxy(addr1, target1)
		if err == nil {
			addr2 := net.JoinHostPort(listenHost, strconv.Itoa(startPort+1))
			proxy2, err = NewUdpProxy(addr2, target2)
			if err == nil {
				break
			} else {
				proxy1.Stop()
			}
		}
		startPort += 2
		if startPort > maxPort {
			err = fmt.Errorf("Failed to allocate UDP proxy pair in port range %v-%v", startPort, maxPort)
			break
		}
	}
	return
}

func (proxy *UdpProxy) Start() {
	go proxy.readPackets()
	go proxy.forwardPackets()
}

func (proxy *UdpProxy) Observe(wg *sync.WaitGroup) <-chan interface{} {
	return proxy.proxyClosed.Observe(wg)
}

func (proxy *UdpProxy) Stop() {
	proxy.doclose(nil)
}

func (proxy *UdpProxy) String() string {
	return fmt.Sprintf("%v->%v", proxy.listenAddr, proxy.targetAddr)
}

func (proxy *UdpProxy) doclose(err error) {
	proxy.proxyClosed.Enable(func() {
		proxy.listenConn.Close()
		proxy.targetConn.Close()
		proxy.Err = err
		proxy.Closed = true
		proxy.Stats.Stop()
	})
}

func (proxy *UdpProxy) readPackets() {
	defer close(proxy.packets)
	for {
		buf := make([]byte, buf_size)
		nbytes, _ /*sourceAddr*/, err := proxy.listenConn.ReadFrom(buf)
		if err != nil {
			proxy.doclose(err)
			return
		}
		if proxy.Closed {
			return
		}
		proxy.packets <- buf[:nbytes]
	}
}

func (proxy *UdpProxy) forwardPackets() {
	for bytes := range proxy.packets {
		sentbytes, err := proxy.targetConn.Write(bytes)
		if err != nil {
			proxy.doclose(err)
			return
		}
		proxy.Stats.AddNow(uint(sentbytes))
	}
}
