package stats

import (
	"fmt"
	"github.com/zfjagann/golang-ring"
	"time"
)

var (
	IncomingPacketBuffer = 10
)

type Stats struct {
	timestamps      ring.Ring
	incomingPackets <-chan time.Time
	Packets         chan<- time.Time
}

func NewStats() *Stats {
	incomingPackets := make(chan time.Time, IncomingPacketBuffer)
	stats := &Stats{
		Packets:         incomingPackets,
		incomingPackets: incomingPackets,
	}
	go stats.addPackets()
	return stats
}

func (stats *Stats) addPackets() {
	for {
		if t, ok := <-stats.incomingPackets; ok {
			// TODO hack
			if stats.timestamps.Capacity() == ring.DefaultCapacity {
				stats.timestamps.SetCapacity(1000)
			} else if stats.timestamps.Size > 0 && stats.timestamps.Size == stats.timestamps.Capacity() {
				panic(fmt.Sprintf("Cannot add more elements at %v", stats.timestamps.Size))
			}
			stats.timestamps.Enqueue(t)
		} else {
			return
		}
	}
}

func (stats *Stats) Size() uint {
	return uint(stats.timestamps.Size)
}

func (stats *Stats) FlushPackets(secondsAge uint) {
	now := time.Now()
	timeout := now.Add(time.Duration(-secondsAge) * time.Second)
	for {
		peeked := stats.timestamps.Peek()
		if peeked == nil {
			break
		}
		stamp := peeked.(time.Time)
		if stamp.Before(timeout) {
			stats.timestamps.Dequeue()
		} else {
			break
		}
	}
}

func (stats *Stats) PrintStats(secondsPeriod uint, messagePrefix string) {
	stats.FlushPackets(secondsPeriod)
	numPackets := stats.Size()
	fmt.Printf("%s: packets per second: %v\n", messagePrefix, numPackets/secondsPeriod)
}

func (stats *Stats) LoopPrintStats(secondsTimeout, secondsPeriod uint, messagePrefix string) {
	for {
		time.Sleep(time.Duration(secondsTimeout) * time.Second)
		stats.PrintStats(secondsPeriod, messagePrefix)
	}
}
