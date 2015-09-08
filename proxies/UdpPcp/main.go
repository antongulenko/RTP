package main

// Handle PCP requests. Set up and manage UDP proxies accordingly.

import (
	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/proxies"
)

const (
	local_addr = "127.0.0.1:7777"
)

func main() {
	udpAddr, err := net.ResolveUDPAddr("udp", local_addr)
	Checkerr(err)
	listenConn, err := net.ListenUDP("udp", udpAddr)
	Checkerr(err)
}
