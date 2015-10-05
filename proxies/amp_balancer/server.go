package amp_balancer

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

func NewAmpPluginServer(local_addr string) (*protocols.PluginServer, error) {
	handler := new(ampPluginServerHandler)
	server, err := amp.NewServer(local_addr, handler)
	if err != nil {
		return nil, err
	}
	handler.PluginServer = protocols.NewPluginServer(server.Server)
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

func (handler *ampPluginServerHandler) RedirectStream(val *amp.RedirectStream) error {
	return fmt.Errorf("AMP RedirectStream not implemented for this AMP server")
}

func (handler *ampPluginServerHandler) PauseStream(val *amp.PauseStream) error {
	return fmt.Errorf("AMP PauseStream not implemented for this AMP server")
}

func (handler *ampPluginServerHandler) ResumeStream(val *amp.ResumeStream) error {
	return fmt.Errorf("AMP ResumeStream not implemented for this AMP server")
}
