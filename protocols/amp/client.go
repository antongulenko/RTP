package amp

import "github.com/antongulenko/RTP/protocols"

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

func (client *Client) StartStream(clientHost string, port int, mediaFile string) error {
	val := &StartStream{
		ClientDescription: ClientDescription{
			ReceiverHost: clientHost,
			Port:         port,
		},
		MediaFile: mediaFile,
	}
	reply, err := client.SendRequest(CodeStartStream, val)
	if err != nil {
		return err
	}
	return client.CheckReply(reply)
}

func (client *Client) StopStream(clientHost string, port int) error {
	val := &StopStream{
		ClientDescription{
			ReceiverHost: clientHost,
			Port:         port,
		},
	}
	reply, err := client.SendRequest(CodeStopStream, val)
	if err != nil {
		return err
	}
	return client.CheckReply(reply)
}
