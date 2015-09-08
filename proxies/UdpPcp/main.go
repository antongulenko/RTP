package main

// Handle PCP requests. Set up and manage UDP proxies accordingly.

import (
	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/proxies"
)
import (
	"log"
)

const (
	local_addr = "127.0.0.1:7778"
)

func printPcpErrors(proxy *proxies.PcpProxy) {
	for err := range proxy.Errors() {
		log.Println("PCP error: " + err.Error())
	}
}

func main() {
	proxy, err := proxies.NewPcpProxy(local_addr)
	Checkerr(err)

	go printPcpErrors(proxy)
	proxy.Start()

	log.Println("Press Ctrl-C to interrupt")
	<-ExternalInterrupt()

	proxy.Stop()
}
