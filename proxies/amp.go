package proxies

// Converts AMP to RTSP

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/rtpClient"
)

const (
	minPort = 20000
	maxPort = 50000
)

type AmpProxy struct {
	*amp.Server

	rtspURL   *url.URL
	proxyHost string
	sessions  map[string]*streamSession

	StreamStartedCallback func(rtsp *helpers.Command, proxies []*UdpProxy)
	StreamStoppedCallback func(rtsp *helpers.Command, proxies []*UdpProxy)
}

type streamSession struct {
	backend   *helpers.Command
	rtpProxy  *UdpProxy
	rtcpProxy *UdpProxy
	port      int
	mediaFile string
	client    string
	proxy     *AmpProxy
	stopped   *helpers.OneshotCondition
}

// ampAddr: address to listen on for AMP requests
// rtspURL: base URL used when sending RTSP requests to the backend media server
// localProxyIP: address to receive RTP/RTCP packets from the media server
func NewAmpProxy(local_addr, rtspURL, localProxyIP string) (proxy *AmpProxy, err error) {
	u, err := url.Parse(rtspURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "rtsp" {
		return nil, errors.New("Need rtsp:// rtspURL for AmpProxy")
	}

	ip, err := net.ResolveIPAddr("ip", localProxyIP)
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve IP address %v: %v", localProxyIP, err)
	}

	proxy = &AmpProxy{
		rtspURL:   u,
		proxyHost: ip.String(),
		sessions:  make(map[string]*streamSession),
	}
	proxy.Server, err = amp.NewServer(local_addr, proxy)
	if err != nil {
		proxy = nil
	}
	return
}

func (proxy *AmpProxy) StopServer() {
	for _, session := range proxy.sessions {
		session.Stop()
	}
}

func (proxy *AmpProxy) StartStream(desc *amp.StartStream) error {
	client := desc.Client()
	_, ok := proxy.sessions[client]
	if ok {
		return fmt.Errorf("Session already exists for client %v", client)
	}

	session, err := proxy.newStreamSession(desc)
	if err != nil {
		return err
	}
	proxy.sessions[client] = session
	session.Start()
	return nil
}

func (proxy *AmpProxy) StopStream(desc *amp.StopStream) error {
	client := desc.Client()
	session, ok := proxy.sessions[client]
	if !ok {
		return fmt.Errorf("Session not found for client %v", client)
	}
	session.Stop()
	return nil
}

func (proxy *AmpProxy) newStreamSession(desc *amp.StartStream) (*streamSession, error) {
	client := desc.Client()
	rtcpClient := net.JoinHostPort(desc.ReceiverHost, strconv.Itoa(desc.Port+1))
	rtpProxy, rtcpProxy, err := NewUdpProxyPair(proxy.proxyHost, client, rtcpClient, minPort, maxPort)
	if err != nil {
		return nil, err
	}
	rtpProxy.Start()
	rtcpProxy.Start()
	rtpPort := rtpProxy.listenAddr.Port

	mediaURL := proxy.rtspURL.ResolveReference(&url.URL{Path: desc.MediaFile})
	logfile := fmt.Sprintf("amp-proxy-%v-%v.log", rtpPort, desc.MediaFile)
	rtsp, err := rtpClient.StartRtspClient(mediaURL.String(), rtpPort, logfile)
	if err != nil {
		return nil, fmt.Errorf("Failed to start RTSP client: %v", err)
	}
	return &streamSession{
		backend:   rtsp,
		mediaFile: desc.MediaFile,
		port:      desc.Port,
		rtpProxy:  rtpProxy,
		rtcpProxy: rtcpProxy,
		client:    client,
		proxy:     proxy,
		stopped:   helpers.NewOneshotCondition(),
	}, nil
}

func (session *streamSession) proxies() []*UdpProxy {
	return []*UdpProxy{session.rtpProxy, session.rtcpProxy}
}

func (session *streamSession) Start() {
	if session.proxy.StreamStartedCallback != nil {
		session.proxy.StreamStartedCallback(session.backend, session.proxies())
	}
	observees := []helpers.Observee{session.backend, session.rtpProxy, session.rtcpProxy}
	go func() {
		helpers.WaitForAnyObservee(session.proxy.Wg, observees)
		session.Stop()
	}()
}

func (session *streamSession) Stop() {
	session.stopped.Enable(func() {
		session.backend.Stop()
		for _, p := range session.proxies() {
			p.Stop()
			if p.Err != nil {
				session.proxy.LogError(fmt.Errorf("Proxy %s error: %v", p, p.Err))
			}
		}
		delete(session.proxy.sessions, session.client)
		if session.proxy.StreamStoppedCallback != nil {
			session.proxy.StreamStoppedCallback(session.backend, session.proxies())
		}
	})
}
