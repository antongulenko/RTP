package protocols

import (
	"fmt"
	"sync"
	"time"
)

const (
	default_ping_check_duration = 200 * time.Millisecond
)

var (
	initialErr = fmt.Errorf("Status was not checked yet")
)

type FaultDetectorCallback func(key interface{})

type FaultDetector interface {
	Start()
	Close() error
	Check()
	Error() error
	Online() bool
	DoOnline(execute func() error) bool
	AddCallback(callback FaultDetectorCallback, key interface{})
}

type faultDetectorCallbackData struct {
	callback FaultDetectorCallback
	key      interface{}
}

type faultDetectorBase struct {
	callbacks []faultDetectorCallbackData
	lastErr   error
	lock      sync.Mutex
}

func (detector *faultDetectorBase) AddCallback(callback FaultDetectorCallback, key interface{}) {
	detector.callbacks = append(detector.callbacks, faultDetectorCallbackData{callback, key})
}

func (detector *faultDetectorBase) Online() bool {
	return detector.DoOnline(func() error { return nil })
}

func (detector *faultDetectorBase) invokeCallback(wasOnline bool) {
	isOnline := detector.lastErr == nil
	if wasOnline != isOnline {
		for _, data := range detector.callbacks {
			data.callback(data.key)
		}
	}
}

func (detector *faultDetectorBase) DoOnline(execute func() error) bool {
	// Double-checked locking for minimum wait-time
	if detector.lastErr == nil {
		detector.lock.Lock()
		wasOnline := detector.lastErr == nil
		defer detector.invokeCallback(wasOnline)
		defer detector.lock.Unlock() // Unlock first, then invoke callback
		if wasOnline {
			detector.lastErr = execute()
			return true
		}
	}
	return false
}

type PingFaultDetector struct {
	faultDetectorBase
	client ExtendedClient
}

func NewPingFaultDetector(client ExtendedClient) FaultDetector {
	return &PingFaultDetector{
		faultDetectorBase: faultDetectorBase{
			lastErr: initialErr,
		},
		client: client,
	}
}

func (detector *PingFaultDetector) Start() {
	go detector.loopCheck(default_ping_check_duration)
}

func (detector *PingFaultDetector) Close() error {
	return detector.client.Close()
}

func (detector *PingFaultDetector) Check() {
	detector.lock.Lock()
	wasOnline := detector.lastErr == nil
	detector.lastErr = detector.client.Ping()
	detector.lock.Unlock()
	detector.invokeCallback(wasOnline)
}

func (detector *PingFaultDetector) Error() (err error) {
	lastErr := detector.lastErr
	if lastErr != nil {
		var server = ""
		if serverAddr := detector.client.Server(); serverAddr != nil {
			server = " on " + serverAddr.String()
		}
		err = fmt.Errorf("%v%s is currently offline: %v",
			detector.client.Protocol().Name(),
			server, lastErr)
	}
	return
}

func (detector *PingFaultDetector) loopCheck(timeout time.Duration) {
	for !detector.client.Closed() {
		detector.Check()
		time.Sleep(timeout)
	}
}
