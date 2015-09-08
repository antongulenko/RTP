package proxies

// Converts AMP to RTSP

import (
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/rtpClient"
)
import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

type AmpProxy struct {
	*protocols.Server

	targetURL *url.URL
	sessions  map[int]*proxySession

	ProcessDiedCallback func(rtsp *rtpClient.RtspClient)
}

type proxySession struct {
	// TODO Observe RtspClient / report errors
	backend   *rtpClient.RtspClient
	port      int
	mediaFile string
}

func NewAmpProxy(ampAddr string, targetURL string) (*AmpProxy, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "rtsp" {
		return nil, errors.New("Need rtsp:// targetURL for AmpProxy")
	}

	proxy := &AmpProxy{
		targetURL: u,
		sessions:  make(map[int]*proxySession),
	}
	proxy.Server, err = protocols.NewServer(ampAddr, proxy)
	if err != nil {
		return nil, err
	}
	return proxy, nil
}

func (proxy *AmpProxy) StopServer() {
	for _, session := range proxy.sessions {
		session.stop()
	}
}

func (proxy *AmpProxy) ReceivePacket(conn *net.UDPConn) (*protocols.Packet, error) {
	packet, err := amp.ReceivePacket(conn)
	if err != nil {
		return nil, err
	}
	return packet.Packet, err
}

func (proxy *AmpProxy) HandleRequest(request *protocols.Packet) {
	packet := &amp.AmpPacket{request}
	switch packet.Code {
	case amp.CodeStartSession:
		if desc := packet.StartSession(); desc == nil {
			proxy.ReplyError(packet.Packet, fmt.Errorf("Illegal value for AMP CodeStartSession: %v", packet.Val))
		} else {
			proxy.ReplyCheck(packet.Packet, proxy.startSession(desc))
		}
	case amp.CodeStopSession:
		if desc := packet.StopSession(); desc == nil {
			proxy.ReplyError(packet.Packet, fmt.Errorf("Illegal value for AMP CodeStopSession: %v", packet.Val))
		} else {
			proxy.ReplyCheck(packet.Packet, proxy.stopSession(desc))
		}
	default:
		proxy.LogError(fmt.Errorf("Received unexpected AMP code: %v", packet.Code))
	}
}

func (proxy *AmpProxy) startSession(desc *amp.StartSessionValue) error {
	_, ok := proxy.sessions[desc.Port]
	if ok {
		return fmt.Errorf("Session already exists for mediaFile %v on port %v", desc.MediaFile, desc.Port)
	}
	mediaURL := proxy.targetURL.ResolveReference(&url.URL{Path: desc.MediaFile})
	logfile := fmt.Sprintf("amp-proxy-%v-%v.log", desc.Port, desc.MediaFile)
	rtsp, err := rtpClient.StartRtspClient(mediaURL.String(), desc.Port, logfile)
	if err != nil {
		return fmt.Errorf("Failed to start RTSP client: %v", err)
	}
	session := &proxySession{
		backend:   rtsp,
		mediaFile: desc.MediaFile,
		port:      desc.Port,
	}
	proxy.sessions[desc.Port] = session
	proxy.observe(session)
	return nil
}

func (proxy *AmpProxy) stopSession(desc *amp.StopSessionValue) error {
	session, ok := proxy.sessions[desc.Port]
	if !ok {
		return fmt.Errorf("Session not found for mediaFile %v on port %v", desc.MediaFile, desc.Port)
	}
	proxy.cleanupSession(session)
	return nil
}

func (proxy *AmpProxy) cleanupSession(session *proxySession) {
	session.stop()
	delete(proxy.sessions, session.port)
}

func (proxy *AmpProxy) observe(session *proxySession) {
	rtsp := session.backend
	c := rtsp.Observe(proxy.Wg)
	go func() {
		<-c
		if proxy.ProcessDiedCallback != nil {
			proxy.ProcessDiedCallback(rtsp)
		}
		proxy.cleanupSession(session)
	}()
}

func (session *proxySession) stop() {
	session.backend.Stop()
}
