package main

import (
	"github.com/antongulenko/RTP/MediaServerProxy"
	"github.com/antongulenko/RTP/PacketStats"
	"github.com/antongulenko/RTP/RtpClient"
	"log"
	"net"
	"os"
	"os/signal"
	"reflect"
	"strconv"
)

var statistics []*stats.Stats

const (
	print_stats = true
	rtp_ip      = "127.0.0.1"
	rtp_port    = 9000
	proxy_port  = 9500
	rtsp_url    = "rtsp://127.0.1.1:8554/Sample.264"
)

func checkerr(err error) {
	if err != nil {
		log.Fatalln("Error:", err)
	}
}

func waitForAny(channels []<-chan interface{}) int {
	// Use reflect package to wait for any of the given channels
	cases := make([]reflect.SelectCase, len(channels))
	for i, ch := range channels {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	choice, _, _ := reflect.Select(cases)
	return choice
}

// This must be done after starting the openRTSP subprocess.
// Otherwise openRTSP will not inherit the ignore-handler for
// the SIGINT signal provided by the external ./noint program.
func externalInterrupt() <-chan interface{} {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	stop := make(chan interface{})
	go func() {
		defer signal.Stop(interrupt)
		<-interrupt
		stop <- nil
	}()
	return stop
}

func doRunClient(dataPort int, stopConditions []<-chan interface{}) {
	client, err := rtpClient.NewRtpClient(rtp_ip, rtp_port)
	checkerr(err)

	err = client.Request(rtsp_url, dataPort)
	checkerr(err)

	statistics = append(statistics, client.ReceiveStats)
	statistics = append(statistics, client.MissedStats)
	statistics = append(statistics, client.CtrlStats)
	statistics = append(statistics, client.RtpSession.DroppedDataPackets)
	statistics = append(statistics, client.RtpSession.DroppedCtrlPackets)
	if print_stats {
		go stats.LoopPrintStats(1, 3, statistics)
	}

	log.Println("Press Ctrl-C to interrupt")
	stopConditions = append(stopConditions,
		externalInterrupt(),
		client.SubprocessDied())
	choice := waitForAny(stopConditions)

	log.Println("Stopping because of", choice)
	client.Stop()
}

func runClient() {
	doRunClient(rtp_port, nil)
}

func makeProxy(listenPort, targetPort int) *proxy.UdpProxy {
	listenUDP := net.JoinHostPort(rtp_ip, strconv.Itoa(listenPort))
	targetUDP := net.JoinHostPort(rtp_ip, strconv.Itoa(targetPort))
	p, err := proxy.NewUdpProxy(listenUDP, targetUDP)
	checkerr(err)
	p.Start()
	return p
}

func closeProxy(proxy *proxy.UdpProxy, port int) {
	proxy.Close()
	if proxy.Err != nil {
		log.Printf("Proxy %v error: %v\n", port, proxy.Err)
	}
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
	use_proxy := true
	if use_proxy {
		runClientWithProxies()
	} else {
		runClient()
	}
}
