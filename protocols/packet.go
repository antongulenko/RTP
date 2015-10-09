package protocols

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net"
)

type Code uint

type Packet struct {
	Code       Code
	Val        interface{}
	SourceAddr *net.UDPAddr
}

func decodePacket(reader io.Reader, protocol Protocol) (*Packet, error) {
	dec := gob.NewDecoder(reader)
	var packet Packet
	err := dec.Decode(&packet.Code)
	if err != nil {
		return nil, fmt.Errorf("Error decoding %v status code: %v", protocol.Name(), err)
	}
	val, err := protocol.decodeValue(packet.Code, dec)
	if err != nil {
		return nil, err
	}
	packet.Val = val
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
		bufferSize = protocol.defaultBufferSize()
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
