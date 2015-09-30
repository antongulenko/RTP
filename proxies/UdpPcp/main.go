package main

// Handle PCP requests. Set up and manage UDP proxies accordingly.

import (
	"log"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/proxies"
)

func printPcpErrors(proxy *proxies.PcpProxy) {
	for err := range proxy.Errors() {
		log.Println("Server error: " + err.Error())
	}
}

func proxyStarted(proxy *proxies.UdpProxy) {
	log.Println("Started proxy: " + proxy.String())
}

func proxyStopped(proxy *proxies.UdpProxy) {
	log.Println("Stopped proxy " + proxy.String())
}

func main() {
	proxies.UdpProxyFlags()
	pcp_addr := protocols.ParseServerFlags("0.0.0.0", 7778)

	proxy, err := proxies.NewPcpProxy(pcp_addr)
	Checkerr(err)

	go printPcpErrors(proxy)
	proxy.ProxyStartedCallback = proxyStarted
	proxy.ProxyStoppedCallback = proxyStopped
	proxy.Start()

	log.Println("Listening to PCP on " + pcp_addr)
	log.Println("Press Ctrl-C to close")
	WaitAndStopObservees(nil, []Observee{
		proxy,
		&NoopObservee{ExternalInterrupt(), "external interrupt"},
	})
}
