package amp_balancer

import (
	"fmt"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

type AmpBalancer struct {
	*amp.Server
	sessions protocols.Sessions

	plugins []Plugin

	SessionStartedCallback func(client string)
	SessionStoppedCallback func(client string)
}

type balancerSession struct {
	base       *protocols.SessionBase
	clientAddr string
	startDesc  *amp.StartStream

	balancer *AmpBalancer
	plugins  []PluginSession
}

type Plugin interface {
	NewSession(desc *amp.StartStream) (PluginSession, error) // Will modify desc if necessary
	Stop(containingServer *protocols.Server)
}

type PluginSession interface {
	Start()
	Cleanup() error
	Observees() []helpers.Observee
}

func NewAmpBalancer(local_addr string) (balancer *AmpBalancer, err error) {
	balancer = &AmpBalancer{
		sessions: make(protocols.Sessions),
	}
	balancer.Server, err = amp.NewServer(local_addr, balancer)
	if err != nil {
		balancer = nil
	}
	return
}

func (balancer *AmpBalancer) AddPlugin(plugin Plugin) {
	balancer.plugins = append(balancer.plugins, plugin)
}

func (balancer *AmpBalancer) StopServer() {
	balancer.sessions.StopSessions()
	for _, plugin := range balancer.plugins {
		plugin.Stop(balancer.Server.Server)
	}
}

func (balancer *AmpBalancer) StartStream(desc *amp.StartStream) error {
	client := desc.Client()
	if _, ok := balancer.sessions[client]; ok {
		return fmt.Errorf("Session already running for client %v", client)
	}
	session, err := balancer.newSession(desc)
	if err != nil {
		return err
	}
	session.base = balancer.sessions.NewSession(client, session)
	return nil
}

func (balancer *AmpBalancer) newSession(desc *amp.StartStream) (*balancerSession, error) {
	clientAddr := desc.Client()
	session := &balancerSession{
		clientAddr: clientAddr,
		balancer:   balancer,
		startDesc:  desc,
		plugins:    make([]PluginSession, len(balancer.plugins)),
	}

	// Iterate plugin chain backwards: last plugin is facing the client
	for i := len(balancer.plugins) - 1; i >= 0; i-- {
		plugin := balancer.plugins[i]
		pluginSession, err := plugin.NewSession(desc)
		if err != nil {
			session.cleanupPlugins()
			return nil, err
		}
		session.plugins[i] = pluginSession
	}
	return session, nil
}

func (balancer *AmpBalancer) StopStream(desc *amp.StopStream) error {
	return balancer.sessions.StopSession(desc.Client())
}

func (session *balancerSession) Observees() (result []helpers.Observee) {
	for _, plugin := range session.plugins {
		result = append(result, plugin.Observees()...)
	}
	return
}

func (session *balancerSession) Start() {
	for i := len(session.plugins) - 1; i >= 0; i-- {
		session.plugins[i].Start()
	}
	if session.balancer.SessionStartedCallback != nil {
		session.balancer.SessionStartedCallback(session.clientAddr)
	}
}

func (session *balancerSession) cleanupPlugins() {
	for _, plugin := range session.plugins {
		if plugin != nil {
			if err := plugin.Cleanup(); err != nil {
				session.base.CleanupErr = fmt.Errorf("Error stopping plugin session: %v", err)
			}
		}
	}
}

func (session *balancerSession) Cleanup() {
	session.cleanupPlugins()
	if session.balancer.SessionStoppedCallback != nil {
		session.balancer.SessionStoppedCallback(session.clientAddr)
	}
}
