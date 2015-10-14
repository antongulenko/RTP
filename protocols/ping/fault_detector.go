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
	server string
}

func NewFaultDetector(client protocols.Client, server string) (*FaultDetector, error) {
	pingClient, err := NewClient(client)
	if err != nil {
		return nil, err
	}
	return &FaultDetector{
		FaultDetectorBase: protocols.NewFaultDetectorBase(client.Protocol(), server),
		client:            pingClient,
		server:            server,
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
	if detector.client.Server() == nil {
		err := detector.client.SetServer(detector.server)
		if err != nil {
			return err
		}
	}
	err := detector.client.Ping()
	// Next ping with fresh connection
	detector.client.ResetConnection()
	return err
}
