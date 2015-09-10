package amp

import "github.com/antongulenko/RTP/protocols"

type Client struct {
	*protocols.Client
	*ampProtocol
}

func NewClient(local_ip string) (client *Client, err error) {
	client = new(Client)
	client.Client, err = protocols.NewClient(local_ip, client)
	if err != nil {
		client = nil
	}
	return
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
