package main

import (
	"log"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/proxies"
)

const (
	amp_addr = "127.0.0.1:7779"
)

func printAmpErrors(balancer *proxies.AmpBalancer) {
	for err := range balancer.Errors() {
		log.Println("AMP error: " + err.Error())
	}
}

func main() {
	balancer, err := proxies.NewAmpBalancer(amp_addr)
	Checkerr(err)

	go printAmpErrors(balancer)
	err = balancer.AddMediaServer("127.0.0.1:7777")
	Checkerr(err)
	balancer.Start()

	log.Println("Listening to AMP on " + amp_addr)
	log.Println("Press Ctrl-C to close")
	WaitAndStopObservees(nil, []Observee{
		balancer,
		&NoopObservee{ExternalInterrupt()},
	})
}
