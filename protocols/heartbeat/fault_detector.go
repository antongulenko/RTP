package heartbeat

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/golib"
)

// ======================= Server for receiving heartbeats =======================

var (
	tokenRand = rand.New(rand.NewSource(time.Now().Unix()))
)

type HeartbeatServer struct {
	*protocols.Server
	detectors map[int64]*HeartbeatFaultDetector
}

type serverStopper struct {
	protocols.Protocol
	*HeartbeatServer
}

func NewHeartbeatServer(local_addr string) (*HeartbeatServer, error) {
	heartbeatServer := &HeartbeatServer{
		detectors: make(map[int64]*HeartbeatFaultDetector),
	}
	if server, err := protocols.NewServer(local_addr, &serverStopper{MiniProtocol, heartbeatServer}); err == nil {
		heartbeatServer.Server = server
		if err = RegisterServerHandler(server, heartbeatServer); err == nil {
			return heartbeatServer, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (server *HeartbeatServer) Start() {
	server.Server.Start()
	for _, detector := range server.detectors {
		detector.Start()
	}
}

func (server *serverStopper) StopServer() {
	for _, detector := range server.detectors {
		if err := detector.Close(); err != nil {
			server.LogError(fmt.Errorf("Error closing %v: %v", detector, err))
		}
	}
}

func (server *HeartbeatServer) HeartbeatReceived(beat *HeartbeatPacket) {
	received := time.Now()
	token := beat.Token
	if detector, ok := server.detectors[token]; ok {
		detector.heartbeatReceived(received, beat)
	} else {
		server.LogError(fmt.Errorf("Unexpected heartbeat (seq %v) from %v", beat.Seq, beat.Source))
	}
}

// ======================= FaultDetector interface =======================

type HeartbeatFaultDetector struct {
	*protocols.FaultDetectorBase
	server             *HeartbeatServer
	client             *Client
	configError        error
	acceptableTimeout  time.Duration
	heartbeatFrequency time.Duration

	token                 int64
	seq                   uint64
	lastHeartbeatSent     time.Time
	lastHeartbeatReceived time.Time
}

func (detector *HeartbeatFaultDetector) String() string {
	return fmt.Sprintf("%v-HeartbeatFaultDetector for %v", detector.server.Protocol().Name(), detector.ObservedServer())
}

func (server *HeartbeatServer) ObserveServer(endpoint string, heartbeatFrequency time.Duration, acceptableTimeout time.Duration) (protocols.FaultDetector, error) {
	client, err := NewClientFor(endpoint)
	if err != nil {
		return nil, err
	}
	var token int64
	for {
		token = tokenRand.Int63()
		if _, ok := server.detectors[token]; !ok && token != 0 {
			break
		}
	}
	detector := &HeartbeatFaultDetector{
		FaultDetectorBase:  protocols.NewFaultDetectorBase(server.Protocol(), client.Server()),
		client:             client,
		acceptableTimeout:  acceptableTimeout,
		heartbeatFrequency: heartbeatFrequency,
		server:             server,
		token:              token,
		lastHeartbeatReceived: time.Now(),
	}
	server.detectors[token] = detector
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
	return detector.Closed.Enabled() || detector.server.Stopped
}

func (detector *HeartbeatFaultDetector) Check() {
	detector.PerformCheck(func() error {
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
	detector.seq = 0
	detector.configError = detector.client.ConfigureHeartbeat(detector.server.Server, detector.token, detector.heartbeatFrequency)
	if detector.configError != nil {
		detector.client.ResetConnection()
	}
}

func (detector *HeartbeatFaultDetector) Start() {
	if !detector.IsStopped() {
		detector.configureObservedServer() // Once when starting up
		go func() {
			// TODO sleep something less than the timeout. This is random and probably will not scale.
			timeout := detector.acceptableTimeout
			time.Sleep(timeout) // Sleep now to wait for first heartbeat
			detector.LoopCheck(detector.Check, timeout)
		}()
	}
}

func (detector *HeartbeatFaultDetector) Close() error {
	var err golib.MultiError
	detector.Closed.Enable(func() {
		delete(detector.server.detectors, detector.token)
		// Notify remote server to stop sending heartbeats.
		if detector.Online() {
			err.Add(detector.client.ConfigureHeartbeat(detector.server.Server, 0, 0))
		}
		err.Add(detector.client.Close())
	})
	return err.NilOrError()
}
