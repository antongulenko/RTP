package protocols

import (
	"encoding/gob"
	"fmt"
)

var (
	EmptyProtocol *emptyProtocol // "Singleton"
)

type emptyProtocol struct {
}

func (*emptyProtocol) Name() string {
	return "Empty"
}

func (*emptyProtocol) DefaultBufferSize() uint {
	return 256
}

func (*emptyProtocol) DecodeValue(code uint, dec *gob.Decoder) (interface{}, error) {
	return nil, fmt.Errorf("Unknown Empty protocol code: %v", code)
}

type EmptyServerHandler struct {
	emptyProtocol
	*Server
}

func (handler *EmptyServerHandler) StopServer() {
	// Nothing.
}
func (handler *EmptyServerHandler) HandleRequest(request *Packet) {
	handler.LogError(fmt.Errorf("Received unexpected Empty code: %v", request.Code))
}

func NewEmptyServer(local_addr string) (*Server, error) {
	handler := new(EmptyServerHandler)
	var err error
	handler.Server, err = NewServer(local_addr, handler)
	if err != nil {
		return nil, err
	}
	return handler.Server, nil
}

func NewEmptyClientFor(local_addr string) (ExtendedClient, error) {
	if client, err := NewClientFor(local_addr, EmptyProtocol); err == nil {
		return ExtendClient(client), nil
	} else {
		return nil, err
	}
}
