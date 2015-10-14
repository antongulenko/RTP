package protocols

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
)

var (
	Marshaller MarshallingProvider = gobMarshaller
)

type Code uint

type Packet struct {
	Code       Code
	Val        interface{}
	SourceAddr Addr
}

type MarshallingProvider interface {
	MarshalPacket(packet *Packet) ([]byte, error)
	UnmarshalPacket([]byte, Protocol) (*Packet, error)
}

// ========================== gob Marshaller ==========================

var (
	gobMarshaller *gobMarshallingProvider
)

type gobMarshallingProvider struct {
}

func (m *gobMarshallingProvider) MarshalPacket(packet *Packet) ([]byte, error) {
	var buf bytes.Buffer
	if err := m.encode(packet, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (m *gobMarshallingProvider) UnmarshalPacket(buf []byte, protocol Protocol) (*Packet, error) {
	return m.decode(bytes.NewReader(buf), protocol)
}

func (m *gobMarshallingProvider) safeEncode(enc *gob.Encoder, val interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Panic encoding: %v", r)
		}
	}()
	err = enc.Encode(val)
	return
}

func (m *gobMarshallingProvider) encode(packet *Packet, writer io.Writer) error {
	enc := gob.NewEncoder(writer)
	err := m.safeEncode(enc, packet.Code)
	if err != nil {
		return fmt.Errorf("Error encoding status code %v: %v", packet.Code, err)
	}
	err = m.safeEncode(enc, packet.Val)
	if err != nil {
		return fmt.Errorf("Error encoding value for code %v: %v", packet.Code, err)
	}
	return nil
}

func (m *gobMarshallingProvider) decode(reader io.Reader, protocol Protocol) (*Packet, error) {
	dec := gob.NewDecoder(reader)
	var packet Packet
	err := dec.Decode(&packet.Code)
	if err != nil {
		return nil, fmt.Errorf("Error decoding %v status code: %v", protocol.Name(), err)
	}
	// TODO move the gob-specific decoding here completely!
	val, err := protocol.decodeValue(packet.Code, dec)
	if err != nil {
		return nil, err
	}
	packet.Val = val
	return &packet, nil
}
