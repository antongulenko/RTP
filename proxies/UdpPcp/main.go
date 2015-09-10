package main

// Handle PCP requests. Set up and manage UDP proxies accordingly.

import (
	"log"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/proxies"
)

const (
	local_addr = "127.0.0.1:7778"
)

func printPcpErrors(proxy *proxies.PcpProxy) {
	for err := range proxy.Errors() {
		log.Println("PCP error: " + err.Error())
	}
}

func proxyStarted(proxy *proxies.UdpProxy) {
	log.Println("Started proxy: " + proxy.String())
}

func proxyStopped(proxy *proxies.UdpProxy) {
	log.Println("Stopped proxy " + proxy.String())
}

func main() {
	proxy, err := proxies.NewPcpProxy(local_addr)
	Checkerr(err)

	go printPcpErrors(proxy)
	proxy.ProxyStartedCallback = proxyStarted
	proxy.ProxyStoppedCallback = proxyStopped
	proxy.Start()

	log.Println("Listening to PCP on " + local_addr)
	log.Println("Press Ctrl-C to close")
	WaitAndStopObservees(nil, []Observee{
		proxy,
		&NoopObservee{ExternalInterrupt()},
	})
}
