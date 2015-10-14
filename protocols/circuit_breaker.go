package protocols

import (
	"fmt"
	"time"

	"github.com/antongulenko/RTP/helpers"
)

const (
	checkRequestTimeout = 300 * time.Millisecond
)

type CircuitBreaker interface {
	Client

	Error() error
	Online() bool
	AddCallback(callback FaultDetectorCallback, key interface{})
}

type circuitBreaker struct {
	FaultDetector
	client Client // Backend client for the actual operations
}

func NewCircuitBreakerOn(localAddr string, protocol Protocol, detector FaultDetector) (CircuitBreaker, error) {
	client, err := NewClient(localAddr, protocol)
	if err != nil {
		return nil, err
	}
	return NewCircuitBreaker(client, detector)
}

func NewCircuitBreaker(client Client, detector FaultDetector) (CircuitBreaker, error) {
	breaker := &circuitBreaker{
		client:        client,
		FaultDetector: detector,
	}
	client.SetTimeout(checkRequestTimeout)
	if err := client.SetServer(detector.ObservedServer().String()); err != nil {
		return nil, err
	}
	return breaker, nil
}

func (breaker *circuitBreaker) Close() error {
	var err helpers.MultiError
	err.Add(breaker.client.Close())
	err.Add(breaker.FaultDetector.Close())
	return err.NilOrError()
}

func (breaker *circuitBreaker) Closed() bool {
	return breaker.client.Closed()
}

func (breaker *circuitBreaker) SetServer(server_addr string) error {
	if breaker.ObservedServer().String() != server_addr {
		return fmt.Errorf("Cannot SetServer with address different from FaultDetector. Have %v, received %v.", breaker.ObservedServer(), server_addr)
	}
	return nil
}

func (breaker *circuitBreaker) Server() Addr {
	return breaker.client.Server()
}

func (breaker *circuitBreaker) String() string {
	return fmt.Sprintf("CircuitBreaker(%v)", breaker.client.String())
}

func (breaker *circuitBreaker) SetTimeout(timeout time.Duration) {
	// no-op, circuitBreaker controls its own timeout
}

func (breaker *circuitBreaker) Protocol() Protocol {
	return breaker.client.Protocol()
}

func (breaker *circuitBreaker) CheckError(reply *Packet, expectedCode Code) error {
	return breaker.client.CheckError(reply, expectedCode)
}

func (breaker *circuitBreaker) CheckReply(reply *Packet) error {
	return breaker.client.CheckReply(reply)
}

func (breaker *circuitBreaker) SendPacket(packet *Packet) error {
	if breaker.Online() {
		err := breaker.client.SendPacket(packet)
		if err != nil {
			breaker.ErrorDetected(err)
		}
		return err
	} else {
		return breaker.Error()
	}
}

func (breaker *circuitBreaker) SendRequestPacket(packet *Packet) (reply *Packet, err error) {
	if breaker.Online() {
		reply, err := breaker.client.SendRequestPacket(packet)
		if err != nil {
			breaker.ErrorDetected(err)
		}
		return reply, err
	} else {
		return nil, breaker.Error()
	}
}

func (breaker *circuitBreaker) Send(code Code, val interface{}) error {
	return breaker.SendPacket(&Packet{
		Code: code,
		Val:  val,
	})
}

func (breaker *circuitBreaker) SendRequest(code Code, val interface{}) (*Packet, error) {
	return breaker.SendRequestPacket(&Packet{
		Code: code,
		Val:  val,
	})
}
