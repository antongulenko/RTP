package protocols

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/antongulenko/RTP/helpers"
)

const (
	ErrorChanBuffer = 16
	SendTimeout     = 1 * time.Second
)

type Server struct {
	stopped  *helpers.OneshotCondition
	listener Listener
	errors   chan error

	protocol *serverProtocolInstance

	Wg      *sync.WaitGroup
	Stopped bool
}

func NewServer(addr_string string, protocol Protocol) (*Server, error) {
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
	addr, err := protocol.Transport().Resolve(addr_string)
	if err != nil {
		return nil, err
	}
	listener, err := protocol.Transport().Listen(addr, protocol)
	if err != nil {
		return nil, err
	}
	server.listener = listener
	return server, nil
}

func (server *Server) LocalAddr() Addr {
	return server.listener.LocalAddr()
}

func (server *Server) String() string {
	return fmt.Sprintf("%v on %v", server.protocol.Name(), server.LocalAddr())
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
		if err := server.listener.Close(); err != nil {
			server.LogError(fmt.Errorf("Error closing listener: %v", err))
		}
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
		conn, err := server.listener.Accept()
		if err != nil {
			if server.Stopped {
				return // error because listener was closed
			}
			server.LogError(err)
		} else {
			// TODO separate goroutine for this?
			packet, err := conn.Receive(time.Duration(0))
			if err != nil {
				server.LogError(fmt.Errorf("Error receiving on accepted connection: %v", err))
				continue
			}
			reply := server.protocol.HandleServerPacket(packet)
			if reply != nil {
				err := conn.Send(reply, SendTimeout) // TODO arbitrary timeout...
				if err != nil {
					server.LogError(fmt.Errorf("Failed to send reply: %v", err))
				}
			}
		}
	}
}

func (server *Server) Reply(code Code, value interface{}) *Packet {
	return &Packet{Code: code, Val: value}
}

func (server *Server) ReplyCheck(err error) *Packet {
	if err == nil {
		return server.ReplyOK()
	} else {
		return server.ReplyError(err)
	}
}

func (server *Server) ReplyOK() *Packet {
	return server.Reply(CodeOK, "")
}

func (server *Server) ReplyError(err error) *Packet {
	return server.Reply(CodeError, err.Error())
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
