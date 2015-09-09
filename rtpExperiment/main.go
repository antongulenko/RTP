package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/protocols/pcp"
	"github.com/antongulenko/RTP/proxies"
	"github.com/antongulenko/RTP/rtpClient"
	"github.com/antongulenko/RTP/stats"
	"github.com/antongulenko/gortp"
)

var (
	statistics     []*stats.Stats
	stopConditions []<-chan interface{}
)

const (
	print_stats       = true
	running_average   = true
	print_ctrl_events = false

	use_pcp    = true
	pcp_local  = "127.0.0.1:0"
	pcp_server = "127.0.0.1:7778"

	use_amp        = true
	amp_local      = "127.0.0.1:0"
	amp_server     = "127.0.0.1:7777"
	amp_media_file = "Sample.264"

	use_proxy = false

	rtp_ip         = "127.0.0.1"
	start_rtp_port = 9000
	proxy_port     = 9500
	rtsp_url       = "rtsp://127.0.1.1:8554/Sample.264"
)

func open(local string, server string) (conn *net.UDPConn, serverAddr *net.UDPAddr) {
	addr, err := net.ResolveUDPAddr("udp", local)
	Checkerr(err)
	serverAddr, err = net.ResolveUDPAddr("udp", server)
	Checkerr(err)
	conn, err = net.ListenUDP("udp", addr)
	Checkerr(err)
	return
}

func send(conn *net.UDPConn, addr *net.UDPAddr, packet protocols.IPacket) {
	conn.SetDeadline(time.Now().Add(1 * time.Second))
	reply, err := packet.SendRequest(conn, addr)
	Checkerr(err)
	if reply.IsError() {
		log.Fatalf("Protocol error: %v\n", reply.Error())
	} else if !reply.IsOK() {
		log.Fatalf("Protocol reply code %v: %v\n", reply.Code, reply.Val)
	}
}

func startStream(client *rtpClient.RtpClient, rtp_port int, stopConditions []<-chan interface{}) {
	if use_amp {
		conn, addr := open(amp_local, amp_server)
		packet := amp.NewPacket(amp.CodeStartSession, amp.StartSessionValue{
			MediaFile:    amp_media_file,
			ReceiverHost: rtp_ip,
			Port:         rtp_port,
		})
		send(conn, addr, packet)

		defer func() {
			packet := amp.NewPacket(amp.CodeStopSession, amp.StopSessionValue{
				ReceiverHost: rtp_ip,
				Port:         rtp_port,
			})
			send(conn, addr, packet)
			conn.Close()
		}()
	} else {
		rtsp, err := client.RequestRtsp(rtsp_url, rtp_port, "main.log")
		Checkerr(err)
		defer rtsp.Stop()
		stopConditions = append(stopConditions, client.ObserveRtsp(rtsp))
	}

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

func proxyAddrs(listenPort, targetPort int) (listen, target string) {
	listen = net.JoinHostPort(rtp_ip, strconv.Itoa(listenPort))
	target = net.JoinHostPort(rtp_ip, strconv.Itoa(targetPort))
	return
}

func makeProxy(listenPort, targetPort int) *proxies.UdpProxy {
	listen, target := proxyAddrs(listenPort, targetPort)
	p, err := proxies.NewUdpProxy(listen, target)
	Checkerr(err)
	p.Start()
	log.Printf("UDP proxy started: %s\n", p)
	return p
}

func closeProxy(proxy *proxies.UdpProxy, port int) {
	proxy.Close()
	if proxy.Err != nil {
		log.Printf("Proxy %v error: %v\n", port, proxy.Err)
	}
}

func makeProxyPCP(listenPort, targetPort int, conn *net.UDPConn, addr *net.UDPAddr) {
	listen, target := proxyAddrs(listenPort, targetPort)
	packet := pcp.NewPacket(pcp.CodeStartProxySession, pcp.StartProxySession{
		ListenAddr: listen,
		TargetAddr: target,
	})
	send(conn, addr, packet)
}

func closeProxyPCP(listenPort, targetPort int, conn *net.UDPConn, addr *net.UDPAddr) {
	listen, target := proxyAddrs(listenPort, targetPort)
	packet := pcp.NewPacket(pcp.CodeStopProxySession, pcp.StopProxySession{
		ListenAddr: listen,
		TargetAddr: target,
	})
	send(conn, addr, packet)
}

func startClient() (client *rtpClient.RtpClient, rtp_port int) {
	rtp_port = start_rtp_port
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

func startProxies(rtp_port int) {
	if use_pcp {
		conn, server := open(pcp_local, pcp_server)
		makeProxyPCP(proxy_port, rtp_port, conn, server)
		makeProxyPCP(proxy_port+1, rtp_port+1, conn, server)

		defer func() {
			closeProxyPCP(proxy_port, rtp_port, conn, server)
			closeProxyPCP(proxy_port+1, rtp_port+1, conn, server)
			conn.Close()
		}()
	} else {
		proxy1 := makeProxy(proxy_port, rtp_port)
		proxy2 := makeProxy(proxy_port+1, rtp_port+1)
		statistics = append(statistics, proxy1.Stats, proxy2.Stats)
		stopConditions = append(stopConditions, proxy1.ProxyClosed(), proxy2.ProxyClosed())
		defer func() {
			closeProxy(proxy1, rtp_port)
			closeProxy(proxy2, rtp_port+1)
		}()
	}
}

func main() {
	client, rtp_port := startClient()
	if use_proxy {
		startProxies(rtp_port)
	}
	startStream(client, rtp_port, stopConditions)
}
