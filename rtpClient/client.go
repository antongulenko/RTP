package rtpClient

import (
	"net"
	"sync"

	"github.com/antongulenko/RTP/stats"
	"github.com/antongulenko/golib"
	"github.com/antongulenko/gortp"
)

const (
	rtpDataBuffer = 128
	rtpCtrlBuffer = 5
)

type RtpClient struct {
	wg      sync.WaitGroup
	stopped *golib.OneshotCondition

	listenPort     int
	sequenceNumber uint16
	ctrlChan       rtp.CtrlEventChan
	dataChan       rtp.DataReceiveChan

	RtpSession *rtp.Session

	ReceiveStats *stats.Stats
	CtrlStats    *stats.Stats
	MissedStats  *stats.Stats
	DataHandler  func(packet *rtp.DataPacket)
	CtrlHandler  func(packet *rtp.CtrlEvent)
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

	client := &RtpClient{
		listenPort:   listenPort,
		ReceiveStats: stats.NewStats("RTP received"),
		MissedStats:  stats.NewStats("RTP missed"),
		CtrlStats:    stats.NewStats("RTCP events"),
		RtpSession:   session,
		ctrlChan:     session.CreateCtrlEventChan(rtpCtrlBuffer),
		dataChan:     session.CreateDataReceiveChan(rtpDataBuffer),
		stopped:      golib.NewOneshotCondition(),
	}

	client.wg.Add(1)
	go client.handleCtrlEvents()
	client.wg.Add(1)
	go client.handleDataPackets()

	return client, nil
}

func (client *RtpClient) Stop() {
	if client == nil {
		return
	}
	client.stopped.Enable(func() {
		client.ReceiveStats.Stop()
		client.CtrlStats.Stop()
		client.MissedStats.Stop()
		client.RtpSession.CloseSession()
		close(client.dataChan)
		close(client.ctrlChan)
		client.wg.Wait()
	})
}

func (client *RtpClient) Start(wg *sync.WaitGroup) golib.StopChan {
	return client.stopped.Start(wg)
}
