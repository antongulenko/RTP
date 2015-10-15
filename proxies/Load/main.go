package main

import (
	"flag"
	"log"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/protocols/amp_control"
	"github.com/antongulenko/RTP/protocols/heartbeat"
	"github.com/antongulenko/RTP/protocols/ping"
)

func printErrors(server *protocols.Server) {
	for err := range server.Errors() {
		log.Println("Server error: " + err.Error())
	}
}

func main() {
	payloadSize := flag.Uint("payload", 0, "Additional payload to append to Load packets")
	amp_addr := protocols.ParseServerFlags("0.0.0.0", 7770)

	proto, err := protocols.NewProtocol("AMP/Load", amp.Protocol, amp_control.Protocol, ping.Protocol, heartbeat.Protocol)
	Checkerr(err)
	server, err := protocols.NewServer(amp_addr, proto)
	Checkerr(err)
	loadServer, err := RegisterLoadServer(server)
	Checkerr(err)
	loadServer.PayloadSize = *payloadSize

	go printErrors(server)
	server.Start()

	log.Println("Listening:", server)
	log.Println("Press Ctrl-C to close")
	NewObserveeGroup(
		server,
		&NoopObservee{ExternalInterrupt(), "external interrupt"},
	).WaitAndStop(nil)
}
