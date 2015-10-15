package protocols

import (
	"fmt"
	"time"

	"github.com/antongulenko/RTP/helpers"
)

const (
	default_ping_check_duration = 200 * time.Millisecond
)

var (
	stateUnknown = fmt.Errorf("Status was not checked yet")
)

type FaultDetectorCallback func(key interface{})

type FaultDetector interface {
	Close() error
	Check()

	// Implemented by *FaultDetectorBase
	Error() error
	Online() bool
	ErrorDetected(err error)
	ObservedServer() Addr
	AddCallback(callback FaultDetectorCallback, key interface{})
}

type faultDetectorCallbackData struct {
	callback FaultDetectorCallback
	key      interface{}
}

type observedServer struct {
	protocol Protocol
	addr     Addr
}

type FaultDetectorBase struct {
	callbacks      []faultDetectorCallbackData
	lastErr        error
	observedServer observedServer
	Closed         *helpers.OneshotCondition
}

func NewFaultDetectorBase(observedProtocol Protocol, server Addr) *FaultDetectorBase {
	return &FaultDetectorBase{
		lastErr: stateUnknown,
		observedServer: observedServer{
			observedProtocol,
			server,
		},
		Closed: helpers.NewOneshotCondition(),
	}
}

func (detector *FaultDetectorBase) ObservedServer() Addr {
	return detector.observedServer.addr
}

func (detector *FaultDetectorBase) AddCallback(callback FaultDetectorCallback, key interface{}) {
	detector.callbacks = append(detector.callbacks, faultDetectorCallbackData{callback, key})
}

func (detector *FaultDetectorBase) ErrorDetected(err error) {
	detector.lastErr = err
}

func (detector *FaultDetectorBase) Online() bool {
	return detector.lastErr == nil
}

func (detector *FaultDetectorBase) Error() (err error) {
	lastErr := detector.lastErr
	if lastErr != nil {
		err = fmt.Errorf("%v on %s is currently offline: %v",
			detector.observedServer.protocol.Name(), detector.observedServer.addr, lastErr)
	}
	return
}

func (detector *FaultDetectorBase) InvokeCallback(wasOnline bool) {
	isOnline := detector.lastErr == nil
	if wasOnline != isOnline && detector.lastErr != stateUnknown {
		for _, data := range detector.callbacks {
			data.callback(data.key)
		}
	}
}

func (detector *FaultDetectorBase) PerformCheck(checker func() error) {
	wasOnline := detector.Online()
	detector.lastErr = checker()
	detector.InvokeCallback(wasOnline)
}

func (detector *FaultDetectorBase) LoopCheck(checker func(), timeout time.Duration) {
	for !detector.Closed.Enabled() {
		checker()
		time.Sleep(timeout)
	}
	if detector.lastErr == nil {
		detector.lastErr = fmt.Errorf("FaultDetector for %v is closed", detector.observedServer.addr)
	} else {
		detector.lastErr = fmt.Errorf("FaultDetector for %v is closed. Previous error: %v", detector.observedServer.addr, detector.lastErr)
	}
	detector.InvokeCallback(detector.lastErr == nil)
}
