package protocols

import (
	"encoding/gob"
	"fmt"
)

const (
	CodeOK = iota
	CodeError
)
const (
	DefaultBufferSize = 512
)

type Decoder func(decoder *gob.Decoder) (interface{}, error)
type DecoderMap map[Code]Decoder

type ProtocolFragment interface {
	Name() string
	Decoders() DecoderMap
}

type Protocol interface {
	Name() string
	CheckIncludesFragment(fragmentName string) error

	defaultBufferSize() uint
	decodeValue(code Code, decoder *gob.Decoder) (interface{}, error)
	instantiateServer(server *Server) (*serverProtocolInstance, error)
}

type decoderDescription struct {
	decode Decoder
	owner  ProtocolFragment
}

type protocol struct {
	bufferSize uint
	name       string
	fragments  []ProtocolFragment
	decoders   map[Code]decoderDescription
}

func NewProtocol(name string, fragments ...ProtocolFragment) (*protocol, error) {
	decoders := make(map[Code]decoderDescription)
	proto := &protocol{
		bufferSize: DefaultBufferSize,
		name:       name,
		decoders:   decoders,
	}
	fragments = append(fragments, defaultProtocol)
	proto.fragments = fragments
	for _, fragment := range fragments {
		for code, decoder := range fragment.Decoders() {
			if existing, exists := decoders[code]; exists {
				return nil, fmt.Errorf("Code %v used by multiple ProtocolFragments: %v, %v", existing.owner.Name(), fragment.Name())
			}
			decoders[code] = decoderDescription{decoder, fragment}
		}
	}
	return proto, nil
}

func NewMiniProtocol(fragment ProtocolFragment) *protocol {
	proto, err := NewProtocol(fragment.Name(), fragment)
	if err != nil {
		panic(fmt.Errorf("Creating single-fragment protocol should never fail (err: %v)", err))
	}
	return proto
}

func (proto *protocol) CheckIncludesFragment(fragmentName string) error {
	for _, fragment := range proto.fragments {
		if fragment.Name() == fragmentName {
			return nil
		}
	}
	return fmt.Errorf("Protocol %v does not include fragment %v", proto.Name(), fragmentName)
}

func (proto *protocol) Name() string {
	return proto.name
}

func (proto *protocol) defaultBufferSize() uint {
	return proto.bufferSize
}

func (proto *protocol) decodeValue(code Code, decoder *gob.Decoder) (interface{}, error) {
	description, ok := proto.decoders[code]
	if !ok {
		return nil, fmt.Errorf("Packet code %v not registered for %v", code, proto.Name())
	}
	return description.decode(decoder)
}

// =================== Extensions for Server

type ServerRequestHandler func(packet *Packet)
type ServerHandlerMap map[Code]ServerRequestHandler
type ServerStopper func()

// If a ProtocolFragment implements this, the returned handlers will be automatically
// added to a Server
type ServerProtocolFragment interface {
	ProtocolFragment
	ServerHandlers(server *Server) ServerHandlerMap
}

type serverProtocolInstance struct {
	Protocol
	server   *Server
	handlers ServerHandlerMap
	stoppers []ServerStopper
}

func (proto *protocol) instantiateServer(server *Server) (*serverProtocolInstance, error) {
	inst := &serverProtocolInstance{
		handlers: make(ServerHandlerMap),
		Protocol: proto,
		server:   server,
	}
	for _, fragment := range proto.fragments {
		if serverFragment, ok := fragment.(ServerProtocolFragment); ok {
			if err := inst.registerHandlers(serverFragment.ServerHandlers(server)); err != nil {
				return nil, err
			}
		}
	}
	return inst, nil
}

func (inst *serverProtocolInstance) stopServer() {
	for _, stopper := range inst.stoppers {
		if stopper != nil {
			stopper()
		}
	}
}

func (inst *serverProtocolInstance) registerHandlers(handlers ServerHandlerMap) error {
	for code, _ := range handlers {
		if _, ok := inst.handlers[code]; ok {
			return fmt.Errorf("Duplicate ServerRequestHandler for code %v in server %v", code, inst.server)
		}
	}
	for code, handler := range handlers {
		inst.handlers[code] = handler
	}
	return nil
}

func (inst *serverProtocolInstance) registerStopper(stopper ServerStopper) {
	inst.stoppers = append(inst.stoppers, stopper)
}

func (inst *serverProtocolInstance) HandleServerPacket(packet *Packet) {
	code := packet.Code
	handler, ok := inst.handlers[code]
	if !ok {
		inst.server.LogError(fmt.Errorf("Packet code %v not handled %v", code, inst.Name()))
	} else {
		handler(packet)
	}
}

// =================== The default protocol fragment (OK & Error messages)

var defaultProtocol *defaultProtocolFragment

type defaultProtocolFragment struct {
}

type defaultServerState struct {
	*Server
}

func (frag *defaultProtocolFragment) Decoders() DecoderMap {
	return DecoderMap{
		CodeOK:    frag.decodeOK,
		CodeError: frag.decodeError,
	}
}
func (*defaultProtocolFragment) Name() string {
	return "Default"
}
func (frag *defaultProtocolFragment) decodeError(decoder *gob.Decoder) (interface{}, error) {
	var val string
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding Error value: %v", err)
	}
	return val, nil
}
func (*defaultProtocolFragment) decodeOK(decoder *gob.Decoder) (interface{}, error) {
	return nil, nil
}

func (frag *defaultProtocolFragment) ServerHandlers(server *Server) ServerHandlerMap {
	state := &defaultServerState{server}
	return ServerHandlerMap{
		CodeOK:    state.handleOK,
		CodeError: state.handleError,
	}
}
func (state *defaultServerState) handleOK(packet *Packet) {
	state.LogError(fmt.Errorf("Received standalone OK message from %v", packet.SourceAddr))
}
func (state *defaultServerState) handleError(packet *Packet) {
	state.LogError(fmt.Errorf("Received standalone Error message from %v: %v", packet.SourceAddr, packet.Val))
}
