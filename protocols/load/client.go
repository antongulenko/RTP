package load

import (
	"sync"
	"time"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
)

type Client struct {
	protocols.Client
	seq         uint
	loadRunning bool
	wg          sync.WaitGroup
	waitTime    time.Duration
	lastErr     error

	pausedCond *sync.Cond
	paused     bool
}

func NewClient() *Client {
	client := &Client{
		Client:     protocols.NewClient(MiniProtocol),
		pausedCond: sync.NewCond(new(sync.Mutex)),
	}
	client.Pause()
	client.sendLoad()
	return client
}

func (client *Client) SendLoad() error {
	err := client.Send(codeLoad, &LoadPacket{
		Seq: client.seq,
	})
	client.seq++
	return err
}

func (client *Client) StartLoad(bytePerSecond uint64) {
	client.waitTime = time.Duration(uint64(time.Second) * PacketSize / bytePerSecond)
	client.Resume()
}

func (client *Client) sendLoad() {
	client.wg.Add(1)
	go func() {
		defer client.wg.Done()
		for !client.Closed() {
			client.waitWhilePaused()
			if client.Closed() {
				return
			}
			client.lastErr = client.SendLoad()
			if client.lastErr != nil {
				if client.Closed() {
					client.lastErr = nil
					return
				}
				client.Pause()
			}
			if client.waitTime == 0 {
				client.waitTime = 1 * time.Second
			}
			time.Sleep(client.waitTime)
		}
	}()
}

func (client *Client) Pause() {
	client.pausedCond.L.Lock()
	defer client.pausedCond.L.Unlock()
	client.paused = true
}

func (client *Client) Resume() {
	client.pausedCond.L.Lock()
	defer client.pausedCond.L.Unlock()
	client.paused = false
	client.pausedCond.Broadcast()
}

func (client *Client) waitWhilePaused() {
	client.pausedCond.L.Lock()
	defer client.pausedCond.L.Unlock()
	for client.paused {
		client.pausedCond.Wait()
	}
}

func (client *Client) Close() error {
	var err helpers.MultiError
	err.Add(client.Client.Close())
	client.Resume()
	client.wg.Wait()
	err.Add(client.lastErr)
	return err.NilOrError()
}
