package protocols

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/antongulenko/RTP/helpers"
)

const (
	ErrorChanBuffer = 16
)

type Server struct {
	stopped    *helpers.OneshotCondition
	listenConn *net.UDPConn
	errors     chan error

	protocol *serverProtocolInstance

	Wg        *sync.WaitGroup
	Stopped   bool
	LocalAddr *net.UDPAddr
}

func NewServer(local_addr string, protocol Protocol) (*Server, error) {
	server := &Server{
		Wg:      new(sync.WaitGroup),
		errors:  make(chan error, ErrorChanBuffer),
		stopped: helpers.NewOneshotCondition(),
	}
	var err error
	server.protocol, err = protocol.instantiateServer(server)
	if err != nil {
		return nil, err
	}
	udpAddr, err := net.ResolveUDPAddr("udp4", local_addr)
	if err != nil {
		return nil, err
	}
	listenConn, err := net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return nil, err
	}
	server.listenConn = listenConn
	localUdpAddr, ok := listenConn.LocalAddr().(*net.UDPAddr)
	server.LocalAddr = localUdpAddr
	if !ok {
		_ = listenConn.Close()
		return nil, fmt.Errorf("Failed to convert to *net.UdpAddr: %v", listenConn.LocalAddr())
	}
	return server, nil
}

func (server *Server) String() string {
	return fmt.Sprintf("%v on %v", server.protocol.Name(), server.LocalAddr)
}

func (server *Server) Protocol() Protocol {
	return server.protocol
}

func (server *Server) Errors() <-chan error {
	return server.errors
}

func (server *Server) Start() {
	server.Wg.Add(1)
	go server.listen()
}

func (server *Server) Observe(wg *sync.WaitGroup) <-chan interface{} {
	return server.stopped.Observe(wg)
}

func (server *Server) Stop() {
	server.stopped.Enable(func() {
		server.Stopped = true
		server.listenConn.Close()
		server.protocol.stopServer()
		server.Wg.Wait()
	})
}

func (server *Server) RegisterHandlers(handlers ServerHandlerMap) error {
	return server.protocol.registerHandlers(handlers)
}

func (server *Server) RegisterStopHandler(handler ServerStopper) {
	server.protocol.registerStopper(handler)
}

func (server *Server) listen() {
	defer server.Wg.Done()
	for {
		if server.Stopped {
			return
		}
		packet, err := receivePacket(server.listenConn, 0, server.protocol)
		if err != nil {
			if server.Stopped {
				return // error because of read from closed connection
			}
			server.LogError(err)
		} else {
			server.protocol.HandleServerPacket(packet)
		}
	}
}

func (server *Server) SendPacket(packet *Packet, target *net.UDPAddr) error {
	return packet.sendPacket(server.listenConn, target, server.protocol)
}

func (server *Server) Reply(request *Packet, code Code, value interface{}) {
	packet := Packet{Code: code, Val: value}
	err := server.SendPacket(&packet, request.SourceAddr)
	if err != nil {
		server.LogError(fmt.Errorf("Failed to send reply: %v", err))
	}
}

func (server *Server) ReplyCheck(request *Packet, err error) {
	if err == nil {
		server.ReplyOK(request)
	} else {
		server.ReplyError(request, err)
	}
}

func (server *Server) ReplyOK(request *Packet) {
	server.Reply(request, CodeOK, "")
}

func (server *Server) ReplyError(request *Packet, err error) {
	server.Reply(request, CodeError, err.Error())
}

func (server *Server) LogError(err error) {
	select {
	case server.errors <- err:
	default:
		log.Printf("Warning: dropped server error: %v\n", err)
	}
}

func ParseServerFlags(default_ip string, default_port int) string {
	port := flag.Int("port", default_port, "The port to start the server")
	ip := flag.String("host", default_ip, "The ip to listen for traffic")
	flag.Parse()
	return net.JoinHostPort(*ip, strconv.Itoa(int(*port)))
}
