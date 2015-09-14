package protocols

import (
	"fmt"
	"math/rand"
	"net"
	"time"
)

const (
	DefaultTimeout = time.Second * 1
)

var (
	pingRand = rand.New(rand.NewSource(time.Now().Unix()))
)

type Client interface {
	Close() error

	SetServer(server_addr string) error
	SetTimeout(timeout time.Duration)
	Protocol() Protocol

	SendPacket(packet *Packet) error
	SendRequestPacket(packet *Packet) (reply *Packet, err error)
}

type ExtendedClient interface {
	Client

	Send(code uint, val interface{}) error
	SendRequest(code uint, val interface{}) (*Packet, error)
	CheckReply(reply *Packet) error
	Ping() error
}

type client struct {
	serverAddr *net.UDPAddr
	localAddr  *net.UDPAddr
	protocol   Protocol
	timeout    time.Duration
	conn       *net.UDPConn
}

type extendedClient struct {
	Client
}

func NewClient(local_ip string, protocol Protocol) (Client, error) {
	if protocol == nil {
		return nil, fmt.Errorf("Need non-nil Protocol")
	}
	localAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(local_ip, "0"))
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, err
	}
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("Failed to get *UDPAddr from UDPConn.LocalAddr()")
	}
	return &client{
		timeout:   DefaultTimeout,
		protocol:  protocol,
		localAddr: localAddr,
		conn:      conn,
	}, nil
}

func ExtendClient(client Client) ExtendedClient {
	return &extendedClient{client}
}

func NewExtendedClient(local_ip string, protocol Protocol) (result ExtendedClient, err error) {
	client, err := NewClient(local_ip, protocol)
	if err == nil {
		result = ExtendClient(client)
	}
	return
}

func (client *client) Close() error {
	return client.conn.Close()
}

func (client *client) Protocol() Protocol {
	return client.protocol
}

func (client *client) SetTimeout(timeout time.Duration) {
	client.timeout = timeout
}

func (client *client) SetServer(server_addr string) error {
	serverAddr, err := net.ResolveUDPAddr("udp", server_addr)
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

func (client *extendedClient) Send(code uint, val interface{}) error {
	return client.SendPacket(&Packet{
		Code: code,
		Val:  val,
	})
}

func (client *extendedClient) SendRequest(code uint, val interface{}) (*Packet, error) {
	return client.SendRequestPacket(&Packet{
		Code: code,
		Val:  val,
	})
}

func (client *extendedClient) CheckReply(reply *Packet) error {
	if reply.IsError() {
		return fmt.Errorf("%v error: %v", client.Protocol().Name(), reply.Error())
	} else if !reply.IsOK() {
		return fmt.Errorf("Unexpected %v reply (code %v): %v", client.Protocol().Name(), reply.Code, reply.Val)
	}
	return nil
}

func (client *extendedClient) Ping() error {
	ping := &PingValue{pingRand.Int()}
	reply, err := client.SendRequest(CodePing, ping)
	if err != nil {
		return err
	}
	pong, ok := reply.Val.(*PongValue)
	if !ok {
		return fmt.Errorf("Illegal Pong payload: %s", reply.Val)
	}
	if !pong.Check(ping) {
		return fmt.Errorf("Server returned wrong Pong %s (expected %s)", pong.Value, ping.Pong())
	}
	return nil
}
