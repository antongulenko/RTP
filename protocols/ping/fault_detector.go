package ping

import (
	"time"

	"github.com/antongulenko/RTP/protocols"
)

const (
	default_ping_check_duration = 200 * time.Millisecond
)

type FaultDetector struct {
	*protocols.FaultDetectorBase
	client *Client
}

func NewFaultDetector(client protocols.Client) (*FaultDetector, error) {
	pingClient, err := NewClient(client)
	if err != nil {
		return nil, err
	}
	return &FaultDetector{
		FaultDetectorBase: protocols.NewFaultDetectorBase(client.Protocol(), client.Server()),
		client:            pingClient,
	}, nil
}

func DialNewFaultDetector(endpoint string) (*FaultDetector, error) {
	client, err := protocols.NewClientFor(endpoint, MiniProtocol)
	if err != nil {
		return nil, err
	}
	return NewFaultDetector(client)
}

func (detector *FaultDetector) Start() {
	go detector.LoopCheck(detector.Check, default_ping_check_duration)
}

func (detector *FaultDetector) Close() (err error) {
	detector.Closed.Enable(func() {
		err = detector.client.Close()
	})
	return
}

func (detector *FaultDetector) Check() {
	detector.PerformCheck(detector.client.Ping)
}
