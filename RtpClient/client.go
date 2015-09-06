package rtpClient

import (
	"github.com/antongulenko/RTP/PacketStats"
	"github.com/antongulenko/gortp"
	"log"
	"net"
	"os"
	"sync"
)

const (
	rtpDataBuffer = 128
	rtpCtrlBuffer = 5
)

type RtpClient struct {
	listenPort int
	wg         sync.WaitGroup
	rtspProc   *os.Process

	RtpSession *rtp.Session
	ctrlChan   rtp.CtrlEventChan
	dataChan   rtp.DataReceiveChan

	requestOnce    sync.Once
	subprocessDied chan interface{}

	SequenceNumber uint16
	ReceiveStats   *stats.Stats
	CtrlStats      *stats.Stats
	MissedStats    *stats.Stats
}

func NewRtpClient(listenIP string, listenPort int) (*RtpClient, error) {
	localAddr, err := net.ResolveIPAddr("ip", listenIP)
	if err != nil {
		return nil, err
	}
	rtpTransport, err := rtp.NewTransportUDP(localAddr, listenPort)
	if err != nil {
		return nil, err
	}
	session := rtp.NewSession(rtpTransport, rtpTransport)
	err = session.StartSession()
	if err != nil {
		return nil, err
	}
	log.Printf("Listening on %v UDP ports %v and %v for rtp/rtcp\n", listenIP, listenPort, listenPort+1)

	client := &RtpClient{
		listenPort:     listenPort,
		ReceiveStats:   stats.NewStats("RTP received"),
		MissedStats:    stats.NewStats("RTP missed"),
		CtrlStats:      stats.NewStats("RTCP events"),
		RtpSession:     session,
		ctrlChan:       session.CreateCtrlEventChan(rtpCtrlBuffer),
		dataChan:       session.CreateDataReceiveChan(rtpDataBuffer),
		subprocessDied: make(chan interface{}, 1),
	}

	client.wg.Add(1)
	go client.handleCtrlEvents()
	client.wg.Add(1)
	go client.handleDataPackets()

	return client, nil
}

func (client *RtpClient) SubprocessDied() <-chan interface{} {
	return client.subprocessDied
}

func (client *RtpClient) Request(rtspUrl string, proxyPort int) (err error) {
	if proxyPort == 0 {
		proxyPort = client.listenPort
	}
	client.requestOnce.Do(func() {
		err := client.startRTSP(rtspUrl, proxyPort)
		if err != nil {
			return
		}
		client.wg.Add(1)
		go client.observeRTSP()
	})
	return
}

func (client *RtpClient) Stop() {
	client.ReceiveStats.Stop()
	client.MissedStats.Stop()
	client.stopRTSP()
	client.RtpSession.CloseSession()
	close(client.dataChan)
	close(client.ctrlChan)
	client.wg.Wait()
}
