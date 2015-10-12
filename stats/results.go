package stats

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/antongulenko/RTP/helpers"
)

var (
	IncomingPacketChanBuffer = 50
)

type packet struct {
	timestamp time.Time
	bytes     uint
}

type Results struct {
	packets        *list.List
	startTimestamp time.Time

	runningAverage  bool
	startOnce       sync.Once
	stopped         *helpers.OneshotCondition
	incomingPackets chan packet

	totalPackets uint
	totalBytes   uint
}

func NewResults() *Results {
	return &Results{
		packets:        list.New(),
		startTimestamp: time.Now(),
		stopped:        helpers.NewOneshotCondition(),
	}
}

func (stats *Results) start() {
	stats.runningAverage = true
	stats.incomingPackets = make(chan packet, IncomingPacketChanBuffer)
	go stats.addPackets()
}

func (stats *Results) addPackets() {
	for p := range stats.incomingPackets {
		stats.packets.PushBack(p)
	}
}

func (stats *Results) stop() {
	stats.stopped.Enable(func() {
		if stats.runningAverage {
			close(stats.incomingPackets)
		}
	})
}

func (stats *Results) add(t time.Time, bytes uint) {
	stats.stopped.IfNotEnabled(func() {
		stats.totalPackets++
		stats.totalBytes += bytes
		if stats.runningAverage {
			stats.incomingPackets <- packet{t, bytes}
		}
	})
}

func (stats *Results) Flush(secondsAge uint) {
	timeout := time.Now().Add(time.Duration(-secondsAge) * time.Second)
	for {
		peeked := stats.packets.Front()
		if peeked == nil {
			break
		}
		p := peeked.Value.(packet)
		if p.timestamp.Before(timeout) {
			stats.packets.Remove(peeked)
		} else {
			break
		}
	}
}

func (stats *Results) Packets() uint {
	return stats.totalPackets
}

func (stats *Results) Bytes() uint {
	return stats.totalBytes
}

func (stats *Results) PacketsPerSecond() float32 {
	var packets uint
	if stats.runningAverage {
		packets = uint(stats.packets.Len())
	} else {
		packets = stats.totalPackets
	}
	return stats.perSecond(packets)
}

func (stats *Results) BytesPerSecond() float32 {
	var bytes uint
	if stats.runningAverage {
		for e := stats.packets.Front(); e != nil; e = e.Next() {
			p := e.Value.(packet)
			bytes += p.bytes
		}
	} else {
		bytes = stats.totalBytes
	}
	return stats.perSecond(bytes)
}

func (stats *Results) perSecond(value uint) float32 {
	sec := stats.storedSeconds()
	if sec == 0 {
		return 0
	}
	return float32(value) / sec
}

func (stats *Results) storedSeconds() float32 {
	var timestamp time.Time
	if stats.runningAverage {
		peek := stats.packets.Front()
		if peek == nil {
			return 0
		}
		oldestPacket := peek.Value.(packet)
		timestamp = oldestPacket.timestamp
	} else {
		timestamp = stats.startTimestamp
	}
	return float32(time.Now().Sub(timestamp)) / float32(time.Second)
}

func (stats *Results) String() string {
	ps := fmt.Sprintf("packets/s: %v (%v total)", stats.PacketsPerSecond(), stats.totalPackets)
	if stats.totalBytes > 0 {
		ps += fmt.Sprintf(", %v/s (%v total)", formatBytes(stats.BytesPerSecond()), formatBytes(float32(stats.totalBytes)))
	}
	return ps
}

func formatBytes(bytes float32) string {
	if bytes < 1024 {
		return fmt.Sprintf("%.1f B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f kB", bytes/(1024))
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", bytes/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", bytes/(1024*1024*1024))
}
