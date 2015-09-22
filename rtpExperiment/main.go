package main

import (
	"fmt"
	"log"
	"time"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/rtpClient"
	"github.com/antongulenko/RTP/stats"
	"github.com/antongulenko/gortp"
)

var (
	statistics []*stats.Stats
	observees  []Observee
)

const (
	print_stats       = true
	running_average   = true
	print_ctrl_events = false

	protocol_local = "127.0.0.1"

	use_pcp    = true
	pcp_server = "127.0.0.1:7778"

	use_amp        = true
	amp_server     = "127.0.0.1:7777"
	amp_media_file = "Sample.264"

	use_balancer = true
	amp_balancer = "127.0.0.1:7779"

	use_proxy       = false
	pretend_proxies = false

	rtp_ip         = "127.0.0.1"
	start_rtp_port = 9000
	proxy_port     = 9500
	rtsp_url       = "rtsp://127.0.1.1:8554/Sample.264"
)

func startClient() (rtp_port int) {
	rtp_port = start_rtp_port
	var client *rtpClient.RtpClient
	for {
		var err error
		client, err = rtpClient.NewRtpClient(rtp_ip, rtp_port)
		if err != nil {
			log.Printf("Failed to start RTP client on port %v: %v", rtp_port, err)
			rtp_port += 2
		} else {
			break
		}
	}
	observees = append(observees, client)
	log.Printf("Listening on %v UDP ports %v and %v for rtp/rtcp\n", rtp_ip, rtp_port, rtp_port+1)

	if print_ctrl_events {
		client.CtrlHandler = func(evt *rtp.CtrlEvent) {
			fmt.Println(rtpClient.CtrlEventString(evt))
		}
	}

	statistics = append(statistics, client.ReceiveStats)
	statistics = append(statistics, client.MissedStats)
	statistics = append(statistics, client.CtrlStats)
	statistics = append(statistics, client.RtpSession.DroppedDataPackets)
	statistics = append(statistics, client.RtpSession.DroppedCtrlPackets)
	return
}

func startStream(rtp_port int) {
	if use_amp {
		server := amp_server
		if use_balancer {
			server = amp_balancer
		}
		client, err := amp.NewClient(protocol_local)
		Checkerr(err)
		Checkerr(client.SetServer(server))
		Checkerr(client.StartStream(rtp_ip, rtp_port, amp_media_file))
		observees = append(observees, CleanupObservee(func() {
			Printerr(client.StopStream(rtp_ip, rtp_port))
			Printerr(client.Close())
		}))
	} else {
		rtsp, err := rtpClient.StartRtspClient(rtsp_url, rtp_port, "main.log")
		Checkerr(err)
		observees = append(observees, rtsp)
	}
}

func stopObservees() {
	ReverseStopObservees(observees)
}

func main() {
	ExitHook = stopObservees
	rtp_port := startClient()
	if use_proxy {
		rtp_port = startProxies(rtp_port)
	}
	startStream(rtp_port)

	if print_stats {
		if running_average {
			for _, s := range statistics {
				s.Start()
			}
		}
		observees = append(observees, LoopObservee(func() {
			stats.PrintStats(3, statistics)
			time.Sleep(time.Second)
		}))
	}

	log.Println("Press Ctrl-C to interrupt")
	observees = append(observees, &NoopObservee{ExternalInterrupt()})
	choice := WaitForAnyObservee(nil, observees)
	log.Println("Stopped because of", choice)
	stopObservees()
}
