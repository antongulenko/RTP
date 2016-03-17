package main

import (
	"flag"
	"log"
	"net"
	"time"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/golib"
)

const (
	default_target = "127.0.0.1:6061"
)

func main() {
	target_addr := flag.String("target", default_target, "The partner to exchange latency measurement packets with")
	local_addr := protocols.ParseServerFlags("0.0.0.0", 6060)
	local_host, _, err := net.SplitHostPort(local_addr)
	golib.Checkerr(err)

	server, err := NewServer(local_addr)
	golib.Checkerr(err)
	client, err := NewClientFor(local_host)
	golib.Checkerr(err)
	err = client.SetServer(*target_addr)
	golib.Checkerr(err)

	measureLatency := golib.LoopTask(func(stop golib.StopChan) {
		err := client.SendMeasureLatency()
		if err != nil {
			log.Println("Error sending Latency packet:", err)
		}
		select {
		case <-time.After(1 * time.Second):
		case <-stop:
		}
	})

	log.Println("Listening to Latency on " + local_addr + ", sending to: " + *target_addr)
	log.Println("Press Ctrl-C to close")
	golib.NewTaskGroup(
		server, measureLatency,
		&golib.NoopTask{golib.ExternalInterrupt(), "external interrupt"},
	).PrintWaitAndStop()
}
