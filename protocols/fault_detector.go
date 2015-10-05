package protocols

import (
	"fmt"
	"net"
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

	// Implemented by *faultDetectorBase
	Error() error
	Online() bool
	ErrorDetected(err error)
	AddCallback(callback FaultDetectorCallback, key interface{})
}

type faultDetectorCallbackData struct {
	callback FaultDetectorCallback
	key      interface{}
}

type faultDetectorBase struct {
	callbacks      []faultDetectorCallbackData
	lastErr        error
	protocol       Protocol
	observedServer *net.UDPAddr
	closed         *helpers.OneshotCondition
}

func (detector *faultDetectorBase) AddCallback(callback FaultDetectorCallback, key interface{}) {
	detector.callbacks = append(detector.callbacks, faultDetectorCallbackData{callback, key})
}

func (detector *faultDetectorBase) ErrorDetected(err error) {
	detector.lastErr = err
}

func (detector *faultDetectorBase) Online() bool {
	return detector.lastErr == nil
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
	if wasOnline != isOnline && detector.lastErr != stateUnknown {
		for _, data := range detector.callbacks {
			data.callback(data.key)
		}
	}
}

func (detector *faultDetectorBase) performCheck(checker func() error) {
	wasOnline := detector.Online()
	detector.lastErr = checker()
	detector.invokeCallback(wasOnline)
}

func (detector *faultDetectorBase) loopCheck(checker func(), timeout time.Duration) {
	for !detector.closed.Enabled() {
		checker()
		time.Sleep(timeout)
	}
	if detector.lastErr == nil {
		detector.lastErr = fmt.Errorf("FaultDetector for %v is closed", detector.observedServer)
	} else {
		detector.lastErr = fmt.Errorf("FaultDetector for %v is closed. Previous error: %v", detector.observedServer, detector.lastErr)
	}
	detector.invokeCallback(detector.lastErr == nil)
}

type PingFaultDetector struct {
	faultDetectorBase
	client ExtendedClient
}

func DialNewPingFaultDetector(endpoint string) (*PingFaultDetector, error) {
	client, err := NewEmptyClientFor(endpoint)
	if err != nil {
		return nil, err
	}
	return NewPingFaultDetector(client), nil
}

func NewPingFaultDetector(client ExtendedClient) *PingFaultDetector {
	return &PingFaultDetector{
		faultDetectorBase: faultDetectorBase{
			lastErr:        stateUnknown,
			protocol:       client.Protocol(),
			observedServer: client.Server(),
			closed:         helpers.NewOneshotCondition(),
		},
		client: client,
	}
}

func (detector *PingFaultDetector) Start() {
	go detector.loopCheck(detector.Check, default_ping_check_duration)
}

func (detector *PingFaultDetector) Close() (err error) {
	detector.closed.Enable(func() {
		err = detector.client.Close()
	})
	return
}

func (detector *PingFaultDetector) Check() {
	detector.performCheck(detector.client.Ping)
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

func (server *HeartbeatServer) Start() {
	server.Server.Start()
	for _, detector := range server.detectors {
		detector.Start()
	}
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
	addr := source.String()
	if detector, ok := server.detectors[addr]; ok {
		detector.heartbeatReceived(received, beat)
	} else {
		server.LogError(fmt.Errorf("Unexpected heartbeat (seq %v) from %v", beat.Seq, addr))
	}
}

type HeartbeatFaultDetector struct {
	faultDetectorBase
	server             *HeartbeatServer
	client             ExtendedClient
	configError        error
	acceptableTimeout  time.Duration
	heartbeatFrequency time.Duration

	seq                   uint64
	lastHeartbeatSent     time.Time
	lastHeartbeatReceived time.Time
}

func (detector *HeartbeatFaultDetector) String() string {
	return fmt.Sprintf("%v-HeartbeatFaultDetector for %v", detector.server.handler.Name(), detector.observedServer)
}

func (server *HeartbeatServer) ObserveServer(endpoint string, heartbeatFrequency time.Duration, acceptableTimeout time.Duration) (FaultDetector, error) {
	client, err := NewEmptyClientFor(endpoint)
	if err != nil {
		return nil, err
	}
	addr := client.Server().String()
	if _, ok := server.detectors[addr]; ok {
		_ = client.Close() // Drop error
		return nil, fmt.Errorf("Server with address %v is already being observed.", addr)
	}

	detector := &HeartbeatFaultDetector{
		faultDetectorBase: faultDetectorBase{
			lastErr:        stateUnknown,
			protocol:       server.handler,
			observedServer: client.Server(),
			closed:         helpers.NewOneshotCondition(),
		},
		client:                client,
		acceptableTimeout:     acceptableTimeout,
		heartbeatFrequency:    heartbeatFrequency,
		server:                server,
		lastHeartbeatReceived: time.Now(),
	}
	server.detectors[addr] = detector
	return detector, nil
}

func (detector *HeartbeatFaultDetector) heartbeatReceived(received time.Time, beat *HeartbeatPacket) {
	if detector.seq != 0 && detector.seq != beat.Seq {
		detector.server.LogError(fmt.Errorf("Heartbeat sequence jump (%v -> %v) for %v", detector.seq, beat.Seq, detector))
	}
	detector.seq = beat.Seq + 1
	detector.lastHeartbeatReceived = received
	detector.lastHeartbeatSent = beat.TimeSent
	detector.Check()
}

func (detector *HeartbeatFaultDetector) IsStopped() bool {
	return detector.closed.Enabled() || detector.server.Stopped
}

func (detector *HeartbeatFaultDetector) Check() {
	detector.performCheck(func() error {
		timeSinceLastHeartbeat := time.Now().Sub(detector.lastHeartbeatReceived)
		if timeSinceLastHeartbeat <= detector.acceptableTimeout {
			return nil
		} else {
			var configErr string
			if detector.configError != nil {
				configErr = fmt.Sprintf(". Error configuring remote server: %v", detector.configError)
			}
			return fmt.Errorf("Heartbeat timeout: last heartbeat %v ago%s", timeSinceLastHeartbeat, configErr)
		}
	})
	if !detector.Online() {
		detector.configureObservedServer()
	}
}

func (detector *HeartbeatFaultDetector) configureObservedServer() {
	detector.configError = detector.client.ConfigureHeartbeat(detector.server.Server, detector.heartbeatFrequency)
	detector.seq = 0
}

func (detector *HeartbeatFaultDetector) Start() {
	if !detector.IsStopped() {
		detector.configureObservedServer() // Once when starting up
		func() {
			// TODO sleep something less than the timeout. This is random and probably will not scale.
			timeout := detector.acceptableTimeout
			time.Sleep(timeout) // Sleep now to wait for first heartbeat
			go detector.loopCheck(detector.Check, timeout)
		}()
	}
}

func (detector *HeartbeatFaultDetector) Close() error {
	var err helpers.MultiError
	detector.closed.Enable(func() {
		delete(detector.server.detectors, detector.observedServer.String())
		// Notify remote server to stop sending heartbeats.
		err.Add(detector.client.ConfigureHeartbeat(detector.server.Server, 0))
		err.Add(detector.client.Close())
	})
	return err.NilOrError()
}
