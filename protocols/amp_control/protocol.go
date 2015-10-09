package amp_control

// AMP extension for controlling running streams

import (
	"encoding/gob"
	"fmt"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

var (
	Protocol     *ampControlProtocol
	MiniProtocol = protocols.NewMiniProtocol(Protocol)
)

const (
	CodeRedirectStream = protocols.Code(17 + iota)
	CodePauseStream
	CodeResumeStream
)

// ======================= Packets =======================

type RedirectStream struct {
	OldClient amp.ClientDescription
	NewClient amp.ClientDescription
}

type PauseStream struct {
	amp.ClientDescription
}

type ResumeStream struct {
	amp.ClientDescription
}

// ======================= Protocol =======================

type ampControlProtocol struct {
}

func (*ampControlProtocol) Name() string {
	return "AMPcontrol"
}

func (proto *ampControlProtocol) Decoders() protocols.DecoderMap {
	return protocols.DecoderMap{
		CodeRedirectStream: proto.decodeRedirectStream,
		CodePauseStream:    proto.decodePauseStream,
		CodeResumeStream:   proto.decodeResumeStream,
	}
}

func (proto *ampControlProtocol) decodeRedirectStream(decoder *gob.Decoder) (interface{}, error) {
	var val RedirectStream
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding AMPcontrol RedirectStream value: %v", err)
	}
	return &val, nil
}
func (proto *ampControlProtocol) decodePauseStream(decoder *gob.Decoder) (interface{}, error) {
	var val PauseStream
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding AMPcontrol PauseStream value: %v", err)
	}
	return &val, nil
}
func (proto *ampControlProtocol) decodeResumeStream(decoder *gob.Decoder) (interface{}, error) {
	var val ResumeStream
	err := decoder.Decode(&val)
	if err != nil {
		return nil, fmt.Errorf("Error decoding AMPcontrol ResumeStream value: %v", err)
	}
	return &val, nil
}
