package main

import (
	"log"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/proxies"
	"github.com/antongulenko/RTP/rtpClient"
)

const (
	rtsp_url = "rtsp://127.0.1.1:8554"
	amp_addr = "127.0.0.1:7777"
	local_ip = "127.0.0.1"
)

func printAmpErrors(proxy *proxies.AmpProxy) {
	for err := range proxy.Errors() {
		log.Println("AMP error: " + err.Error())
	}
}

func printRtspStart(rtsp *rtpClient.RtspClient) {
	log.Printf("RTSP subprocess started (%v). Logfile: %v\n", rtsp.Proc.Pid, rtsp.Logfile)
}

func printRtspStop(rtsp *rtpClient.RtspClient) {
	log.Printf("RTSP subprocess: %v. Logfile: %v\n", rtsp.StateString, rtsp.Logfile)
}

func main() {
	proxy, err := proxies.NewAmpProxy(amp_addr, rtsp_url, local_ip)
	Checkerr(err)

	// Set up error reporting
	go printAmpErrors(proxy)
	proxy.RtspStartedCallback = printRtspStart
	proxy.RtspEndedCallback = printRtspStop

	proxy.Start()

	log.Println("Listening to AMP on " + amp_addr + ", backend URL: " + rtsp_url)
	log.Println("Press Ctrl-D to close")
	<-StdinClosed()

	proxy.Stop()
}
