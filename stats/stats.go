package stats

import (
	"fmt"
	"time"
)

type Stats struct {
	Results *Results // Don't change after calling Start()
	Name    string
}

func NewStats(name string) *Stats {
	stats := &Stats{
		Name:    name,
		Results: NewResults(), // The default
	}
	return stats
}

func (stats *Stats) Start() {
	stats.Results.start()
}

func (stats *Stats) Stop() {
	stats.Results.stop()
}

func (stats *Stats) String() string {
	return fmt.Sprintf("%s: %s", stats.Name, stats.Results.String())
}

func (stats *Stats) AddNow(bytes uint) {
	stats.Results.add(time.Now(), bytes)
}

func (stats *Stats) AddPacket(t time.Time) {
	stats.Results.add(t, 0)
}

func (stats *Stats) AddPackets(t time.Time, num uint) {
	for i := uint(0); i < num; i++ {
		stats.AddPacket(t)
	}
}

func (stats *Stats) AddPacketNow() {
	stats.Results.add(time.Now(), 0)
}

func (stats *Stats) AddPacketsNow(num uint) {
	stats.AddPackets(time.Now(), num)
}
