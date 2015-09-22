package proxies

// Converts AMP to RTSP

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/rtpClient"
)

const (
	minPort = 20000
	maxPort = 50000
)

type AmpProxy struct {
	*amp.Server
	sessions protocols.Sessions

	rtspURL   *url.URL
	proxyHost string

	StreamStartedCallback func(rtsp *helpers.Command, proxies []*UdpProxy)
	StreamStoppedCallback func(rtsp *helpers.Command, proxies []*UdpProxy)
}

type streamSession struct {
	*protocols.SessionBase

	backend   *helpers.Command
	rtpProxy  *UdpProxy
	rtcpProxy *UdpProxy
	port      int
	mediaFile string
	client    string
	proxy     *AmpProxy
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
		sessions:  make(protocols.Sessions),
	}
	proxy.Server, err = amp.NewServer(local_addr, proxy)
	if err != nil {
		proxy = nil
	}
	return
}

func (proxy *AmpProxy) StopServer() {
	proxy.sessions.StopSessions()
}

func (proxy *AmpProxy) StartStream(desc *amp.StartStream) error {
	client := desc.Client()
	if _, ok := proxy.sessions[client]; ok {
		return fmt.Errorf("Session already exists for client %v", client)
	}

	session, err := proxy.newStreamSession(desc)
	if err != nil {
		return err
	}
	session.SessionBase = proxy.sessions.NewSession(client, session)
	return nil
}

func (proxy *AmpProxy) StopStream(desc *amp.StopStream) error {
	return proxy.sessions.StopSession(desc.Client())
}

func (proxy *AmpProxy) emergencyStopSession(client string, err error) error {
	stopErr := proxy.sessions.StopSession(client)
	if stopErr == nil {
		return fmt.Errorf("Error redirecting session: %v. Session for %v was stopped.", err, client)
	} else {
		return fmt.Errorf("Error redirecting session: %v. Error stopping session for %v: %v", err, client, stopErr)
	}
}

func (proxy *AmpProxy) RedirectStream(desc *amp.RedirectStream) error {
	oldClient := desc.OldClient.Client()
	newClient := desc.NewClient.Client()
	sessionBase, err := proxy.sessions.ReKeySession(oldClient, newClient)
	if err != nil {
		return err
	}
	session, ok := sessionBase.Session.(*streamSession)
	if !ok {
		return proxy.emergencyStopSession(newClient, // Should never happen
			fmt.Errorf("Illegal session type %T: %v", sessionBase, sessionBase))
	}

	err = session.rtpProxy.RedirectOutput(newClient)
	if err != nil {
		return proxy.emergencyStopSession(newClient, err)
	}
	newRtcpClient := net.JoinHostPort(desc.NewClient.ReceiverHost, strconv.Itoa(desc.NewClient.Port+1))
	err = session.rtcpProxy.RedirectOutput(newRtcpClient)
	if err != nil {
		return proxy.emergencyStopSession(newClient, err)
	}
	return nil
}

func (proxy *AmpProxy) newStreamSession(desc *amp.StartStream) (*streamSession, error) {
	client := desc.Client()
	rtcpClient := net.JoinHostPort(desc.ReceiverHost, strconv.Itoa(desc.Port+1))
	rtpProxy, rtcpProxy, err := NewUdpProxyPair(proxy.proxyHost, client, rtcpClient, minPort, maxPort)
	if err != nil {
		return nil, err
	}
	rtpPort := rtpProxy.listenAddr.Port

	mediaURL := proxy.rtspURL.ResolveReference(&url.URL{Path: desc.MediaFile})
	logfile := fmt.Sprintf("amp-proxy-%v-%v.log", rtpPort, desc.MediaFile)
	rtsp, err := rtpClient.StartRtspClient(mediaURL.String(), rtpPort, logfile)
	if err != nil {
		rtpProxy.Stop()
		rtcpProxy.Stop()
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
	}, nil
}

func (session *streamSession) proxies() []*UdpProxy {
	return []*UdpProxy{session.rtpProxy, session.rtcpProxy}
}

func (session *streamSession) Observees() []helpers.Observee {
	return []helpers.Observee{
		session.rtpProxy,
		session.rtcpProxy,
		session.backend,
	}
}

func (session *streamSession) Start() {
	session.rtpProxy.Start()
	session.rtcpProxy.Start()
	if session.proxy.StreamStartedCallback != nil {
		session.proxy.StreamStartedCallback(session.backend, session.proxies())
	}
}

func (session *streamSession) Cleanup() {
	for _, p := range session.proxies() {
		if p.Err != nil {
			session.proxy.LogError(fmt.Errorf("Proxy %s error: %v", p, p.Err))
			session.CleanupErr = p.Err
		}
	}
	if !session.backend.Success() {
		session.CleanupErr = errors.New(session.backend.StateString())
	}
	if session.proxy.StreamStoppedCallback != nil {
		session.proxy.StreamStoppedCallback(session.backend, session.proxies())
	}
}
