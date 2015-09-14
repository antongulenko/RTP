package protocols

import (
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	checkRequestTimeout = 300 * time.Millisecond
	checkPeriod         = 200 * time.Millisecond
)

type CircuitBreaker interface {
	ExtendedClient

	Start()
	Check()
	Error() error
	Online() bool

	SetStateChangedCallback(callback func(err error))
}

type circuitBreaker struct {
	ExtendedClient // Extended self

	client   ExtendedClient
	lastErr  error
	lock     sync.Mutex
	callback func(err error)
}

func NewCircuitBreaker(client ExtendedClient) CircuitBreaker {
	client.SetTimeout(checkRequestTimeout)
	breaker := &circuitBreaker{
		client: client,
	}
	breaker.ExtendedClient = ExtendClient(breaker)
	return breaker
}

func (breaker *circuitBreaker) Start() {
	go breaker.loopCheck()
}

func (breaker *circuitBreaker) Error() (err error) {
	lastErr := breaker.lastErr
	if lastErr != nil {
		var server = ""
		if serverAddr := breaker.client.Server(); serverAddr != nil {
			server = " on " + serverAddr.String()
		}
		err = fmt.Errorf("%v%s is currently offline: %v",
			breaker.client.Protocol().Name(),
			server, lastErr)
	}
	return
}

func (breaker *circuitBreaker) loopCheck() {
	for !breaker.client.Closed() {
		breaker.Check()
		time.Sleep(checkPeriod)
	}
}

func (breaker *circuitBreaker) invokeCallback(wasOnline bool) {
	if breaker.callback != nil {
		isOnline := breaker.lastErr == nil
		if wasOnline != isOnline {
			breaker.callback(breaker.lastErr)
		}
	}
}

func (breaker *circuitBreaker) lockedOnline(execute func() error) bool {
	// Double-checked locking for minimum wait-time
	if breaker.lastErr == nil {
		breaker.lock.Lock()
		wasOnline := breaker.lastErr == nil
		defer breaker.invokeCallback(wasOnline)
		defer breaker.lock.Unlock() // Unlock first, then invoke callback
		if wasOnline {
			breaker.lastErr = execute()
			return true
		}
	}
	return false
}

func (breaker *circuitBreaker) SetStateChangedCallback(callback func(err error)) {
	breaker.callback = callback
}

func (breaker *circuitBreaker) Check() {
	breaker.lock.Lock()
	wasOnline := breaker.lastErr == nil
	breaker.lastErr = breaker.client.Ping()
	breaker.lock.Unlock()
	breaker.invokeCallback(wasOnline)
}

func (breaker *circuitBreaker) Online() bool {
	return breaker.lockedOnline(func() error { return nil })
}

func (breaker *circuitBreaker) Close() error {
	return breaker.client.Close()
}

func (breaker *circuitBreaker) Closed() bool {
	return breaker.client.Closed()
}

func (breaker *circuitBreaker) SetServer(server_addr string) error {
	return breaker.client.SetServer(server_addr)
}

func (breaker *circuitBreaker) Server() *net.UDPAddr {
	return breaker.client.Server()
}

func (breaker *circuitBreaker) SetTimeout(timeout time.Duration) {
	// no-op, circuitBreaker controls its own timeout
}

func (breaker *circuitBreaker) Protocol() Protocol {
	return breaker.client.Protocol()
}

func (breaker *circuitBreaker) SendPacket(packet *Packet) (err error) {
	online := breaker.lockedOnline(func() error {
		err = breaker.client.SendPacket(packet)
		return err
	})
	if !online {
		err = breaker.Error()
	}
	return
}

func (breaker *circuitBreaker) SendRequestPacket(packet *Packet) (reply *Packet, err error) {
	online := breaker.lockedOnline(func() error {
		reply, err = breaker.client.SendRequestPacket(packet)
		return err
	})
	if !online {
		err = breaker.Error()
	}
	return
}
