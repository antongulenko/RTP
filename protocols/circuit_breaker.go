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

	Start()
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

func (breaker *circuitBreaker) SendPacket(packet *Packet) (err error) {
	online := breaker.DoOnline(func() error {
		err = breaker.client.SendPacket(packet)
		return err
	})
	if !online {
		err = breaker.Error()
	}
	return
}

func (breaker *circuitBreaker) SendRequestPacket(packet *Packet) (reply *Packet, err error) {
	online := breaker.DoOnline(func() error {
		reply, err = breaker.client.SendRequestPacket(packet)
		return err
	})
	if !online {
		err = breaker.Error()
	}
	return
}
