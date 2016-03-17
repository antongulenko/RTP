package main

import (
	"log"
	"net"
	"strconv"
	"time"

	"github.com/antongulenko/RTP/protocols/pcp"
	"github.com/antongulenko/RTP/proxies"
	"github.com/antongulenko/golib"
)

func makeProxy(listenPort, targetPort int) *proxies.UdpProxy {
	// Local proxy: use same IP as for RTP traffic
	listen := net.JoinHostPort(rtp_ip, strconv.Itoa(listenPort))
	target := net.JoinHostPort(rtp_ip, strconv.Itoa(targetPort))
	p, err := proxies.NewUdpProxy(listen, target)
	golib.Checkerr(err)
	log.Printf("UDP proxy started: %s\n", p)
	return p
}

func pcpProxyIp() string {
	proxy_ip, _, err := net.SplitHostPort(pcp_url)
	golib.Checkerr(err)
	return proxy_ip
}

func pcpProxyAddrs(listenPort, targetPort int) (listen, target string) {
	listen = net.JoinHostPort(pcpProxyIp(), strconv.Itoa(listenPort))
	target = net.JoinHostPort(rtp_ip, strconv.Itoa(targetPort))
	return
}

func makeProxyPCP(client *pcp.Client, listenPort, targetPort int) {
	golib.Checkerr(client.StartProxy(pcpProxyAddrs(listenPort, targetPort)))
}

func closeProxyPCP(client *pcp.Client, listenPort, targetPort int) {
	golib.Printerr(client.StopProxy(pcpProxyAddrs(listenPort, targetPort)))
}

func startProxies(proxy_port, rtp_port int) (string, int) {
	proxy_ip := rtp_ip
	if !pretend_proxy {
		if use_pcp {
			proxy_ip = pcpProxyIp()
			client, err := pcp.NewClientFor(pcp_url)
			client.SetTimeout(time.Duration(client_timeout * float64(time.Second)))
			golib.Checkerr(err)
			log.Printf("Starting external proxies using %v\n", client)
			makeProxyPCP(client, proxy_port, rtp_port)
			makeProxyPCP(client, proxy_port+1, rtp_port+1)
			tasks.AddNamed("proxy", &golib.CleanupTask{Description: "stop proxies",
				Cleanup: func() {
					closeProxyPCP(client, proxy_port, rtp_port)
					closeProxyPCP(client, proxy_port+1, rtp_port+1)
					golib.Printerr(client.Close())
				}})
		} else {
			proxy1 := makeProxy(proxy_port, rtp_port)
			proxy2 := makeProxy(proxy_port+1, rtp_port+1)
			statistics = append(statistics, proxy1.Stats, proxy2.Stats)
			tasks.AddNamed("proxy", proxy1, proxy2, &golib.CleanupTask{
				Description: "print proxy errors",
				Cleanup: func() {
					if proxy1.Err != nil {
						log.Printf("Proxy %v error: %v\n", proxy_port, proxy1.Err)
					}
					if proxy2.Err != nil {
						log.Printf("Proxy %v error: %v\n", proxy_port+1, proxy2.Err)
					}
				}})
		}
	}
	return proxy_ip, proxy_port
}
