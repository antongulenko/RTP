package pcp

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

type Client struct {
	protocols.ExtendedClient
	*PcpProtocol
}

type CircuitBreaker interface {
	protocols.CircuitBreaker
	StartProxy(listenAddr string, targetAddr string) error
	StopProxy(listenAddr string, targetAddr string) error
	StartProxyPair(	receiverHost string, receiverPort1, receiverPort2 int) (*StartProxyPairResponse, error)
	StopProxyPair(proxyPort1 int) error
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
	proto := new(PcpProtocol)
	baseClient, err := protocols.NewExtendedClient(local_ip, proto)
	if err != nil {
		return nil, err
	}
	breaker := protocols.NewCircuitBreaker(baseClient)
	return &circuitBreaker{
		CircuitBreaker: breaker,
		Client: &Client{
			ExtendedClient: breaker,
			PcpProtocol:    proto,
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

func (client *Client) StartProxyPair(receiverHost string, receiverPort1, receiverPort2 int) (*StartProxyPairResponse, error) {
	val := &StartProxyPair{
		ReceiverHost:  receiverHost,
		ReceiverPort1: receiverPort1,
		ReceiverPort2: receiverPort2,
	}
	reply, err := client.SendRequest(CodeStartProxyPair, val)
	if err != nil {
		return nil, err
	}
	if err = client.CheckError(reply, CodeStartProxyPairResponse); err != nil {
		return nil, err
	}
	response, ok := reply.Val.(*StartProxyPairResponse)
	if !ok {
		return nil, fmt.Errorf("Illegal StartProxyPairResponse payload: (%T) %s", reply.Val, reply.Val)
	}
	return response, nil
}

func (client *Client) StopProxyPair(proxyPort1 int) error {
	val := &StopProxyPair{
		ProxyPort1: proxyPort1,
	}
	reply, err := client.SendRequest(CodeStopProxyPair, val)
	if err != nil {
		return err
	}
	return client.CheckReply(reply)
}
