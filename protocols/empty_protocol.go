package protocols

import (
	"encoding/gob"
	"fmt"
)

type EmptyProtocol struct {
}

func (*EmptyProtocol) Name() string {
	return "Empty"
}

func (*EmptyProtocol) DefaultBufferSize() uint {
	return 256
}

func (*EmptyProtocol) DecodeValue(code uint, dec *gob.Decoder) (interface{}, error) {
	return nil, fmt.Errorf("Unknown Empty protocol code: %v", code)
}
