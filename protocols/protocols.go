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
	CodeOther
)

var (
	PacketOK = Packet{Code: CodeOK, Val: "OK"}
)

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

type IPacket interface {
	SendRequest(conn *net.UDPConn, addr *net.UDPAddr) (reply *Packet, err error)
}

type ProtocolReader func(code uint, decoder *gob.Decoder) (interface{}, error)

func ReadPacket(reader io.Reader, protocol ProtocolReader) (*Packet, error) {
	dec := gob.NewDecoder(reader)
	var packet Packet
	err := dec.Decode(&packet.Code)
	if err != nil {
		return nil, fmt.Errorf("Error decoding status code: %v", err)
	}
	switch packet.Code {
	case CodeOK:
	case CodeError:
		var val string
		err = dec.Decode(&val)
		if err != nil {
			return nil, fmt.Errorf("Error decoding Error value: %v", err)
		}
		packet.Val = val
	default:
		packet.Val, err = protocol(packet.Code, dec)
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

func (packet *Packet) WritePacket(writer io.Writer) error {
	enc := gob.NewEncoder(writer)
	err := safeEncode(enc, packet.Code)
	if err != nil {
		return fmt.Errorf("Error encoding status code %v: %v", packet.Code, err)
	}
	err = safeEncode(enc, packet.Val)
	if err != nil {
		return fmt.Errorf("Error encoding value for code %v: %v", packet.Code, err)
	}
	return nil
}

func ReceivePacket(conn *net.UDPConn, bufferSize uint, protocol ProtocolReader) (*Packet, error) {
	buf := make([]byte, bufferSize)
	n, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(buf[:n])
	packet, err := ReadPacket(reader, protocol)
	if err != nil {
		return nil, err
	}
	packet.SourceAddr = addr
	return packet, nil
}

func (packet *Packet) asBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := packet.WritePacket(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (packet *Packet) SendPacket(conn *net.UDPConn) error {
	b, err := packet.asBytes()
	if err != nil {
		return err
	}
	_, err = conn.Write(b)
	return err
}

func (packet *Packet) SendPacketTo(conn *net.UDPConn, addr *net.UDPAddr) error {
	b, err := packet.asBytes()
	if err != nil {
		return err
	}
	_, err = conn.WriteToUDP(b, addr)
	return err
}

func (packet *Packet) SendRawRequest(conn *net.UDPConn, addr *net.UDPAddr, replyBufferSize uint, protocol ProtocolReader) (reply *Packet, err error) {
	if err = packet.SendPacketTo(conn, addr); err == nil {
		reply, err = ReceivePacket(conn, replyBufferSize, protocol)
	}
	return
}
