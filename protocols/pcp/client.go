package pcp

import "github.com/antongulenko/RTP/protocols"

type Client struct {
	protocols.ExtendedClient
	*pcpProtocol
}

type CircuitBreaker interface {
	protocols.CircuitBreaker
	StartProxy(listenAddr string, targetAddr string) error
	StopProxy(listenAddr string, targetAddr string) error
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
	proto := new(pcpProtocol)
	baseClient, err := protocols.NewExtendedClient(local_ip, proto)
	if err != nil {
		return nil, err
	}
	breaker := protocols.NewCircuitBreaker(baseClient)
	return &circuitBreaker{
		CircuitBreaker: breaker,
		Client: &Client{
			ExtendedClient: breaker,
			pcpProtocol:    proto,
		},
	}, nil
}

func (client *Client) StartProxy(listenAddr string, targetAddr string) error {
	val := &StartProxy{
		ProxyDescription{
			ListenAddr: listenAddr,
			TargetAddr: targetAddr,
		},
	}
	reply, err := client.SendRequest(CodeStartProxy, val)
	if err != nil {
		return err
	}
	return client.CheckReply(reply)
}

func (client *Client) StopProxy(listenAddr string, targetAddr string) error {
	val := &StopProxy{
		ProxyDescription{
			ListenAddr: listenAddr,
			TargetAddr: targetAddr,
		},
	}
	reply, err := client.SendRequest(CodeStopProxy, val)
	if err != nil {
		return err
	}
	return client.CheckReply(reply)
}
