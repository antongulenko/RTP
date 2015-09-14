package pcp

import "github.com/antongulenko/RTP/protocols"

type Client struct {
	protocols.ExtendedClient
	*pcpProtocol
}

func NewClient(local_ip string) (client *Client, err error) {
	client = new(Client)
	client.ExtendedClient, err = protocols.NewExtendedClient(local_ip, client)
	if err != nil {
		client = nil
	}
	return
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
