package main

import (
	"log"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/proxies/amp_balancer"
)

var (
	amp_servers = []string{"127.0.0.1:7777"}
	pcp_servers = []string{"127.0.0.1:7778", "0.0.0.0:7776"}
)

func printAmpErrors(server *amp_balancer.ExtendedAmpServer) {
	for err := range server.Errors() {
		log.Println("Server error: " + err.Error())
	}
}

func printSessionStarted(client string) {
	log.Println("Started session for", client)
}

func printSessionStopped(client string) {
	log.Println("Stopped session for", client)
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
	amp_addr := protocols.ParseServerFlags("0.0.0.0", 7779)
	ampPlugin := amp_balancer.NewAmpBalancingPlugin()
	for _, amp := range amp_servers {
		err := ampPlugin.AddBackendServer(amp, stateChangePrinter)
		Checkerr(err)
	}
	pcpPlugin := amp_balancer.NewPcpBalancingPlugin()
	for _, pcp := range pcp_servers {
		err := pcpPlugin.AddBackendServer(pcp, stateChangePrinter)
		Checkerr(err)
	}

	server, err := amp_balancer.NewExtendedAmpServer(amp_addr)
	Checkerr(err)
	server.AddPlugin(ampPlugin)
	server.AddPlugin(pcpPlugin)

	go printAmpErrors(server)
	server.SessionStartedCallback = printSessionStarted
	server.SessionStoppedCallback = printSessionStopped

	server.Start()

	log.Println("Listening to AMP on " + amp_addr)
	log.Println("Press Ctrl-C to close")
	WaitAndStopObservees(nil, []Observee{
		server,
		&NoopObservee{ExternalInterrupt()},
	})
}
