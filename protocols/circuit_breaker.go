package protocols

import (
	"net"
	"time"

	"github.com/antongulenko/RTP/helpers"
)

const (
	checkRequestTimeout = 300 * time.Millisecond
)

type CircuitBreaker interface {
	ExtendedClient

	Error() error
	Online() bool

	AddCallback(callback FaultDetectorCallback, key interface{})
}

type circuitBreaker struct {
	ExtendedClient // Extended self
	FaultDetector

	client ExtendedClient // Backend client for the actual operations
}

func NewCircuitBreaker(client ExtendedClient, detector FaultDetector) CircuitBreaker {
	client.SetTimeout(checkRequestTimeout)
	breaker := &circuitBreaker{
		client:        client,
		FaultDetector: detector,
	}
	breaker.ExtendedClient = ExtendClient(breaker)
	return breaker
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
	// This can be wrong if the FaultDetector uses a different server
	return breaker.client.SetServer(server_addr)
}

func (breaker *circuitBreaker) Server() *net.UDPAddr {
	return breaker.client.Server()
}

func (breaker *circuitBreaker) String() string {
	return breaker.client.String()
}

func (breaker *circuitBreaker) SetTimeout(timeout time.Duration) {
	// no-op, circuitBreaker controls its own timeout
}

func (breaker *circuitBreaker) Protocol() Protocol {
	return breaker.client.Protocol()
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
