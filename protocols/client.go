package protocols

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/antongulenko/RTP/helpers"
)

const (
	DefaultTimeout = time.Second * 1
)

type Client interface {
	Close() error
	Closed() bool

	SetServer(server_addr string) error
	Server() *net.UDPAddr
	SetTimeout(timeout time.Duration)
	Protocol() Protocol
	String() string

	SendPacket(packet *Packet) error
	Send(code Code, val interface{}) error
	SendRequest(code Code, val interface{}) (*Packet, error)
	SendRequestPacket(packet *Packet) (reply *Packet, err error)
	CheckReply(reply *Packet) error
	CheckError(reply *Packet, expectedCode Code) error
}

type client struct {
	serverAddr *net.UDPAddr
	localAddr  *net.UDPAddr
	conn       *net.UDPConn

	protocol    Protocol
	timeout     time.Duration
	closed      *helpers.OneshotCondition
	requestLock sync.Mutex
}

func NewClient(local_ip string, protocol Protocol) (Client, error) {
	localAddr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(local_ip, "0"))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", localAddr)
	if err != nil {
		return nil, err
	}
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		_ = conn.Close()
		return nil, fmt.Errorf("Failed to get *UDPAddr from UDPConn.LocalAddr()")
	}
	return &client{
		timeout:   DefaultTimeout,
		protocol:  protocol,
		localAddr: localAddr,
		conn:      conn,
		closed:    helpers.NewOneshotCondition(),
	}, nil
}

func NewClientFor(server string, protocol Protocol) (Client, error) {
	_, localAddr, err := helpers.ResolveUdp(server)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(localAddr.IP.String(), protocol)
	if err != nil {
		return nil, err
	}
	if err = client.SetServer(server); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func NewMiniClientFor(server_addr string, fragment ProtocolFragment) (Client, error) {
	proto := NewMiniProtocol(fragment)
	return NewClientFor(server_addr, proto)
}

func (client *client) Close() (err error) {
	client.closed.Enable(func() {
		err = client.conn.Close()
	})
	return
}

func (client *client) Closed() bool {
	return client.closed.Enabled()
}

func (client *client) String() string {
	return fmt.Sprintf("%s at %s", client.Protocol().Name(), client.Server())
}

func (client *client) Protocol() Protocol {
	return client.protocol
}

func (client *client) Server() *net.UDPAddr {
	return client.serverAddr
}

func (client *client) SetTimeout(timeout time.Duration) {
	client.timeout = timeout
}

func (client *client) SetServer(server_addr string) error {
	serverAddr, err := net.ResolveUDPAddr("udp4", server_addr)
	if err != nil {
		return err
	}
	client.serverAddr = serverAddr
	return nil
}

func (client *client) checkServer() error {
	if client.serverAddr == nil {
		return fmt.Errorf("Use SetServer to set the server address for %v client", client.protocol.Name())
	}
	return nil
}

func (client *client) SendPacket(packet *Packet) error {
	if err := client.checkServer(); err != nil {
		return err
	}
	return packet.sendPacket(client.conn, client.serverAddr, client.protocol)
}

func (client *client) SendRequestPacket(packet *Packet) (reply *Packet, err error) {
	if err = client.checkServer(); err != nil {
		return
	}
	client.requestLock.Lock()
	defer client.requestLock.Unlock()
	if err = packet.sendPacket(client.conn, client.serverAddr, client.protocol); err == nil {
		if client.timeout != 0 {
			var zeroTime time.Time
			defer client.conn.SetDeadline(zeroTime)
			if err = client.conn.SetDeadline(time.Now().Add(client.timeout)); err != nil {
				return
			}
		}
		reply, err = receivePacket(client.conn, 0, client.protocol)
		if err != nil {
			err = fmt.Errorf("Receiving %s reply from %s: %s", client.protocol.Name(), client.serverAddr, err)
		}
	} else {
		err = fmt.Errorf("Sending %s request to %s: %s", client.protocol.Name(), client.serverAddr, err)
	}
	return
}

func (client *client) Send(code Code, val interface{}) error {
	return client.SendPacket(&Packet{
		Code: code,
		Val:  val,
	})
}

func (client *client) SendRequest(code Code, val interface{}) (*Packet, error) {
	return client.SendRequestPacket(&Packet{
		Code: code,
		Val:  val,
	})
}

func (client *client) CheckError(reply *Packet, expectedCode Code) error {
	if reply.Code == CodeError {
		var errString string
		if reply.Code == CodeError {
			errString, _ = reply.Val.(string)
		}
		return fmt.Errorf("%v error: %v", client.Protocol().Name(), errString)
	}
	if reply.Code != expectedCode {
		return fmt.Errorf("Unexpected %s reply code %v. Expected %v. Payload: %v",
			client.Protocol().Name(), reply.Code, expectedCode, reply.Val)
	}
	return nil
}

func (client *client) CheckReply(reply *Packet) error {
	if err := client.CheckError(reply, CodeOK); err != nil {
		return err
	}
	return nil
}
