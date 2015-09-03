package main

import (
	"fmt"
	"net"
)

func Aeasy() {
	con, err := net.ListenPacket("udp", "127.0.0.1:10000")
	checkerr(err)
	defer con.Close()
	fmt.Println("Listening on 127.0.0.1:10000")
	targetAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8554")
	checkerr(err)
	fmt.Println("Connected to target 127.0.0.1:8554")
	target, err := net.DialUDP("udp", nil, targetAddr)
	checkerr(err)
	defer target.Close()
	for {
		buf := make([]byte, 4096)
		n, addr, err := con.ReadFrom(buf)
		checkerr(err)
		fmt.Printf("======================== Forwarding %v bytes from %v\n", n, addr)
		fmt.Printf("%s\n", buf[:n])
		//sent, err := target.WriteToUDP(buf, targetAddr)
		//checkerr(err)
		//fmt.Printf("Transmitted %v bytes to %v\n\n", sent, targetAddr)
	}
}
