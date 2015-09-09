package main

import (
	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/proxies"
	"github.com/antongulenko/RTP/rtpClient"
)
import (
	"log"
)

const (
	rtsp_url   = "rtsp://127.0.1.1:8554"
	local_addr = "127.0.0.1:7777"
)

func printAmpErrors(proxy *proxies.AmpProxy) {
	for err := range proxy.Errors() {
		log.Println("AMP error: " + err.Error())
	}
}

func printRtspErrors(rtsp *rtpClient.RtspClient) {
	log.Printf("RTSP subprocess: %s. Logfile: %v\n", rtsp.StateString, rtsp.Logfile)
}

func main() {
	proxy, err := proxies.NewAmpProxy(local_addr, rtsp_url)
	Checkerr(err)

	// Set up error reporting
	go printAmpErrors(proxy)
	proxy.ProcessDiedCallback = printRtspErrors

	proxy.Start()

	log.Println("Press Ctrl+D to close")
	<-StdinClosed()

	proxy.Stop()
}
