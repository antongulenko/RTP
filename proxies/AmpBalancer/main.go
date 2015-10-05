package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"time"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/balancer"
	"github.com/antongulenko/RTP/proxies/amp_balancer"
)

var (
	amp_servers      = []string{"127.0.0.1:7777"}
	pcp_servers      = []string{"127.0.0.1:7778", "0.0.0.0:7776"}
	heartbeat_server = "0.0.0.0:9122" // Random port
)

func printServerErrors(servername string, server *protocols.Server) {
	for err := range server.Errors() {
		log.Println(servername + " error: " + err.Error())
	}
}

func printSessionStarted(session *protocols.PluginSession) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Started session for %v (", session.Client)
	for i, plugin := range session.Plugins {
		if i != 0 {
			fmt.Fprintf(&buf, ", ")
		}
		fmt.Fprintf(&buf, "%v", plugin)
	}
	fmt.Fprintf(&buf, ")")
	log.Println(buf.String())
}

func printSessionStopped(session *protocols.PluginSession) {
	log.Printf("Stopped session for %v\n", session.Client)
}

func stateChangePrinter(key interface{}) {
	breaker, ok := key.(protocols.CircuitBreaker)
	if !ok {
		log.Printf("Failed to convert %v (%T) to CircuitBreaker\n", key, key)
		return
	}
	err, server := breaker.Error(), breaker.String()
	if err != nil {
		log.Printf("%s down: %v\n", server, err)
	} else {
		log.Printf("%s up\n", server)
	}
}

func main() {
	useHeartbeat := flag.Bool("heartbeat", false, "Use heartbeat-based fault detection instead of active ping-based detection")
	_heartbeat_frequency := flag.Uint("heartbeat_frequency", 200, "Time between two heartbeats which observers will send (milliseconds)")
	_heartbeat_timeout := flag.Uint("heartbeat_timeout", 350, "Time between two heartbeats before assuming offline server (milliseconds)")
	amp_addr := protocols.ParseServerFlags("0.0.0.0", 7779)
	heartbeat_frequency := time.Duration(*_heartbeat_frequency) * time.Millisecond
	heartbeat_timeout := time.Duration(*_heartbeat_timeout) * time.Millisecond

	var detector_factory balancer.FaultDetectorFactory
	var observees []Observee
	var heartbeatServer *protocols.HeartbeatServer
	if *useHeartbeat {
		var err error
		heartbeatServer, err = protocols.NewEmptyHeartbeatServer(heartbeat_server)
		Checkerr(err)
		go printServerErrors("Heartbeat", heartbeatServer.Server)
		log.Println("Listening for Heartbeats on", heartbeatServer.LocalAddr)
		detector_factory = func(endpoint string) (protocols.FaultDetector, error) {
			return heartbeatServer.ObserveServer(endpoint, heartbeat_frequency, heartbeat_timeout)
		}
		observees = append(observees, heartbeatServer)
	} else {
		detector_factory = func(endpoint string) (protocols.FaultDetector, error) {
			detector, err := protocols.DialNewPingFaultDetector(endpoint)
			if err != nil {
				return nil, err
			}
			detector.Start()
			return detector, nil
		}
	}

	ampPlugin := amp_balancer.NewAmpBalancingPlugin(detector_factory)
	for _, amp := range amp_servers {
		err := ampPlugin.AddBackendServer(amp, stateChangePrinter)
		Checkerr(err)
	}
	pcpPlugin := amp_balancer.NewPcpBalancingPlugin(detector_factory)
	for _, pcp := range pcp_servers {
		err := pcpPlugin.AddBackendServer(pcp, stateChangePrinter)
		Checkerr(err)
	}

	server, err := amp_balancer.NewAmpPluginServer(amp_addr)
	Checkerr(err)
	observees = append(observees, server)
	server.AddPlugin(ampPlugin)
	server.AddPlugin(pcpPlugin)

	go printServerErrors("Server", server.Server)
	server.SessionStartedCallback = printSessionStarted
	server.SessionStoppedCallback = printSessionStopped

	log.Println("Listening to AMP on " + amp_addr)
	log.Println("Press Ctrl-C to close")

	server.Start()
	if heartbeatServer != nil {
		heartbeatServer.Start()
	}

	observees = append(observees, &NoopObservee{ExternalInterrupt(), "external interrupt"})
	WaitAndStopObservees(nil, observees)
}
