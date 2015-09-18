package amp_balancer

import (
	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

type Plugin interface {
	// NewSession will modify desc if necessary
	NewSession(desc *amp.StartStream) (PluginSession, error)
	Stop(containingServer *protocols.Server)
}

type PluginSession interface {
	Start()
	Cleanup() error
	Observees() []helpers.Observee
}
