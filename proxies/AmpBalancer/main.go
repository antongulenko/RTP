package main

import (
	"log"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/proxies/amp_balancer"
)

const (
	amp_addr = "127.0.0.1:7779"
)

func printAmpErrors(balancer *amp_balancer.AmpBalancer) {
	for err := range balancer.Errors() {
		log.Println("AMP error: " + err.Error())
	}
}

func printSessionStarted(client string) {
	log.Println("Started session for", client)
}

func printSessionStopped(client string) {
	log.Println("Stopped session for", client)
}

func stateChangePrinter(breaker protocols.CircuitBreaker) {
	err, server := breaker.Error(), breaker.String()
	if err != nil {
		log.Printf("%v down: %v\n", server, err)
	} else {
		log.Printf("%s up\n", server)
	}
}

func main() {
	ampPlugin := amp_balancer.NewAmpBalancingPlugin()
	err := ampPlugin.AddBackendServer("127.0.0.1:7777", stateChangePrinter)
	Checkerr(err)
	pcpPlugin := amp_balancer.NewPcpBalancingPlugin()
	err = pcpPlugin.AddBackendServer("127.0.0.1:7778", stateChangePrinter)
	Checkerr(err)

	balancer, err := amp_balancer.NewAmpBalancer(amp_addr)
	Checkerr(err)
	balancer.AddPlugin(ampPlugin)
	balancer.AddPlugin(pcpPlugin)

	go printAmpErrors(balancer)
	balancer.SessionStartedCallback = printSessionStarted
	balancer.SessionStoppedCallback = printSessionStopped

	balancer.Start()

	log.Println("Listening to AMP on " + amp_addr)
	log.Println("Press Ctrl-C to close")
	WaitAndStopObservees(nil, []Observee{
		balancer,
		&NoopObservee{ExternalInterrupt()},
	})
}
