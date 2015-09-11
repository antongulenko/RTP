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

type Client struct {
	ServerAddr *net.UDPAddr
	LocalAddr  *net.UDPAddr
	Protocol   Protocol
	Timeout    time.Duration

	conn *net.UDPConn
}

func NewClient(local_ip string, protocol Protocol) (*Client, error) {
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
	return &Client{
		Timeout:   DefaultTimeout,
		Protocol:  protocol,
		LocalAddr: localAddr,
		conn:      conn,
	}, nil
}

func (client *Client) Close() error {
	return client.conn.Close()
}

func (client *Client) SetServer(server_addr string) error {
	serverAddr, err := net.ResolveUDPAddr("udp", server_addr)
	if err != nil {
		return err
	}
	client.ServerAddr = serverAddr
	return nil
}

func (client *Client) checkServer() error {
	if client.ServerAddr == nil {
		return fmt.Errorf("Use SetServer to set the server address for %v client", client.Protocol.Name())
	}
	return nil
}

func (client *Client) SendPacket(packet *Packet) error {
	if err := client.checkServer(); err != nil {
		return err
	}
	return packet.sendPacket(client.conn, client.ServerAddr, client.Protocol)
}

func (client *Client) Send(code uint, val interface{}) error {
	return client.SendPacket(&Packet{
		Code: code,
		Val:  val,
	})
}

func (client *Client) SendRequestPacket(packet *Packet) (reply *Packet, err error) {
	if err = client.checkServer(); err != nil {
		return
	}
	if err = packet.sendPacket(client.conn, client.ServerAddr, client.Protocol); err == nil {
		if client.Timeout != 0 {
			var zeroTime time.Time
			defer client.conn.SetDeadline(zeroTime)
			if err = client.conn.SetDeadline(time.Now().Add(client.Timeout)); err != nil {
				return
			}
		}
		reply, err = receivePacket(client.conn, 0, client.Protocol)
		if err != nil {
			err = fmt.Errorf("Receiving %s reply from %s: %s", client.Protocol.Name(), client.ServerAddr, err)
		}
	} else {
		err = fmt.Errorf("Sending %s request to %s: %s", client.Protocol.Name(), client.ServerAddr, err)
	}
	return
}

func (client *Client) SendRequest(code uint, val interface{}) (*Packet, error) {
	return client.SendRequestPacket(&Packet{
		Code: code,
		Val:  val,
	})
}

func (client *Client) CheckReply(reply *Packet) error {
	if reply.IsError() {
		return fmt.Errorf("%v error: %v", client.Protocol.Name(), reply.Error())
	} else if !reply.IsOK() {
		return fmt.Errorf("Unexpected %v reply (code %v): %v", client.Protocol.Name(), reply.Code, reply.Val)
	}
	return nil
}

func (client *Client) Ping() error {
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
