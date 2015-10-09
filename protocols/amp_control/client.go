package amp_control

import (
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

type Client struct {
	protocols.Client
}

func NewClient(client protocols.Client) (*Client, error) {
	if err := client.Protocol().CheckIncludesFragment(Protocol.Name()); err != nil {
		return nil, err
	}
	return &Client{client}, nil
}

func NewClientFor(server_addr string) (*Client, error) {
	client, err := protocols.NewMiniClientFor(server_addr, Protocol)
	if err != nil {
		return nil, err
	}
	return &Client{client}, nil
}

func (client *Client) RedirectStream(oldHost string, oldPort int, newHost string, newPort int) error {
	val := &RedirectStream{
		OldClient: amp.ClientDescription{
			ReceiverHost: oldHost,
			Port:         oldPort,
		},
		NewClient: amp.ClientDescription{
			ReceiverHost: newHost,
			Port:         newPort,
		},
	}
	reply, err := client.SendRequest(CodeRedirectStream, val)
	if err != nil {
		return err
	}
	return client.CheckReply(reply)
}
