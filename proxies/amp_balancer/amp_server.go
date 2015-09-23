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
	NewSession(containingSession *ampServerSession, desc *amp.StartStream) (PluginSession, error) // Will modify desc if necessary. Do not store desc itself.
	Stop(containingServer *protocols.Server) []error
}

type PluginSession interface {
	Start()
	Cleanup() error
	Observees() []helpers.Observee
	RedirectStream(newHost string, newPort int) error
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
	for _, err := range server.sessions.StopSessions() {
		server.LogError(fmt.Errorf("Error closing session: %v", err))
	}
	for _, plugin := range server.plugins {
		for _, err := range plugin.Stop(server.Server.Server) {
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
	session.base = server.sessions.NewSession(client, session)
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
			session.cleanupPlugins()
			return nil, err
		}
		session.plugins[i] = pluginSession
	}
	return session, nil
}

func (server *ExtendedAmpServer) StopStream(desc *amp.StopStream) error {
	return server.sessions.StopSession(desc.Client())
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

func (session *ampServerSession) Start() {
	for i := len(session.plugins) - 1; i >= 0; i-- {
		session.plugins[i].Start()
	}
	if session.server.SessionStartedCallback != nil {
		session.server.SessionStartedCallback(session.clientAddr)
	}
}

func (session *ampServerSession) cleanupPlugins() {
	for _, plugin := range session.plugins {
		if plugin != nil {
			if err := plugin.Cleanup(); err != nil {
				session.base.CleanupErr = fmt.Errorf("Error stopping plugin session: %v", err)
			}
		}
	}
}

func (session *ampServerSession) Cleanup() {
	session.cleanupPlugins()
	if session.server.SessionStoppedCallback != nil {
		session.server.SessionStoppedCallback(session.clientAddr)
	}
}

func (session *ampServerSession) RedirectStream(receivingSession PluginSession, newHost string, newPort int) error {
	var sendingSession PluginSession
	for _, session := range session.plugins {
		if session == receivingSession {
			break
		}
		sendingSession = session
	}
	if sendingSession == nil {
		return fmt.Errorf("Failed to find sending plugin for %T plugin in session for %v", receivingSession, session.clientAddr)
	}
	return sendingSession.RedirectStream(newHost, newPort)
}
