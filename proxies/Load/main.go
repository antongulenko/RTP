package main

import (
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
	amp_addr := protocols.ParseServerFlags("0.0.0.0", 7770)

	proto, err := protocols.NewProtocol("AMP/Load", amp.Protocol, amp_control.Protocol, ping.Protocol, heartbeat.Protocol)
	Checkerr(err)
	server, err := protocols.NewServer(amp_addr, proto)
	Checkerr(err)
	err = RegisterLoadServer(server)
	Checkerr(err)

	go printErrors(server)
	server.Start()

	log.Println("Listening:", server)
	log.Println("Press Ctrl-C to close")
	NewObserveeGroup(
		server,
		&NoopObservee{ExternalInterrupt(), "external interrupt"},
	).WaitAndStop(nil)
}
