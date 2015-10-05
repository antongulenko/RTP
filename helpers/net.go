package helpers

import (
	"fmt"
	"net"
)

func ResolveUdp(server string) (*net.UDPAddr, *net.UDPAddr, error) {
	server_addr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return nil, nil, err
	}
	conn, err := net.DialUDP("udp", nil, server_addr)
	if err != nil {
		return nil, nil, err
	}
	local_addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, nil, fmt.Errorf("Failed to convert to *net.UDPAddr: %v", conn.LocalAddr())
	}
	_ = conn.Close() // Drop error
	return server_addr, local_addr, nil
}
