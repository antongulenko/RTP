package heartbeat

import (
	"fmt"
	"net"
	"time"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
)

// ======================= Server for receiving heartbeats =======================

type HeartbeatServer struct {
	*protocols.Server
	detectors map[string]*HeartbeatFaultDetector
}

type serverStopper struct {
	protocols.Protocol
	*HeartbeatServer
}

func NewHeartbeatServer(local_addr string) (*HeartbeatServer, error) {
	heartbeatServer := &HeartbeatServer{
		detectors: make(map[string]*HeartbeatFaultDetector),
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

func (server *HeartbeatServer) HeartbeatReceived(source *net.UDPAddr, beat *HeartbeatPacket) {
	received := time.Now()
	addr := source.String()
	if detector, ok := server.detectors[addr]; ok {
		detector.heartbeatReceived(received, beat)
	} else {
		server.LogError(fmt.Errorf("Unexpected heartbeat (seq %v) from %v", beat.Seq, addr))
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
	addr := client.Server().String()
	if _, ok := server.detectors[addr]; ok {
		_ = client.Close() // Drop error
		return nil, fmt.Errorf("Server with address %v is already being observed.", addr)
	}

	detector := &HeartbeatFaultDetector{
		FaultDetectorBase:     protocols.NewFaultDetectorBase(server.Protocol(), client.Server()),
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
	detector.configError = detector.client.ConfigureHeartbeat(detector.server.Server, detector.heartbeatFrequency)
	detector.seq = 0
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
	var err helpers.MultiError
	detector.Closed.Enable(func() {
		delete(detector.server.detectors, detector.ObservedServer().String())
		// Notify remote server to stop sending heartbeats.
		err.Add(detector.client.ConfigureHeartbeat(detector.server.Server, 0))
		err.Add(detector.client.Close())
	})
	return err.NilOrError()
}
