package amp_balancer

import (
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

func RegisterPluginServer(server *protocols.Server) (*protocols.PluginServer, error) {
	handler := new(ampPluginServerHandler)
	err := amp.RegisterServer(server, handler)
	if err != nil {
		return nil, err
	}
	handler.PluginServer = protocols.NewPluginServer(server)
	return handler.PluginServer, nil
}

type ampPluginServerHandler struct {
	*protocols.PluginServer
}

func (handler *ampPluginServerHandler) StartStream(desc *amp.StartStream) error {
	return handler.NewSession(desc)
}

func (handler *ampPluginServerHandler) StopStream(desc *amp.StopStream) error {
	return handler.DeleteSession(desc.Client())
}
