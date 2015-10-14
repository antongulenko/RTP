package protocols

import (
	"fmt"
	"sync"
	"time"

	"github.com/antongulenko/RTP/helpers"
)

const (
	DefaultTimeout = 1 * time.Second
)

type Client interface {
	Close() error
	Closed() bool
	ResetConnection() // Will open new connection next time it's required

	SetServer(server_addr string) error
	Server() Addr
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
	serverAddr Addr
	conn       Conn

	protocol    Protocol
	closed      *helpers.OneshotCondition
	requestLock sync.Mutex

	// The worst case delay for one request will be up to two times this.
	timeout time.Duration
}

func NewClient(protocol Protocol) Client {
	return &client{
		protocol: protocol,
		closed:   helpers.NewOneshotCondition(),
		timeout:  DefaultTimeout,
	}
}

func NewClientFor(server string, protocol Protocol) (Client, error) {
	client := NewClient(protocol)
	if err := client.SetServer(server); err != nil {
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
		if client.conn != nil {
			err = client.conn.Close()
		}
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

func (client *client) Server() Addr {
	return client.serverAddr
}

func (client *client) SetTimeout(timeout time.Duration) {
	client.timeout = timeout
}

func (client *client) SetServer(server_addr string) error {
	addr, err := client.protocol.Transport().Resolve(server_addr)
	if err != nil {
		return err
	}
	client.ResetConnection()
	client.serverAddr = addr
	return nil
}

func (client *client) ResetConnection() {
	if client.conn != nil {
		_ = client.conn.Close() // Drop error...
		client.conn = nil
	}
}

func (client *client) checkServer() error {
	if client.serverAddr == nil {
		return fmt.Errorf("Use SetServer to configure %v client", client.protocol.Name())
	}
	if client.conn == nil {
		conn, err := client.protocol.Transport().Dial(client.serverAddr, client.protocol)
		if err != nil {
			return err
		}
		client.conn = conn
	}
	return nil
}

func (client *client) SendPacket(packet *Packet) error {
	if err := client.checkServer(); err != nil {
		return err
	}
	return client.conn.Send(packet, client.timeout)
}

func (client *client) SendRequestPacket(packet *Packet) (reply *Packet, err error) {
	if err = client.checkServer(); err != nil {
		return
	}
	client.requestLock.Lock()
	defer client.requestLock.Unlock()
	if err = client.conn.Send(packet, client.timeout); err == nil {
		reply, err = client.conn.Receive(client.timeout)
		if err != nil {
			err = fmt.Errorf("Receiving %s reply from %s: %s", client.protocol.Name(), client.conn.RemoteAddr(), err)
		}
	} else {
		err = fmt.Errorf("Sending %s request to %s: %s", client.protocol.Name(), client.conn.RemoteAddr(), err)
	}
	client.ResetConnection() // TODO hack to make tcp work...
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
