package main

import (
	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/proxies"
	"github.com/antongulenko/RTP/rtpClient"
	"github.com/antongulenko/RTP/stats"
	"github.com/antongulenko/gortp"
)
import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

var statistics []*stats.Stats

const (
	print_stats       = true
	running_average   = true
	print_ctrl_events = false

	use_amp        = true
	amp_local      = "127.0.0.1:0"
	amp_server     = "127.0.0.1:7777"
	amp_media_file = "Sample.264"

	rtp_ip     = "127.0.0.1"
	rtp_port   = 9000
	proxy_port = 9500
	rtsp_url   = "rtsp://127.0.1.1:8554/Sample.264"
)

func sendAMP(conn *net.UDPConn, addr *net.UDPAddr, packet *amp.AmpPacket) {
	conn.SetDeadline(time.Now().Add(1 * time.Second))
	reply, err := packet.SendAmpRequest(conn, addr)
	Checkerr(err)
	if reply.IsError() {
		log.Fatalf("AMP error: %v\n", reply.Error())
	} else if !reply.IsOK() {
		log.Fatalf("AMP reply code %v: %v\n", reply.Code, reply.Val)
	}
}

func doRunClient(dataPort int, stopConditions []<-chan interface{}) {
	log.Printf("Listening on %v UDP ports %v and %v for rtp/rtcp\n", rtp_ip, rtp_port, rtp_port+1)
	client, err := rtpClient.NewRtpClient(rtp_ip, rtp_port)
	Checkerr(err)

	if print_ctrl_events {
		client.CtrlHandler = func(evt *rtp.CtrlEvent) {
			fmt.Println(rtpClient.CtrlEventString(evt))
		}
	}

	if use_amp {
		ampAddr, err := net.ResolveUDPAddr("udp", amp_local)
		Checkerr(err)
		ampServerAddr, err := net.ResolveUDPAddr("udp", amp_server)
		Checkerr(err)
		ampConn, err := net.ListenUDP("udp", ampAddr)
		Checkerr(err)

		packet := amp.NewPacket(amp.CodeStartSession, amp.StartSessionValue{
			MediaFile: amp_media_file,
			Port:      dataPort,
		})
		sendAMP(ampConn, ampServerAddr, packet)

		defer func() {
			packet := amp.NewPacket(amp.CodeStopSession, amp.StopSessionValue{
				MediaFile: amp_media_file,
				Port:      dataPort,
			})
			sendAMP(ampConn, ampServerAddr, packet)
			ampConn.Close()
		}()
	} else {
		rtsp, err := client.RequestRtsp(rtsp_url, dataPort, "main.log")
		Checkerr(err)
		defer rtsp.Stop()
		stopConditions = append(stopConditions, client.ObserveRtsp(rtsp))
	}

	statistics = append(statistics, client.ReceiveStats)
	statistics = append(statistics, client.MissedStats)
	statistics = append(statistics, client.CtrlStats)
	statistics = append(statistics, client.RtpSession.DroppedDataPackets)
	statistics = append(statistics, client.RtpSession.DroppedCtrlPackets)
	if print_stats {
		if running_average {
			for _, s := range statistics {
				s.Start()
			}
		}
		go stats.LoopPrintStats(1, 3, statistics)
	}

	log.Println("Press Ctrl-C to interrupt")
	stopConditions = append(stopConditions, ExternalInterrupt())
	choice := WaitForAny(stopConditions)

	log.Println("Stopping because of", choice)
	client.Stop()
}

func makeProxy(listenPort, targetPort int) *proxies.UdpProxy {
	listenUDP := net.JoinHostPort(rtp_ip, strconv.Itoa(listenPort))
	targetUDP := net.JoinHostPort(rtp_ip, strconv.Itoa(targetPort))
	p, err := proxies.NewUdpProxy(listenUDP, targetUDP)
	Checkerr(err)
	p.Start()
	log.Printf("UDP proxy started from %v to %v\n", listenUDP, targetUDP)
	return p
}

func closeProxy(proxy *proxies.UdpProxy, port int) {
	proxy.Close()
	if proxy.Err != nil {
		log.Printf("Proxy %v error: %v\n", port, proxy.Err)
	}
}

func runClient() {
	doRunClient(rtp_port, nil)
}

func runClientWithProxies() {
	proxy1 := makeProxy(proxy_port, rtp_port)
	proxy2 := makeProxy(proxy_port+1, rtp_port+1)
	statistics = append(statistics, proxy1.Stats)
	statistics = append(statistics, proxy2.Stats)

	doRunClient(proxy_port, []<-chan interface{}{
		proxy1.ProxyClosed(),
		proxy2.ProxyClosed(),
	})

	closeProxy(proxy1, rtp_port)
	closeProxy(proxy2, rtp_port+1)
}

func main() {
	use_proxy := false
	if use_proxy {
		runClientWithProxies()
	} else {
		runClient()
	}
}
