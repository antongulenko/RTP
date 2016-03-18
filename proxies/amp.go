package proxies

// Converts AMP to RTSP

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/protocols/amp_control"
	"github.com/antongulenko/RTP/rtpClient"
	"github.com/antongulenko/golib"
)

const (
	proxyOnError = OnErrorPause
)

type AmpProxy struct {
	*protocols.Server
	sessions protocols.Sessions

	rtspURL   *url.URL
	proxyHost string

	StreamStartedCallback func(rtsp *golib.Command, proxies []*UdpProxy)
	StreamStoppedCallback func(rtsp *golib.Command, proxies []*UdpProxy)
}

type streamSession struct {
	*protocols.SessionBase

	backend   *golib.Command
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
func RegisterAmpProxy(server *protocols.Server, rtspURL, localProxyIP string) (*AmpProxy, error) {
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

	proxy := &AmpProxy{
		rtspURL:   u,
		proxyHost: ip.String(),
		sessions:  make(protocols.Sessions),
		Server:    server,
	}
	if err := amp.RegisterServer(server, proxy); err != nil {
		return nil, err
	}
	// TODO if second registration fails, the first registration still stays in the server...
	if err := amp_control.RegisterServer(server, proxy); err != nil {
		return nil, err
	}
	return proxy, nil
}

func (proxy *AmpProxy) StopServer() {
	if err := proxy.sessions.DeleteSessions(); err != nil {
		proxy.LogError(fmt.Errorf("Error stopping all sessions: %v", err))
	}
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
	proxy.sessions.StartSession(client, session)
	return nil
}

func (proxy *AmpProxy) StopStream(desc *amp.StopStream) error {
	return proxy.sessions.DeleteSession(desc.Client())
}

func (proxy *AmpProxy) emergencyStopSession(client string, err error) error {
	stopErr := proxy.sessions.StopSession(client)
	if stopErr == nil {
		return fmt.Errorf("Error redirecting session for %v: %v", client, err)
	} else {
		return fmt.Errorf("Error redirecting session for %v: %v. Error stopping: %v", client, err, stopErr)
	}
}

func (proxy *AmpProxy) RedirectStream(desc *amp_control.RedirectStream) error {
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

func (proxy *AmpProxy) PauseStream(val *amp_control.PauseStream) error {
	sessionBase, ok := proxy.sessions[val.Client()]
	if !ok {
		return fmt.Errorf("Session not found exists for client %v", val.Client())
	}
	session, ok := sessionBase.Session.(*streamSession)
	if !ok { // Should never happen
		return fmt.Errorf("Illegal session type %T: %v", sessionBase, sessionBase)
	}
	session.rtpProxy.PauseWrite()
	session.rtcpProxy.PauseWrite()
	return nil
}

func (proxy *AmpProxy) ResumeStream(val *amp_control.ResumeStream) error {
	sessionBase, ok := proxy.sessions[val.Client()]
	if !ok {
		return fmt.Errorf("Session not found exists for client %v", val.Client())
	}
	session, ok := sessionBase.Session.(*streamSession)
	if !ok { // Should never happen
		return fmt.Errorf("Illegal session type %T: %v", sessionBase, sessionBase)
	}
	session.rtpProxy.ResumeWrite()
	session.rtcpProxy.ResumeWrite()
	return nil
}

func (proxy *AmpProxy) newStreamSession(desc *amp.StartStream) (*streamSession, error) {
	client := desc.Client()
	rtcpClient := net.JoinHostPort(desc.ReceiverHost, strconv.Itoa(desc.Port+1))
	rtpProxy, rtcpProxy, err := NewUdpProxyPair(proxy.proxyHost, client, rtcpClient)
	if err != nil {
		return nil, err
	}
	rtpProxy.OnError = proxyOnError
	rtcpProxy.OnError = proxyOnError
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

func (session *streamSession) Tasks() []golib.Task {
	errors1 := session.rtpProxy.WriteErrors()
	errors2 := session.rtcpProxy.WriteErrors()
	return []golib.Task{
		session.rtpProxy,
		session.rtcpProxy,
		session.backend,
		golib.NewLoopTask("printing proxy errors", func(stop golib.StopChan) {
			select {
			case err := <-errors1:
				session.proxy.LogError(fmt.Errorf("RTP %v write error: %v", session.rtpProxy, err))
			case err := <-errors2:
				session.proxy.LogError(fmt.Errorf("RTCP %v write error: %v", session.rtcpProxy, err))
			case <-stop:
			}
		}),
	}
}

func (session *streamSession) Start(base *protocols.SessionBase) {
	session.SessionBase = base
	if session.proxy.StreamStartedCallback != nil {
		session.proxy.StreamStartedCallback(session.backend, session.proxies())
	}
}

func (session *streamSession) Cleanup() {
	var errors golib.MultiError
	for _, p := range session.proxies() {
		if p.Err != nil {
			errors = append(errors, fmt.Errorf("Proxy %s error: %v", p, p.Err))
		}
	}
	if !session.backend.Success() {
		errors = append(errors, fmt.Errorf("%s", session.backend.StateString()))
	}
	session.CleanupErr = errors.NilOrError()
	if session.proxy.StreamStoppedCallback != nil {
		session.proxy.StreamStoppedCallback(session.backend, session.proxies())
	}
}
