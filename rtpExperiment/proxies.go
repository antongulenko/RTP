package main

import (
	"log"
	"net"
	"strconv"

	. "github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols/pcp"
	"github.com/antongulenko/RTP/proxies"
)

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

func makeProxyPCP(client *pcp.Client, listenPort, targetPort int) {
	Checkerr(client.StartProxy(proxyAddrs(listenPort, targetPort)))
}

func closeProxyPCP(client *pcp.Client, listenPort, targetPort int) {
	Printerr(client.StopProxy(proxyAddrs(listenPort, targetPort)))
}

func startProxies(rtp_port int) int {
	if pretend_proxies {
		return proxy_port
	}

	if use_pcp {
		client, err := pcp.NewClient(protocol_local)
		Checkerr(err)
		Checkerr(client.SetServer(pcp_server))
		makeProxyPCP(client, proxy_port, rtp_port)
		makeProxyPCP(client, proxy_port+1, rtp_port+1)
		observees = append(observees, CleanupObservee(func() {
			closeProxyPCP(client, proxy_port, rtp_port)
			closeProxyPCP(client, proxy_port+1, rtp_port+1)
			Printerr(client.Close())
		}))
	} else {
		proxy1 := makeProxy(proxy_port, rtp_port)
		proxy2 := makeProxy(proxy_port+1, rtp_port+1)
		statistics = append(statistics, proxy1.Stats, proxy2.Stats)
		observees = append(observees, proxy1, proxy2, CleanupObservee(func() {
			if proxy1.Err != nil {
				log.Printf("Proxy %v error: %v\n", proxy_port, proxy1.Err)
			}
			if proxy2.Err != nil {
				log.Printf("Proxy %v error: %v\n", proxy_port+1, proxy2.Err)
			}
		}))
	}
	return proxy_port
}
