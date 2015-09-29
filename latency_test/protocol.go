package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"time"

	"github.com/antongulenko/RTP/protocols"
)

const (
	CodeMeasureLatency = protocols.CodeOther + iota
)

type MeasureLatency struct {
	Sequence uint
	SendTime time.Time
}

type LatencyProtocol struct {
}

func (*LatencyProtocol) Name() string {
	return "Latency"
}

func (*LatencyProtocol) DefaultBufferSize() uint {
	return 256
}

func (*LatencyProtocol) DecodeValue(code uint, dec *gob.Decoder) (interface{}, error) {
	switch code {
	case CodeMeasureLatency:
		var val MeasureLatency
		err := dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding MeasureLatency value: %v", err)
		}
		return &val, nil
	default:
		return nil, fmt.Errorf("Unknown Latency code: %v", code)
	}
}

type Client struct {
	protocols.ExtendedClient
	*LatencyProtocol
	Sequence uint
}

func NewClient(local_ip string) (client *Client, err error) {
	client = new(Client)
	client.ExtendedClient, err = protocols.NewExtendedClient(local_ip, client)
	if err != nil {
		client = nil
	}
	return
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
	*LatencyProtocol
	Sequence uint
}

func NewServer(local_addr string) (server *Server, err error) {
	server = new(Server)
	server.Server, err = protocols.NewServer(local_addr, server)
	if err != nil {
		server = nil
	}
	return
}

func (server *Server) StopServer() {
	// Nothing
}

func (server *Server) HandleRequest(packet *protocols.Packet) {
	val := packet.Val
	switch packet.Code {
	case CodeMeasureLatency:
		if latencyPacket, ok := val.(*MeasureLatency); ok {
			server.MeasureLatencyReceived(latencyPacket)
			server.ReplyOK(packet)
		} else {
			server.ReplyError(packet, fmt.Errorf("Illegal value for MeasureLatency: %v", packet.Val))
		}
	default:
		server.LogError(fmt.Errorf("Received unexpected code: %v", packet.Code))
	}
}

func (server *Server) MeasureLatencyReceived(packet *MeasureLatency) {
	received := time.Now()
	latency := received.Sub(packet.SendTime)
	seq := packet.Sequence
	if seq != server.Sequence {
		log.Println("Sequence jump from", server.Sequence, "to", seq)
	}
	server.Sequence = seq + 1
	log.Printf("Seq. %v, latency: %s\n", seq, latency)
}
