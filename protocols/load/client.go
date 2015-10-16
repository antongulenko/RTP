package load

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
)

type Client struct {
	protocols.Client
	seq          uint
	loadRunning  bool
	waitTime     time.Duration
	lastErr      error
	extraPayload []byte

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

func (client *Client) SetPayload(size uint) error {
	payload := make([]byte, size)
	_, err := rand.Read(payload)
	if err != nil {
		return fmt.Errorf("Warning: error reading random payload data:", err)
	}
	client.extraPayload = payload
	return nil
}

func (client *Client) SendLoad() error {
	err := client.Send(codeLoad, &LoadPacket{
		Seq:     client.seq,
		Payload: client.extraPayload,
	})
	client.seq++
	return err
}

func (client *Client) StartLoad(bytePerSecond uint64) {
	size := PacketSize + uint64(len(client.extraPayload))
	client.waitTime = time.Duration(uint64(time.Second) * size / bytePerSecond)
	client.Resume()
}

func (client *Client) sendLoad() {
	go func() {
		for !client.Closed() {
			client.waitWhilePaused()
			if client.Closed() {
				return
			}
			err := client.SendLoad()
			if err != nil {
				client.pausedCond.L.Lock()
				defer client.pausedCond.L.Unlock()
				if client.Closed() {
					return
				}
				client.lastErr = err
				client.pause(false)
			}
			if client.waitTime == 0 {
				client.waitTime = 1 * time.Second
			}
			if client.Closed() {
				return
			}
			time.Sleep(client.waitTime)
		}
	}()
}

func (client *Client) Pause() {
	client.pause(true)
}

func (client *Client) Resume() {
	client.resume(true)
}

func (client *Client) pause(lock bool) {
	if lock {
		client.pausedCond.L.Lock()
		defer client.pausedCond.L.Unlock()
	}
	client.paused = true
}

func (client *Client) resume(lock bool) {
	if lock {
		client.pausedCond.L.Lock()
		defer client.pausedCond.L.Unlock()
	}
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
	client.pausedCond.L.Lock()
	defer client.pausedCond.L.Unlock()
	err.Add(client.Client.Close())
	client.resume(false)
	err.Add(client.lastErr)
	return err.NilOrError()
}
