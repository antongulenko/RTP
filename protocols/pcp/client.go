package pcp

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
)

// ======================= Client =======================

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

func (client *Client) StartProxy(listenAddr string, targetAddr string) error {
	val := &StartProxy{
		ProxyDescription{
			ListenAddr: listenAddr,
			TargetAddr: targetAddr,
		},
	}
	reply, err := client.SendRequest(codeStartProxy, val)
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
	reply, err := client.SendRequest(codeStopProxy, val)
	if err != nil {
		return err
	}
	return client.CheckReply(reply)
}

func (client *Client) StartProxyPair(proxyHost, receiverHost string, receiverPort1, receiverPort2 int) (*StartProxyPairResponse, error) {
	val := &StartProxyPair{
		ProxyHost:     proxyHost,
		ReceiverHost:  receiverHost,
		ReceiverPort1: receiverPort1,
		ReceiverPort2: receiverPort2,
	}
	reply, err := client.SendRequest(codeStartProxyPair, val)
	if err != nil {
		return nil, err
	}
	if err = client.CheckError(reply, codeStartProxyPairResponse); err != nil {
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
	reply, err := client.SendRequest(codeStopProxyPair, val)
	if err != nil {
		return err
	}
	return client.CheckReply(reply)
}
