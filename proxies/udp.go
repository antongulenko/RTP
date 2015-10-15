package proxies

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/stats"
)

const (
	buf_read_size    = 4096
	buf_write_errors = 5
)

var (
	BufferedPackets  uint = 128
	ProxyPairMinPort int  = 20000
	ProxyPairMaxPort int  = 50000
)

func UdpProxyFlags() {
	flag.IntVar(&ProxyPairMinPort, "minport", ProxyPairMinPort, "Lowest port for allocating proxy pairs")
	flag.IntVar(&ProxyPairMaxPort, "maxport", ProxyPairMaxPort, "Highest port for allocating proxy pairs")
	flag.UintVar(&BufferedPackets, "udp_buffer", BufferedPackets, "Size of buffer for storing received packets before forwarding")
}

type UdpProxyErrorBehavior int

const (
	OnErrorClose = UdpProxyErrorBehavior(iota)
	OnErrorContinue
	OnErrorRetry
	OnErrorPause // PauseWrite immediately on error. After ResumeWrite retry last packet.
)

type UdpProxy struct {
	listenConn *net.UDPConn
	listenAddr *net.UDPAddr
	targetConn *net.UDPConn
	targetAddr *net.UDPAddr

	proxyClosed    *helpers.OneshotCondition
	packets        chan []byte
	targetConnLock sync.Mutex

	writePaused     bool
	writePausedCond sync.Cond
	writeErrors     chan error

	OnError UdpProxyErrorBehavior

	CloseOnError bool
	Closed       bool
	Err          error
	Stats        *stats.Stats
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
	// for per-UDP-packet addressing in case one proxy handles multiple connections
	targetConn, err := net.DialUDP("udp", nil, targetUDP)
	if err != nil {
		listenConn.Close()
		return nil, err
	}

	return &UdpProxy{
		listenConn:      listenConn,
		listenAddr:      listenUDP,
		targetConn:      targetConn,
		targetAddr:      targetUDP,
		packets:         make(chan []byte, BufferedPackets),
		proxyClosed:     helpers.NewOneshotCondition(),
		writeErrors:     make(chan error, buf_write_errors),
		Stats:           stats.NewStats("UDP Proxy " + listenAddr),
		OnError:         OnErrorClose,
		writePausedCond: sync.Cond{L: new(sync.Mutex)},
	}, nil
}

func NewUdpProxyPair(listenHost, target1, target2 string) (proxy1 *UdpProxy, proxy2 *UdpProxy, err error) {
	startPort := ProxyPairMinPort
	maxPort := ProxyPairMaxPort
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

func (proxy *UdpProxy) WriteErrors() <-chan error {
	return proxy.writeErrors
}

func (proxy *UdpProxy) RedirectOutput(newTargetAddr string) error {
	var targetUDP *net.UDPAddr
	var err error
	if targetUDP, err = net.ResolveUDPAddr("udp", newTargetAddr); err != nil {
		return err
	}
	targetConn, err := net.DialUDP("udp", nil, targetUDP)
	if err != nil {
		return err
	}

	proxy.targetConnLock.Lock() // Don't close while write is in progress
	defer proxy.targetConnLock.Unlock()
	_ = proxy.targetConn.Close() // TODO Error is dropped
	proxy.targetAddr = targetUDP
	proxy.targetConn = targetConn
	return nil
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
		buf := make([]byte, buf_read_size)
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

		// State for OnErrorRetry
		var firstWriteError *time.Time
		var lastError error
		var writeErrors int

		for {
			proxy.waitWhilePaused()
			proxy.targetConnLock.Lock()
			sentbytes, err := proxy.targetConn.Write(bytes)
			proxy.targetConnLock.Unlock()
			if err != nil {
				switch proxy.OnError {
				case OnErrorContinue:
					proxy.writeError(err)
					break // Fetch next packet
				case OnErrorPause:
					proxy.writeError(fmt.Errorf("Pausing %v because of: %v", proxy, err))
					proxy.PauseWrite() // Will retry packet after ResumeWrite
				case OnErrorRetry:
					if firstWriteError == nil {
						now := time.Now()
						firstWriteError = &now
					}
					lastError = err
					writeErrors++
				case OnErrorClose:
					fallthrough
				default:
					proxy.writeError(err)
					proxy.doclose(err)
					return
				}
			} else {
				if proxy.OnError == OnErrorRetry && firstWriteError != nil {
					// Write is working again
					delay := time.Now().Sub(*firstWriteError).String()
					proxy.writeError(fmt.Errorf("Continuing after %v write errors within %s. Last error: %v", writeErrors, delay, lastError))
				}
				proxy.Stats.AddNow(uint(sentbytes))
				break
			}
		}
	}
}

func (proxy *UdpProxy) writeError(err error) {
	select {
	case proxy.writeErrors <- err:
	default:
		log.Printf("Warning: dropping UDP proxy %v write error: %v\n", proxy, err)
	}
}

func (proxy *UdpProxy) PauseWrite() {
	proxy.writePausedCond.L.Lock()
	defer proxy.writePausedCond.L.Unlock()
	proxy.writePaused = true
}

func (proxy *UdpProxy) ResumeWrite() {
	proxy.writePausedCond.L.Lock()
	defer proxy.writePausedCond.L.Unlock()
	proxy.writePaused = false
	proxy.writePausedCond.Broadcast()
}

func (proxy *UdpProxy) waitWhilePaused() {
	proxy.writePausedCond.L.Lock()
	defer proxy.writePausedCond.L.Unlock()
	wasPaused := proxy.writePaused
	for proxy.writePaused {
		proxy.writePausedCond.Wait()
	}
	if wasPaused {
		proxy.writeError(fmt.Errorf("Resuming %v", proxy))
	}
}
