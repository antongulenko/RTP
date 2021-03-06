package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/protocols/amp"
	"github.com/antongulenko/RTP/protocols/amp_control"
	"github.com/antongulenko/RTP/protocols/load"
	"github.com/antongulenko/golib"
)

var (
	DefaultLoad = 1024 // Default: 1kB/s
)

type LoadServer struct {
	*protocols.Server
	sessions protocols.Sessions

	PayloadSize uint
}

type loadSession struct {
	*protocols.SessionBase
	client *load.Client
	load   uint64
}

func RegisterLoadServer(server *protocols.Server) (*LoadServer, error) {
	load := &LoadServer{
		sessions: make(protocols.Sessions),
		Server:   server,
	}
	if err := amp.RegisterServer(server, load); err != nil {
		return nil, err
	}
	// TODO if second registration fails, the first registration still stays in the server...
	if err := amp_control.RegisterServer(server, load); err != nil {
		return nil, err
	}
	return load, nil
}

func (server *LoadServer) StopServer() {
	if err := server.sessions.DeleteSessions(); err != nil {
		server.LogError(fmt.Errorf("Error stopping all sessions: %v", err))
	}
}

func (server *LoadServer) StartStream(desc *amp.StartStream) error {
	client := desc.Client()
	if _, ok := server.sessions[client]; ok {
		return fmt.Errorf("Session already exists for client %v", client)
	}
	session, err := server.newStreamSession(desc)
	if err != nil {
		return err
	}
	server.sessions.StartSession(client, session)
	return nil
}

func (server *LoadServer) StopStream(desc *amp.StopStream) error {
	return server.sessions.DeleteSession(desc.Client())
}

func (server *LoadServer) emergencyStopSession(client string, err error) error {
	stopErr := server.sessions.StopSession(client)
	if stopErr == nil {
		return fmt.Errorf("Error redirecting session for %v: %v", client, err)
	} else {
		return fmt.Errorf("Error redirecting session for %v: %v. Error stopping: %v", client, err, stopErr)
	}
}

func (server *LoadServer) RedirectStream(desc *amp_control.RedirectStream) error {
	oldClient := desc.OldClient.Client()
	newClient := desc.NewClient.Client()
	sessionBase, err := server.sessions.ReKeySession(oldClient, newClient)
	if err != nil {
		return err
	}
	session, ok := sessionBase.Session.(*loadSession)
	if !ok {
		return server.emergencyStopSession(newClient, // Should never happen
			fmt.Errorf("Illegal session type %T: %v", sessionBase, sessionBase))
	}
	if err := session.client.SetServer(newClient); err != nil {
		return server.emergencyStopSession(newClient, err)
	}
	return nil
}

func (proxy *LoadServer) PauseStream(val *amp_control.PauseStream) error {
	sessionBase, ok := proxy.sessions[val.Client()]
	if !ok {
		return fmt.Errorf("Session not found exists for client %v", val.Client())
	}
	session, ok := sessionBase.Session.(*loadSession)
	if !ok { // Should never happen
		return fmt.Errorf("Illegal session type %T: %v", sessionBase, sessionBase)
	}
	session.client.Pause()
	return nil
}

func (proxy *LoadServer) ResumeStream(val *amp_control.ResumeStream) error {
	sessionBase, ok := proxy.sessions[val.Client()]
	if !ok {
		return fmt.Errorf("Session not found exists for client %v", val.Client())
	}
	session, ok := sessionBase.Session.(*loadSession)
	if !ok { // Should never happen
		return fmt.Errorf("Illegal session type %T: %v", sessionBase, sessionBase)
	}
	session.client.Resume()
	return nil
}

func (server *LoadServer) newStreamSession(desc *amp.StartStream) (*loadSession, error) {
	target := desc.Client()
	client := load.NewClient()
	if err := client.SetServer(target); err != nil {
		_ = client.Close()
		return nil, err
	}
	load, err := strconv.Atoi(desc.MediaFile)
	if err != nil {
		load = DefaultLoad
	}
	client.SetPayload(server.PayloadSize)
	return &loadSession{
		client: client,
		load:   uint64(load),
	}, nil
}

func (session *loadSession) Tasks() []golib.Task {
	return nil
}

func (session *loadSession) Start(base *protocols.SessionBase) {
	session.SessionBase = base
	session.client.StartLoad(session.load)
	log.Println("Sending Load to", session.client.Server())
}

func (session *loadSession) Cleanup() {
	session.CleanupErr = session.client.Close()
	log.Println("Stopped Load to", session.client.Server())
}
