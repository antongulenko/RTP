package protocols

import (
	"errors"
	"fmt"
	"log"
	"net"
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
	handler    ServerHandler

	Wg      *sync.WaitGroup
	Stopped bool
}

type ServerHandler interface {
	Protocol
	HandleRequest(request *Packet)
	StopServer()
}

func NewServer(local_addr string, handler ServerHandler) (*Server, error) {
	if handler == nil {
		return nil, fmt.Errorf("Need non-nil ServerHandler")
	}
	udpAddr, err := net.ResolveUDPAddr("udp", local_addr)
	if err != nil {
		return nil, err
	}
	listenConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}
	return &Server{
		Wg:         new(sync.WaitGroup),
		handler:    handler,
		listenConn: listenConn,
		errors:     make(chan error, ErrorChanBuffer),
		stopped:    helpers.NewOneshotCondition(),
	}, nil
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
		server.handler.StopServer()
		server.Wg.Wait()
	})
}

func (server *Server) listen() {
	defer server.Wg.Done()
	for {
		if server.Stopped {
			return
		}
		packet, err := receivePacket(server.listenConn, 0, server.handler)
		if err != nil {
			if server.Stopped {
				return // error because of read from closed connection
			}
			server.LogError(err)
		} else {
			server.handle(packet)
		}
	}
}

func (server *Server) handle(request *Packet) {
	switch request.Code {
	case CodeOK:
		server.LogError(errors.New("Received standalone OK message"))
	case CodeError:
		server.LogError(fmt.Errorf("Received Error: %s", request.Error()))
	default:
		server.handler.HandleRequest(request)
	}
}

func (server *Server) Reply(request *Packet, code uint, value interface{}) {
	packet := Packet{Code: code, Val: value}
	err := packet.sendPacket(server.listenConn, request.SourceAddr, server.handler)
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
