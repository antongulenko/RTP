package main

import (
	"log"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/protocols/amp_control"
	"github.com/antongulenko/RTP/protocols/heartbeat"
	"github.com/antongulenko/RTP/protocols/ping"
	"github.com/antongulenko/RTP/proxies"
)

const (
	rtsp_url       = "rtsp://127.0.0.1:8554"
	local_media_ip = "127.0.0.1"
)

func printAmpErrors(proxy *proxies.AmpProxy) {
	for err := range proxy.Errors() {
		log.Println("Server error: " + err.Error())
	}
}

func printRtspStart(rtsp *Command, px []*proxies.UdpProxy) {
	log.Printf("Session started. RTSP pid %v, logfile: %v\n", rtsp.Proc.Pid, rtsp.Logfile)
	log.Println("\t\tProxies started:", px)
}

func printRtspStop(rtsp *Command, px []*proxies.UdpProxy) {
	log.Printf("Session stopped. RTSP: %s (logfile: %v)\n", rtsp.StateString(), rtsp.Logfile)
	log.Println("\t\tProxies stopped:", px)
}

func main() {
	proxies.UdpProxyFlags()
	amp_addr := protocols.ParseServerFlags("0.0.0.0", 7777)

	proto, err := protocols.NewProtocol("AMP", amp.Protocol, amp_control.Protocol, ping.Protocol, heartbeat.Protocol)
	Checkerr(err)
	server, err := protocols.NewServer(amp_addr, proto)
	Checkerr(err)
	proxy, err := proxies.RegisterAmpProxy(server, rtsp_url, local_media_ip)
	Checkerr(err)

	go printAmpErrors(proxy)
	proxy.StreamStartedCallback = printRtspStart
	proxy.StreamStoppedCallback = printRtspStop
	server.Start()

	log.Println("Listening:", server, "Backend URL:", rtsp_url)
	log.Println("Press Ctrl-D to close")
	NewObserveeGroup(
		server,
		&NoopObservee{StdinClosed(), "stdin closed"},
	).WaitAndStop(nil)
}
