package amp_balancer

import (
	"fmt"
	"log"
	"net"
	"sort"

	"github.com/antongulenko/RTP/protocols"
)

type BackendServer struct {
	Addr           *net.UDPAddr
	LocalAddr      *net.UDPAddr
	Client         protocols.CircuitBreaker
	Sessions       map[*balancingSession]bool
	AmpServer      *ExtendedAmpServer
	BackupSessions uint
	Load           float64 // 1 per session + backup_session_weight per backup session
}

type BackendServerSlice []*BackendServer

// Implement sort.Interface
func (slice BackendServerSlice) Len() int {
	return len(slice)
}
func (slice BackendServerSlice) Less(i, j int) bool {
	return slice[i].Load < slice[j].Load
}
func (slice BackendServerSlice) Swap(i, j int) {
	tmp := slice[i]
	slice[i], slice[j] = slice[j], tmp
}

func (slice BackendServerSlice) Sort() {
	sort.Sort(slice)
}

func (slice BackendServerSlice) pickServer(client string) (primary *BackendServer, backups BackendServerSlice) {
	// Lowest loaded server for the primary, next lowest loaded servers for backups
	sort.Sort(slice)
	backups = make(BackendServerSlice, 0, num_backup_servers)
	for i := 0; i < len(slice) && len(backups) < num_backup_servers; i++ {
		if server := slice[i]; server.Client.Online() {
			if primary == nil {
				primary = server
			} else {
				backups = append(backups, server)
			}
		}
	}
	return
}

func (_slice *BackendServerSlice) removeServer(toRemove *BackendServer) {
	slice := *_slice
	var i int
	var found bool
	var server *BackendServer
	for i, server = range slice {
		if toRemove == server {
			found = true
			break
		}
	}
	if !found {
		return
	}
	log.Printf("Before removeServer: ", slice)
	copy(slice[i:], slice[i+1:])
	*_slice = slice[:len(slice)-1]
	log.Printf("After removeServer: ", slice)
}

type failoverResults struct {
	newServer *BackendServer
	session   *balancingSession
}

func (server *BackendServer) registerSession(session *balancingSession) {
	server.Sessions[session] = true
	server.Load++
	for _, backup := range session.BackupServers {
		backup.BackupSessions++
		backup.Load += backup_session_weight
	}
}

func (server *BackendServer) unregisterSession(session *balancingSession) {
	delete(server.Sessions, session)
	server.Load--
	for _, backup := range session.BackupServers {
		backup.BackupSessions--
		backup.Load -= backup_session_weight
	}
}

func (server *BackendServer) handleStateChanged() {
	if err := server.Client.Error(); err != nil {
		// Server fault detected!
		if len(server.Sessions) == 0 {
			server.AmpServer.LogError(fmt.Errorf("Backend server %v is down, but no sessions are affected", server.Client))
			return
		}
		// TODO this channel is never closed and the reading routine never finishes. Leak!
		failoverChan := make(chan failoverResults, 3)
		for session := range server.Sessions {
			go server.failoverSession(session, failoverChan)
		}
		go server.handleFinishedFailovers(failoverChan)
	}
}

func (server *BackendServer) failoverSession(session *balancingSession, failoverChan chan<- failoverResults) {
	// Fencing: Stop the original node just to be sure.
	// TODO more reliable fencing.
	session.BackgroundStopRemote()
	if newServer, err := session.HandleServerFault(); err != nil {
		session.containingSession.server.LogError(fmt.Errorf("Could not handle server fault for session %v: %v", session.Client, err))
		failoverChan <- failoverResults{nil, session}
	} else {
		failoverChan <- failoverResults{newServer, session}
	}
}

func (server *BackendServer) handleFinishedFailovers(failoverChan <-chan failoverResults) {
	for failover := range failoverChan {
		newServer, session := failover.newServer, failover.session

		// Remove session from old server
		server.Load--
		session.BackupServers.removeServer(server)
		delete(server.Sessions, session)

		// Add session to new server
		if newServer != nil {
			newServer.BackupSessions--
			newServer.Load += 1 - backup_session_weight
			newServer.Sessions[session] = true
			session.Server = newServer
		}
	}
}
