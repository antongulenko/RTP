package main

import (
	"log"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/proxies"
)

const (
	rtsp_url       = "rtsp://127.0.1.1:8554"
	local_media_ip = "127.0.0.1"
)

func printAmpErrors(proxy *proxies.AmpProxy) {
	for err := range proxy.Errors() {
		log.Println("AMP error: " + err.Error())
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
	amp_addr := protocols.ParseCommandlineFlags("0.0.0.0", 7777)
	proxy, err := proxies.NewAmpProxy(amp_addr, rtsp_url, local_media_ip)
	Checkerr(err)

	go printAmpErrors(proxy)
	proxy.StreamStartedCallback = printRtspStart
	proxy.StreamStoppedCallback = printRtspStop
	proxy.Start()

	log.Println("Listening to AMP on " + amp_addr + ", backend URL: " + rtsp_url)
	log.Println("Press Ctrl-D to close")
	WaitAndStopObservees(nil, []Observee{
		proxy,
		&NoopObservee{StdinClosed()},
	})
}
