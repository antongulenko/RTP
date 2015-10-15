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

func NewFaultDetector(client protocols.Client, server string) (*FaultDetector, error) {
	pingClient, err := NewClient(client)
	if err != nil {
		return nil, err
	}
	err = pingClient.SetServer(server)
	if err != nil {
		_ = pingClient.Close()
		return nil, err
	}
	return &FaultDetector{
		FaultDetectorBase: protocols.NewFaultDetectorBase(client.Protocol(), pingClient.Server()),
		client:            pingClient,
	}, nil
}

func DialNewFaultDetector(endpoint string) (*FaultDetector, error) {
	client := protocols.NewClient(MiniProtocol)
	return NewFaultDetector(client, endpoint)
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
	detector.PerformCheck(detector.doPing)
}

func (detector *FaultDetector) doPing() error {
	err := detector.client.Ping()
	// Next ping with fresh connection
	detector.client.ResetConnection()
	return err
}
