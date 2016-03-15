package protocols

// Extension of server.go allowing for multiple "plugins" to handle one session

import (
	"fmt"

	"github.com/antongulenko/golib"
)

type PluginServer struct {
	*Server
	sessions Sessions

	plugins []Plugin

	SessionStartedCallback func(session *PluginSession)
	SessionStoppedCallback func(session *PluginSession)
}

type Plugin interface {
	Start(server *PluginServer)
	Stop() error

	// Create and fully initialize new session. The param data is passed from
	// plugin to plugin, enabling one plugin to modify the input data for the next plugin.
	// Modify param if necessary and copy values from it. Do not store it.
	NewSession(param SessionParameter) (PluginSessionHandler, error)
}

type PluginSession struct {
	Base    *SessionBase
	Client  string // From the originating SessionParameter
	Server  *PluginServer
	Plugins []PluginSessionHandler
}

type PluginSessionHandler interface {
	Start(sendingSession PluginSessionHandler)
	Observees() []golib.Observee
	Cleanup() error
	String() string
}

type SessionParameter interface {
	// This string will be used as key in the sessions dictionary
	Client() string
}

func NewPluginServer(server *Server) *PluginServer {
	return &PluginServer{
		Server:   server,
		sessions: make(Sessions),
	}
}

func (server *PluginServer) AddPlugin(plugin Plugin) {
	server.plugins = append(server.plugins, plugin)
	plugin.Start(server)
}

func (server *PluginServer) StopServer() {
	if err := server.sessions.DeleteSessions(); err != nil {
		server.LogError(fmt.Errorf("Error stopping sessions: %v", err))
	}
	for _, plugin := range server.plugins {
		if err := plugin.Stop(); err != nil {
			server.LogError(fmt.Errorf("Error stopping plugin: %v", err))
		}
	}
}

func (server *PluginServer) NewSession(param SessionParameter) error {
	clientAddr := param.Client()
	if _, ok := server.sessions[clientAddr]; ok {
		return fmt.Errorf("Session already running for client %v", clientAddr)
	}
	session := &PluginSession{
		Client:  clientAddr,
		Server:  server,
		Plugins: make([]PluginSessionHandler, len(server.plugins)),
	}

	// Iterate plugin chain backwards: last plugin is facing the client
	for i := len(server.plugins) - 1; i >= 0; i-- {
		plugin := server.plugins[i]
		handler, err := plugin.NewSession(param)
		if err != nil {
			_ = session.cleanupPlugins() // Drop error
			return err
		}
		session.Plugins[i] = handler
	}
	server.sessions.StartSession(clientAddr, session)
	return nil
}

func (server *PluginServer) StopSession(client string) error {
	return server.sessions.StopSession(client)
}

func (server *PluginServer) DeleteSession(client string) error {
	return server.sessions.DeleteSession(client)
}

func (session *PluginSession) Observees() (result []golib.Observee) {
	for _, plugin := range session.Plugins {
		result = append(result, plugin.Observees()...)
	}
	return
}

func (session *PluginSession) Start(base *SessionBase) {
	session.Base = base
	for i := len(session.Plugins) - 1; i >= 0; i-- {
		var sendingSession PluginSessionHandler
		if i >= 1 {
			sendingSession = session.Plugins[i-1]
		}
		session.Plugins[i].Start(sendingSession)
	}
	if session.Server.SessionStartedCallback != nil {
		session.Server.SessionStartedCallback(session)
	}
}

func (session *PluginSession) cleanupPlugins() error {
	var errors golib.MultiError
	for _, plugin := range session.Plugins {
		if plugin != nil {
			if err := plugin.Cleanup(); err != nil {
				errors = append(errors, fmt.Errorf("Error stopping plugin session: %v", err))
			}
		}
	}
	return errors.NilOrError()
}

func (session *PluginSession) Cleanup() {
	if err := session.cleanupPlugins(); err != nil {
		session.Base.CleanupErr = err
	}
	if session.Server.SessionStoppedCallback != nil {
		session.Server.SessionStoppedCallback(session)
	}
}
