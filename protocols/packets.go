package protocols

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net"
)

const (
	CodeOK = 10 + iota
	CodeError
	CodePing
	CodePong
	CodeOther
)

type PingValue struct {
	Value int
}

type PongValue struct {
	Value int
}

func (ping *PingValue) Pong() int {
	return ping.Value + 1
}

func (pong *PongValue) Check(ping *PingValue) bool {
	return pong.Value == ping.Pong()
}

type Protocol interface {
	DecodeValue(code uint, decoder *gob.Decoder) (interface{}, error)
	Name() string

	DefaultBufferSize() uint
}

type Packet struct {
	Code       uint
	Val        interface{}
	SourceAddr *net.UDPAddr
}

func (packet *Packet) IsOK() bool {
	return packet.Code == CodeOK
}

func (packet *Packet) IsError() bool {
	return packet.Code == CodeError
}

func (packet *Packet) Error() (res string) {
	if packet.Code == CodeError {
		res, _ = packet.Val.(string)
	}
	return
}

func decodePacket(reader io.Reader, protocol Protocol) (*Packet, error) {
	dec := gob.NewDecoder(reader)
	var packet Packet
	err := dec.Decode(&packet.Code)
	if err != nil {
		return nil, fmt.Errorf("Error decoding %v status code: %v", protocol.Name(), err)
	}
	switch packet.Code {
	case CodeOK:
	case CodeError:
		var val string
		err = dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding %v Error value: %v", protocol.Name(), err)
		}
		packet.Val = val
	case CodePing:
		var val PingValue
		err = dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding %v Ping value: %v", protocol.Name(), err)
		}
		packet.Val = val
	case CodePong:
		var val PongValue
		err = dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding %v Pong value: %v", protocol.Name(), err)
		}
		packet.Val = val
	default:
		packet.Val, err = protocol.DecodeValue(packet.Code, dec)
		if err != nil {
			return nil, err
		}
	}
	return &packet, nil
}

func safeEncode(enc *gob.Encoder, val interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Panic encoding: %v", r)
		}
	}()
	err = enc.Encode(val)
	return
}

func (packet *Packet) encodePacket(writer io.Writer, protocol Protocol) error {
	enc := gob.NewEncoder(writer)
	err := safeEncode(enc, packet.Code)
	if err != nil {
		return fmt.Errorf("Error encoding %v status code %v: %v", protocol.Name(), packet.Code, err)
	}
	err = safeEncode(enc, packet.Val)
	if err != nil {
		return fmt.Errorf("Error encoding %v value for code %v: %v", protocol.Name(), packet.Code, err)
	}
	return nil
}

func (packet *Packet) asBytes(protocol Protocol) ([]byte, error) {
	var buf bytes.Buffer
	if err := packet.encodePacket(&buf, protocol); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func receivePacket(conn *net.UDPConn, bufferSize uint, protocol Protocol) (*Packet, error) {
	if bufferSize == 0 {
		bufferSize = protocol.DefaultBufferSize()
	}
	buf := make([]byte, bufferSize)
	n, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(buf[:n])
	packet, err := decodePacket(reader, protocol)
	if err != nil {
		return nil, err
	}
	packet.SourceAddr = addr
	return packet, nil
}

func (packet *Packet) sendPacket(conn *net.UDPConn, addr *net.UDPAddr, protocol Protocol) error {
	b, err := packet.asBytes(protocol)
	if err != nil {
		return err
	}
	_, err = conn.WriteToUDP(b, addr)
	return err
}
