package main

import (
	"flag"
	"log"
	"time"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
)

const (
	default_target = "127.0.0.1:6061"
)

func main() {
	target_addr := flag.String("target", default_target, "The partner to exchange latency measurement packets with")
	local_addr := protocols.ParseCommandlineFlags("0.0.0.0", 6060)

	server, err := NewServer(local_addr)
	Checkerr(err)
	client, err := NewClient("0.0.0.0")
	Checkerr(err)
	err = client.SetServer(*target_addr)
	Checkerr(err)

	server.Start()

	stopSending := NewOneshotCondition()
	go func() {
		for !stopSending.Enabled() {
			err := client.SendMeasureLatency()
			if err != nil {
				log.Println("Error sending Latency packet:", err)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	log.Println("Listening to Latency on " + local_addr + ", sending to: " + *target_addr)
	log.Println("Press Ctrl-D to close")
	WaitAndStopObservees(nil, []Observee{
		server, stopSending,
		&NoopObservee{StdinClosed()},
	})
}
