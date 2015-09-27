package amp_balancer

import (
	"fmt"

	"github.com/antongulenko/RTP/helpers"
	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
)

type ExtendedAmpServer struct {
	*amp.Server
	sessions protocols.Sessions

	plugins []Plugin

	SessionStartedCallback func(client string)
	SessionStoppedCallback func(client string)
}

type ampServerSession struct {
	base       *protocols.SessionBase
	clientAddr string

	server  *ExtendedAmpServer
	plugins []PluginSession
}

type Plugin interface {
	Start(server *ExtendedAmpServer)
	NewSession(containingSession *ampServerSession, desc *amp.StartStream) (PluginSession, error) // Modify desc if necessary, do not store it.
	Stop() []error
}

type PluginSession interface {
	Start(sendingSession PluginSession)
	Cleanup() error
	Observees() []helpers.Observee
}

func NewExtendedAmpServer(local_addr string) (server *ExtendedAmpServer, err error) {
	server = &ExtendedAmpServer{
		sessions: make(protocols.Sessions),
	}
	server.Server, err = amp.NewServer(local_addr, server)
	if err != nil {
		server = nil
	}
	return
}

func (server *ExtendedAmpServer) AddPlugin(plugin Plugin) {
	server.plugins = append(server.plugins, plugin)
	plugin.Start(server)
}

func (server *ExtendedAmpServer) StopServer() {
	if err := server.sessions.DeleteSessions(); err != nil {
		server.LogError(fmt.Errorf("Error stopping sessions: %v", err))
	}
	for _, plugin := range server.plugins {
		for _, err := range plugin.Stop() {
			server.LogError(fmt.Errorf("Error stopping plugin: %v", err))
		}
	}
}

func (server *ExtendedAmpServer) StartStream(desc *amp.StartStream) error {
	client := desc.Client()
	if _, ok := server.sessions[client]; ok {
		return fmt.Errorf("Session already running for client %v", client)
	}
	session, err := server.newSession(desc)
	if err != nil {
		return err
	}
	server.sessions.StartSession(client, session)
	return nil
}

func (server *ExtendedAmpServer) newSession(desc *amp.StartStream) (*ampServerSession, error) {
	clientAddr := desc.Client()
	session := &ampServerSession{
		clientAddr: clientAddr,
		server:     server,
		plugins:    make([]PluginSession, len(server.plugins)),
	}

	// Iterate plugin chain backwards: last plugin is facing the client
	for i := len(server.plugins) - 1; i >= 0; i-- {
		plugin := server.plugins[i]
		pluginSession, err := plugin.NewSession(session, desc)
		if err != nil {
			_ = session.cleanupPlugins() // Drop error
			return nil, err
		}
		session.plugins[i] = pluginSession
	}
	return session, nil
}

func (server *ExtendedAmpServer) StopStream(desc *amp.StopStream) error {
	return server.sessions.DeleteSession(desc.Client())
}

func (server *ExtendedAmpServer) StopSession(client string) error {
	return server.sessions.StopSession(client)
}

func (server *ExtendedAmpServer) RedirectStream(val *amp.RedirectStream) error {
	return fmt.Errorf("AMP RedirectStream not implemented for this AMP server")
}

func (session *ampServerSession) Observees() (result []helpers.Observee) {
	for _, plugin := range session.plugins {
		result = append(result, plugin.Observees()...)
	}
	return
}

func (session *ampServerSession) Start(base *protocols.SessionBase) {
	session.base = base
	for i := len(session.plugins) - 1; i >= 0; i-- {
		var sendingSession PluginSession
		if i >= 1 {
			sendingSession = session.plugins[i-1]
		}
		session.plugins[i].Start(sendingSession)
	}
	if session.server.SessionStartedCallback != nil {
		session.server.SessionStartedCallback(session.clientAddr)
	}
}

func (session *ampServerSession) cleanupPlugins() error {
	var errors helpers.MultiError
	for _, plugin := range session.plugins {
		if plugin != nil {
			if err := plugin.Cleanup(); err != nil {
				errors = append(errors, fmt.Errorf("Error stopping plugin session: %v", err))
			}
		}
	}
	return errors.NilOrError()
}

func (session *ampServerSession) Cleanup() {
	if err := session.cleanupPlugins(); err != nil {
		session.base.CleanupErr = err
	}
	if session.server.SessionStoppedCallback != nil {
		session.server.SessionStoppedCallback(session.clientAddr)
	}
}
