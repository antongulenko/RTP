package balancer

import (
	"fmt"
	"net"
	"sort"
	"sync"

	"github.com/antongulenko/RTP/protocols"
)

type BackendServer struct {
	Addr           *net.UDPAddr
	LocalAddr      *net.UDPAddr
	Client         protocols.CircuitBreaker
	Sessions       map[*BalancingSession]bool
	Plugin         *BalancingPlugin
	BackupSessions uint
	Load           float64 // 1 per session + backup_session_weight per backup session
}

func (server *BackendServer) String() string {
	return fmt.Sprintf("%s BackendServer at %s", server.Client.Protocol().Name(), server.Addr)
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
	copy(slice[i:], slice[i+1:])
	*_slice = slice[:len(slice)-1]
}

func (server *BackendServer) registerSession(session *BalancingSession) {
	server.Sessions[session] = true
	server.Load++
	for _, backup := range session.BackupServers {
		backup.BackupSessions++
		backup.Load += backup_session_weight
	}
}

func (server *BackendServer) unregisterSession(session *BalancingSession) {
	delete(server.Sessions, session)
	server.Load--
	for _, backup := range session.BackupServers {
		backup.BackupSessions--
		backup.Load -= backup_session_weight
	}
}

type failoverResults struct {
	newServer *BackendServer
	session   *BalancingSession
	err       error
}

func (server *BackendServer) handleStateChanged() {
	if err := server.Client.Error(); err != nil {
		// Server fault detected!
		if len(server.Sessions) == 0 {
			server.Plugin.assertStarted()
			server.Plugin.Server.LogError(fmt.Errorf("Backend server %v is down, but no sessions are affected", server.Client))
			return
		}
		go func() {
			failoverChan := make(chan failoverResults, len(server.Sessions))
			var wg sync.WaitGroup
			wg.Add(len(server.Sessions))
			for session := range server.Sessions {
				go server.failoverSession(session, failoverChan, &wg)
			}
			go server.handleFinishedFailovers(failoverChan)
			wg.Wait()
			close(failoverChan)
		}()
	}
}

func (server *BackendServer) failoverSession(session *BalancingSession, failoverChan chan<- failoverResults, wg *sync.WaitGroup) {
	// Fencing: Stop the original node just to be sure.
	// TODO more reliable fencing.
	session.Handler.BackgroundStopRemote()
	if newServer, err := session.Handler.HandleServerFault(); err != nil {
		failoverChan <- failoverResults{nil, session, err}
	} else {
		failoverChan <- failoverResults{newServer, session, nil}
	}
	wg.Done()
}

func (server *BackendServer) handleFinishedFailovers(failoverChan <-chan failoverResults) {
	for failover := range failoverChan {
		newServer, session, failoverErr := failover.newServer, failover.session, failover.err
		if failoverErr == nil {
			// Remove session from old server
			server.Load--
			session.BackupServers.removeServer(server)
			delete(server.Sessions, session)

			// Add session to new server
			newServer.BackupSessions--
			newServer.Load += 1 - backup_session_weight
			newServer.Sessions[session] = true
			session.PrimaryServer = newServer

			session.LogServerError(fmt.Errorf("Session %v failed over to %v", session.Client, newServer))
		} else {
			// Failover failed - stop session
			err := fmt.Errorf("Could not handle server fault for session %v: %v", session.Client, failoverErr)
			session.LogServerError(err)
			session.failoverError = err
			_ = session.StopContainingSession() // Drop error
		}
	}
}
