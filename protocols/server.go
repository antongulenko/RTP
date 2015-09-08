package protocols

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

const (
	ErrorChanBuffer = 16
)

type Server struct {
	stopOnce   sync.Once
	listenConn *net.UDPConn
	errors     chan error
	handler    ServerHandler

	Wg      *sync.WaitGroup
	Stopped bool
}

type ServerHandler interface {
	ReceivePacket(conn *net.UDPConn) (*Packet, error)
	HandleRequest(request *Packet)
	StopServer()
}

func NewServer(local_addr string, handler ServerHandler) (*Server, error) {
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
	}, nil
}

func (server *Server) Errors() <-chan error {
	return server.errors
}

func (server *Server) Start() {
	server.Wg.Add(1)
	go server.listen()
}

func (server *Server) Stop() {
	server.stopOnce.Do(func() {
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
		packet, err := server.handler.ReceivePacket(server.listenConn)
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
	err := packet.SendPacketTo(server.listenConn, request.SourceAddr)
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
	server.Reply(request, PacketOK.Code, "")
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
