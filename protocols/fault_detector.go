package protocols

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/antongulenko/RTP/helpers"
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

	// Implemented by *faultDetectorBase
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
	callbacks      []faultDetectorCallbackData
	lastErr        error
	lock           sync.Mutex
	protocol       Protocol
	observedServer *net.UDPAddr
}

func (detector *faultDetectorBase) AddCallback(callback FaultDetectorCallback, key interface{}) {
	detector.callbacks = append(detector.callbacks, faultDetectorCallbackData{callback, key})
}

func (detector *faultDetectorBase) Online() bool {
	return detector.DoOnline(func() error { return nil })
}

func (detector *faultDetectorBase) Error() (err error) {
	lastErr := detector.lastErr
	if lastErr != nil {
		var server = ""
		if detector.observedServer != nil {
			server = " on " + detector.observedServer.String()
		}
		err = fmt.Errorf("%v%s is currently offline: %v", detector.protocol.Name(), server, lastErr)
	}
	return
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

func DialNewPingFaultDetector(endpoint string) (FaultDetector, error) {
	client, err := NewEmptyClientFor(endpoint)
	if err != nil {
		return nil, err
	}
	return NewPingFaultDetector(client), nil
}

func NewPingFaultDetector(client ExtendedClient) FaultDetector {
	return &PingFaultDetector{
		faultDetectorBase: faultDetectorBase{
			lastErr:        initialErr,
			protocol:       client.Protocol(),
			observedServer: client.Server(),
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

func (detector *PingFaultDetector) loopCheck(timeout time.Duration) {
	for !detector.client.Closed() {
		detector.Check()
		time.Sleep(timeout)
	}
}

type HeartbeatServer struct {
	*Server
	detectors map[string]*HeartbeatFaultDetector
}

func NewEmptyHeartbeatServer(local_addr string) (*HeartbeatServer, error) {
	if server, err := NewEmptyServer(local_addr); err == nil {
		return NewHeartbeatServer(server), nil
	} else {
		return nil, err
	}
}

func NewHeartbeatServer(server *Server) *HeartbeatServer {
	heartbeatServer := &HeartbeatServer{
		Server:    server,
		detectors: make(map[string]*HeartbeatFaultDetector),
	}
	server.HeartbeatHandler = heartbeatServer.heartbeatReceived
	return heartbeatServer
}

func (server *HeartbeatServer) Stop() {
	for _, detector := range server.detectors {
		if err := detector.Close(); err != nil {
			server.LogError(fmt.Errorf("Error closing %v: %v", detector, err))
		}
	}
	server.Server.Stop()
}

func (server *HeartbeatServer) heartbeatReceived(source *net.UDPAddr, beat *HeartbeatPacket) {
	received := time.Now()
	ip := source.String()
	if detector, ok := server.detectors[ip]; ok {
		detector.heartbeatReceived(received, beat)
	} else {
		server.LogError(fmt.Errorf("Unexpected heartbeat (seq %v) from %v", beat.Seq, source))
	}
}

type HeartbeatFaultDetector struct {
	faultDetectorBase
	server            *HeartbeatServer
	addr              string
	acceptableTimeout time.Duration
	closed            *helpers.OneshotCondition

	seq                   uint64
	lastHeartbeatSent     time.Time
	lastHeartbeatReceived time.Time
}

func (detector *HeartbeatFaultDetector) String() string {
	return fmt.Sprintf("%v-HeartbeatFaultDetector for %v", detector.server.handler.Name(), detector.addr)
}

func (server *HeartbeatServer) ObserveServer(endpoint string, heartbeatFrequency time.Duration, acceptableTimeout time.Duration) (FaultDetector, error) {
	tmpClient, err := NewEmptyClientFor(endpoint)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err = tmpClient.Close(); err != nil {
			server.LogError(fmt.Errorf("Failed to close connection: %v", err))
		}
	}()
	addr := tmpClient.Server().String()
	if _, ok := server.detectors[addr]; ok {
		return nil, fmt.Errorf("Server with address %v is already being observed.", addr)
	}

	detector := &HeartbeatFaultDetector{
		faultDetectorBase: faultDetectorBase{
			lastErr:        initialErr,
			protocol:       server.handler,
			observedServer: tmpClient.Server(),
		},
		addr:                  addr,
		acceptableTimeout:     acceptableTimeout,
		server:                server,
		closed:                helpers.NewOneshotCondition(),
		lastHeartbeatReceived: time.Now(),
	}
	server.detectors[addr] = detector
	err = tmpClient.ConfigureHeartbeat(server.Server, heartbeatFrequency)
	if err == nil {
		delete(server.detectors, addr)
	} else {
		return nil, err
	}
	return detector, nil
}

func (detector *HeartbeatFaultDetector) heartbeatReceived(received time.Time, beat *HeartbeatPacket) {
	if detector.seq != 0 && detector.seq != beat.Seq {
		detector.server.LogError(fmt.Errorf("Heartbeat sequence jump (%v -> %v) for %v", detector.seq, beat.Seq, detector))
	}
	detector.seq = beat.Seq + 1
	detector.lastHeartbeatReceived = received
	detector.lastHeartbeatSent = beat.TimeSent
}

func (detector *HeartbeatFaultDetector) IsStopped() bool {
	return detector.closed.Enabled() || detector.server.Stopped
}

func (detector *HeartbeatFaultDetector) detectTimeouts() {
	for !detector.IsStopped() {
		wasOnline := detector.lastErr == nil
		timeSinceLastHeartbeat := time.Now().Sub(detector.lastHeartbeatReceived)
		isOnline := timeSinceLastHeartbeat <= detector.acceptableTimeout
		if isOnline {
			detector.lastErr = nil
		} else {
			detector.lastErr = fmt.Errorf("Heartbeat timeout: last heartbeats %v ago", timeSinceLastHeartbeat)
		}
		detector.invokeCallback(wasOnline)
	}
	if detector.lastErr == nil {
		detector.lastErr = fmt.Errorf("HeartbeatFaultDetector for %v is closed", detector.addr)
	} else {
		detector.lastErr = fmt.Errorf("HeartbeatFaultDetector for %v is closed. Previous error: %v", detector.lastErr)
	}
	detector.invokeCallback(true)
}

func (detector *HeartbeatFaultDetector) Start() {
	if !detector.IsStopped() {
		go detector.detectTimeouts()
	}
}

func (detector *HeartbeatFaultDetector) Close() error {
	detector.closed.Enable(func() {
		delete(detector.server.detectors, detector.addr)
	})
	return nil
}
