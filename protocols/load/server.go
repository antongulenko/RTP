package load

import (
	"fmt"

	"github.com/antongulenko/RTP/protocols"
	"github.com/antongulenko/RTP/stats"
)

type LoadStats struct {
	server *protocols.Server
	seq    uint

	Received *stats.Stats
	Missed   *stats.Stats

	Handler func(packet *LoadPacket)
}

func RegisterServer(server *protocols.Server) (*LoadStats, error) {
	if err := server.Protocol().CheckIncludesFragment(Protocol.Name()); err != nil {
		return nil, err
	}
	stats := &LoadStats{
		server:   server,
		Received: stats.NewStats("Received"),
		Missed:   stats.NewStats("Missed"),
	}
	err := server.RegisterHandlers(protocols.ServerHandlerMap{
		codeLoad: stats.handleLoad,
	})
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (stats *LoadStats) handleLoad(packet *protocols.Packet) *protocols.Packet {
	if load, ok := packet.Val.(*LoadPacket); ok {
		if handler := stats.Handler; handler != nil {
			handler(load)
		}
		stats.addPacket(load)
	} else {
		stats.server.LogError(fmt.Errorf("Received illegal value for LoadPacket: %v", packet.Val))
	}
	return nil
}

func (stats *LoadStats) addPacket(packet *LoadPacket) {
	stats.Received.AddNow(packet.Size())
	if stats.seq < packet.Seq {
		stats.Missed.AddPacketsNow(packet.Seq - stats.seq)
	} else if stats.seq > packet.Seq {
		stats.server.LogError(fmt.Errorf("Load sequence jump: %v -> %v", stats.seq, packet.Seq))
	}
	stats.seq = packet.Seq + 1
}
