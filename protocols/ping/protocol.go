package ping

import (
	"encoding/gob"
	"fmt"
	"math/rand"
	"time"

	"github.com/antongulenko/RTP/protocols"
)

var (
	Protocol     *pingProtocol
	MiniProtocol = protocols.NewMiniProtocol(Protocol)

	pingRand = rand.New(rand.NewSource(time.Now().Unix()))
)

// ======================= Packets =======================

const (
	codePing = protocols.Code(iota + 3)
	codePong
)

type PingPacket struct {
	Value int
}
type PongPacket struct {
	Value int
}

func (ping *PingPacket) PongValue() PongPacket {
	return PongPacket{ping.Pong()}
}

func (ping *PingPacket) Pong() int {
	return ping.Value + 1
}

func (pong *PongPacket) Check(ping *PingPacket) bool {
	return pong.Value == ping.Pong()
}

// ======================= Protocol =======================

type pingProtocol struct {
}

func (*pingProtocol) Name() string {
	return "Ping"
}

func (proto *pingProtocol) Decoders() protocols.DecoderMap {
	return protocols.DecoderMap{
		codePing: proto.decodePing,
		codePong: proto.decodePong,
	}
}

func (proto *pingProtocol) decodePing(decoder *gob.Decoder) (interface{}, error) {
	var val PingPacket
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding Ping value: %v", err)
	}
	return &val, nil
}

func (proto *pingProtocol) decodePong(decoder *gob.Decoder) (interface{}, error) {
	var val PongPacket
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding Pong value: %v", err)
	}
	return &val, nil
}

// ======================= Server =======================

func (proto *pingProtocol) ServerHandlers(server *protocols.Server) protocols.ServerHandlerMap {
	ping := &pingServer{proto, server}
	return protocols.ServerHandlerMap{
		codePing: ping.handlePing,
	}
}

type pingServer struct {
	*pingProtocol
	*protocols.Server
}

func (server *pingServer) handlePing(packet *protocols.Packet) {
	val := packet.Val
	if ping, ok := val.(*PingPacket); ok {
		server.Reply(packet, codePong, ping.PongValue())
	} else {
		err := fmt.Errorf("%s Ping received with wrong payload: (%T) %v", server.Name(), val, val)
		server.ReplyError(packet, err)
	}
}

// ======================= Client =======================

type Client struct {
	protocols.Client
}

func NewClient(client protocols.Client) (*Client, error) {
	if err := client.Protocol().CheckIncludesFragment(Protocol.Name()); err != nil {
		return nil, err
	}
	return &Client{client}, nil
}

func NewClientFor(server_addr string) (*Client, error) {
	client, err := protocols.NewMiniClientFor(server_addr, Protocol)
	if err != nil {
		return nil, err
	}
	return &Client{client}, nil
}

func (client *Client) Ping() error {
	ping := &PingPacket{Value: pingRand.Int()}
	reply, err := client.SendRequest(codePing, ping)
	if err != nil {
		return err
	}
	if err = client.CheckError(reply, codePong); err != nil {
		return err
	}
	pong, ok := reply.Val.(*PongPacket)
	if !ok {
		return fmt.Errorf("Illegal Pong payload: (%T) %s", reply.Val, reply.Val)
	}
	if !pong.Check(ping) {
		return fmt.Errorf("Server returned wrong Pong %s (expected %s)", pong.Value, ping.Pong())
	}
	return nil
}
