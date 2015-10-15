package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/protocols/load"
	"github.com/antongulenko/RTP/rtpClient"
	"github.com/antongulenko/RTP/stats"
	"github.com/antongulenko/gortp"
)

var (
	statistics []*stats.Stats
	observees  = NewObserveeGroup()
)

var (
	num_clients = 1

	close_int   = true
	close_stdin = false

	show_dropped_packets_stats = false // Packets dropped inside the gortp stack. Enable when something seems off.
	print_stats                = true
	running_average            = true
	print_ctrl_events          = false

	use_amp        = false
	amp_url        = "127.0.0.1:7779"
	amp_media_file = "Sample.264"

	use_proxy     = false
	proxy_port    = 10000
	pretend_proxy = false
	use_pcp       = false
	pcp_url       = "127.0.0.1:7778"

	use_rtsp = false
	rtsp_url = "rtsp://127.0.1.1:8554/Sample.264"

	use_load = false

	rtp_ip         = "127.0.0.1"
	start_rtp_port = 9000
)

func startRtpClient() (rtp_port int) {
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
	observees.AddNamed("client", client)
	log.Printf("Listening on %v UDP ports %v and %v for rtp/rtcp\n", rtp_ip, rtp_port, rtp_port+1)

	if print_ctrl_events {
		client.CtrlHandler = func(evt *rtp.CtrlEvent) {
			fmt.Println(rtpClient.CtrlEventString(evt))
		}
	}

	statistics = append(statistics, client.ReceiveStats)
	statistics = append(statistics, client.MissedStats)
	statistics = append(statistics, client.CtrlStats)
	if show_dropped_packets_stats {
		statistics = append(statistics, client.RtpSession.DroppedDataPackets)
		statistics = append(statistics, client.RtpSession.DroppedCtrlPackets)
	}
	return
}

func startLoadClient() (port int) {
	port = start_rtp_port
	var server *protocols.Server
	var stats *load.LoadStats
	for {
		var err error
		server, err = protocols.NewServer(net.JoinHostPort(rtp_ip, strconv.Itoa(port)), load.MiniProtocol)
		if err != nil {
			log.Printf("Failed to start Load server on port %v: %v", port, err)
			port += 2
			continue
		}
		stats, err = load.RegisterServer(server)
		if err != nil {
			log.Printf("Failed to register Load server on port %v: %v", port, err)
			port += 2
			continue
		}
		break
	}
	observees.AddNamed("client", server)
	server.Start()
	log.Printf("Listening on %v UDP port %v for Load\n", rtp_ip, port)

	statistics = append(statistics, stats.Received)
	statistics = append(statistics, stats.Missed)
	return
}

func startStream(target_ip string, rtp_port int) {
	if use_amp {
		log.Println("Starting stream using AMP at", amp_url)
		client, err := amp.NewClientFor(amp_url)
		Checkerr(err)
		Checkerr(client.StartStream(target_ip, rtp_port, amp_media_file))
		observees.AddNamed("stream", CleanupObservee(func() {
			Printerr(client.StopStream(target_ip, rtp_port))
			Printerr(client.Close())
		}))
	}
	if use_rtsp {
		if target_ip != rtp_ip {
			log.Printf("Warning: RTSP server will stream media to %v, but we are expecting it on %v\n", rtp_ip, target_ip)
		}
		log.Println("Starting stream using RTSP at", rtsp_url)
		rtspCommand, err := rtpClient.StartRtspClient(rtsp_url, rtp_port, "main.log")
		Checkerr(err)
		observees.AddNamed("rtsp", rtspCommand)
	}
}

func stopObservees() {
	observees.ReverseStop()
}

func parseFlags() {
	flag.IntVar(&num_clients, "num", num_clients,
		"Number of parallel RTP streams to initiate.\n"+
			"proxy_port and rtp_port will be used as starting point for allocating the required number of ports.")

	flag.BoolVar(&close_stdin, "stdin", close_stdin, "Exit when stdin is closed")
	flag.BoolVar(&close_int, "int", close_int, "Exit when INT signal is received")

	flag.BoolVar(&show_dropped_packets_stats, "dropped_packet_stats", show_dropped_packets_stats, "Show numbers of packets dropped within the gortp stack")
	flag.BoolVar(&print_stats, "stats", print_stats, "Show various statistics about traffic")
	flag.BoolVar(&running_average, "average", running_average, "Show statistics averaged over the last 3 seconds")
	flag.BoolVar(&print_ctrl_events, "rtcp", print_ctrl_events, "Print RTCP events")

	flag.BoolVar(&use_rtsp, "rtsp", use_rtsp, "Initiate an RTSP session from the URL given by -rtsp_url")
	flag.StringVar(&rtsp_url, "rtsp_url", rtsp_url, "Set the URL used if -rtsp is given")

	flag.BoolVar(&use_amp, "amp", use_amp, "Initiate an AMP session at the server given by -amp_url")
	flag.StringVar(&amp_url, "amp_url", amp_url, "The AMP server used if -amp is given")
	flag.StringVar(&amp_media_file, "amp_file", amp_media_file, "The media file used with -amp")

	flag.BoolVar(&use_proxy, "proxy", use_proxy, "Route the RTP traffic through a proxy")
	flag.IntVar(&proxy_port, "proxy_port", proxy_port, "With -proxy, the port to receive traffic and forward it to -rtp_port")
	flag.BoolVar(&pretend_proxy, "pretend_proxy", pretend_proxy, "Don't really use proxies, but listen on the port that should receive the forwarded traffic. Implies -proxy.")
	flag.BoolVar(&use_pcp, "pcp", use_pcp, "Use external PCP server to satisfy -proxy. Implies -proxy")
	flag.StringVar(&pcp_url, "pcp_url", pcp_url, "The PCP server used for -pcp")

	flag.StringVar(&rtp_ip, "rtp", rtp_ip, "The local IP used to receive RTP/RTCP traffic")
	flag.IntVar(&start_rtp_port, "rtp_port", start_rtp_port, "The local port to receive RTP traffic")

	flag.BoolVar(&use_load, "load", use_load, "Listen for Load traffic instead of RTP/RTCP traffic")

	flag.Parse()

	use_proxy = use_proxy || use_pcp
	use_proxy = use_proxy || pretend_proxy
	if use_load && use_rtsp {
		Checkerr(fmt.Errorf("-load cannot be used with -rtsp"))
	}
}

func startScenario() {
	var client_port int
	if use_load {
		client_port = startLoadClient()
	} else {
		client_port = startRtpClient()
	}
	start_rtp_port = client_port + 2
	stream_ip := rtp_ip
	if use_proxy {
		stream_ip, client_port = startProxies(client_port)
		proxy_port += 2
	}
	startStream(stream_ip, client_port)
}

func printStatistics() {
	agg := make(stats.AggregatedStats, 0, 10)
	for _, stats := range statistics {
		agg.Aggregate(stats)
	}
	if running_average {
		agg.Start()
	}
	observees.AddNamed("stats",
		LoopObservee(func() {
			agg.Flush(3)
			fmt.Printf("==============\n%s", agg.String())
			time.Sleep(time.Second)
		}))
}

func main() {
	parseFlags()
	ExitHook = stopObservees

	for i := 0; i < num_clients; i++ {
		startScenario()
	}
	if print_stats {
		printStatistics()
	}

	if close_stdin {
		log.Println("Press Ctrl-D to interrupt")
		observees.Add(&NoopObservee{StdinClosed(), "stdin closed"})
	}
	if close_int {
		log.Println("Press Ctrl-C to interrupt")
		observees.Add(&NoopObservee{ExternalInterrupt(), "external interrupt"})
	}

	choice := observees.WaitForAny(nil)
	log.Printf("Stopped because of %T: %v\n", choice, choice)
	stopObservees()
}
