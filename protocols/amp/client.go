package amp

import "github.com/antongulenko/RTP/protocols"

type Client struct {
	protocols.ExtendedClient
	*AmpProtocol
}

type CircuitBreaker interface {
	protocols.CircuitBreaker
	StartStream(clientHost string, port int, mediaFile string) error
	StopStream(clientHost string, port int) error
}

func NewClient(local_ip string) (client *Client, err error) {
	client = new(Client)
	client.ExtendedClient, err = protocols.NewExtendedClient(local_ip, client)
	if err != nil {
		client = nil
	}
	return
}

type circuitBreaker struct {
	protocols.CircuitBreaker
	*Client
}

func NewCircuitBreaker(local_ip string) (CircuitBreaker, error) {
	proto := new(AmpProtocol)
	baseClient, err := protocols.NewExtendedClient(local_ip, proto)
	if err != nil {
		return nil, err
	}
	breaker := protocols.NewCircuitBreaker(baseClient)
	return &circuitBreaker{
		CircuitBreaker: breaker,
		Client: &Client{
			ExtendedClient: breaker,
			AmpProtocol:    proto,
		},
	}, nil
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

func (client *Client) RedirectStream(oldHost string, oldPort int, newHost string, newPort int) error {
	val := &RedirectStream{
		OldClient: ClientDescription{
			ReceiverHost: oldHost,
			Port:         oldPort,
		},
		NewClient: ClientDescription{
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
