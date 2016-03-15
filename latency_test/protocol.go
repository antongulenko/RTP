package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"time"

	"github.com/antongulenko/RTP/protocols"
)

const (
	CodeMeasureLatency = protocols.Code(20)
)

var (
	Protocol     *latencyProtocol
	MiniProtocol = protocols.NewMiniProtocol(Protocol)
)

type MeasureLatency struct {
	Sequence uint
	SendTime time.Time
}

type latencyProtocol struct {
}

func (*latencyProtocol) Name() string {
	return "Latency"
}

func (*latencyProtocol) DefaultBufferSize() uint {
	return 256
}

func (proto *latencyProtocol) Decoders() protocols.DecoderMap {
	return protocols.DecoderMap{
		CodeMeasureLatency: proto.decodeMeasureLatency,
	}
}

func (proto *latencyProtocol) decodeMeasureLatency(dec *gob.Decoder) (interface{}, error) {
	var val MeasureLatency
	err := dec.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding MeasureLatency value: %v", err)
	}
	return &val, nil
}

type Client struct {
	protocols.Client
	Sequence uint
}

func NewClient(client protocols.Client) (*Client, error) {
	if err := client.Protocol().CheckIncludesFragment(Protocol.Name()); err != nil {
		return nil, err
	}
	return &Client{client, 0}, nil
}

func NewClientFor(server_addr string) (*Client, error) {
	client, err := protocols.NewMiniClientFor(server_addr, Protocol)
	if err != nil {
		return nil, err
	}
	return &Client{client, 0}, nil
}

func (client *Client) SendMeasureLatency() error {
	val := &MeasureLatency{
		Sequence: client.Sequence,
		SendTime: time.Now(),
	}
	client.Sequence++
	reply, err := client.SendRequest(CodeMeasureLatency, val)
	if err != nil {
		return err
	}
	return client.CheckReply(reply)
}

type Server struct {
	*protocols.Server
	Sequence uint
}

func NewServer(local_addr string) (server *Server, err error) {
	server = new(Server)
	server.Server, err = protocols.NewServer(local_addr, MiniProtocol)
	server.Server.RegisterHandlers(protocols.ServerHandlerMap{
		CodeMeasureLatency: server.HandleMeasureLatency,
	})
	if err != nil {
		server = nil
	}
	return
}

func (server *Server) HandleMeasureLatency(incoming *protocols.Packet) *protocols.Packet {
	if packet, ok := incoming.Val.(*MeasureLatency); ok {
		received := time.Now()
		latency := received.Sub(packet.SendTime)
		seq := packet.Sequence
		if seq != server.Sequence {
			log.Println("Sequence jump from", server.Sequence, "to", seq)
		}
		server.Sequence = seq + 1
		log.Printf("Seq. %v, latency: %s\n", seq, latency)
		return server.ReplyOK()
	} else {
		return server.ReplyError(fmt.Errorf("Illegal value for MeasureLatency: %v", incoming.Val))
	}

}
