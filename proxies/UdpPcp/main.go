package main

// Handle PCP requests. Set up and manage UDP proxies accordingly.

import (
	"log"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/heartbeat"
	"github.com/antongulenko/RTP/protocols/pcp"
	"github.com/antongulenko/RTP/protocols/ping"
	"github.com/antongulenko/RTP/proxies"
	"github.com/antongulenko/golib"
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

	proto, err := protocols.NewProtocol("PCP", pcp.Protocol, ping.Protocol, heartbeat.Protocol)
	golib.Checkerr(err)
	server, err := protocols.NewServer(pcp_addr, proto)
	golib.Checkerr(err)
	proxy, err := proxies.RegisterPcpProxy(server)
	golib.Checkerr(err)

	go printPcpErrors(proxy)
	proxy.ProxyStartedCallback = proxyStarted
	proxy.ProxyStoppedCallback = proxyStopped

	log.Println("Listening:", server)
	log.Println("Press Ctrl-C to close")
	golib.NewTaskGroup(
		server,
		&golib.NoopTask{golib.ExternalInterrupt(), "external interrupt"},
	).WaitAndStop()
}
