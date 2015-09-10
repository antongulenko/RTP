package stats

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

var (
	IncomingPacketChanBuffer = 50
)

type packet struct {
	timestamp time.Time
	bytes     uint
}

type Stats struct {
	packets         *list.List
	incomingPackets chan packet
	runningAverage  bool
	startOnce       sync.Once
	totalPackets    uint
	totalBytes      uint
	startTimestamp  time.Time
	name            string
}

func NewStats(name string) *Stats {
	stats := &Stats{
		name:           name,
		packets:        list.New(),
		startTimestamp: time.Now(),
	}
	return stats
}

func (stats *Stats) Start() {
	stats.startOnce.Do(func() {
		stats.runningAverage = true
		stats.incomingPackets = make(chan packet, IncomingPacketChanBuffer)
		go stats.addPackets()
	})
}

func (stats *Stats) Stop() {
	if stats.runningAverage {
		close(stats.incomingPackets)
	}
}

func (stats *Stats) Packets() uint {
	return stats.totalPackets
}

func (stats *Stats) Bytes() uint {
	return stats.totalBytes
}

func (stats *Stats) perSecond(value uint) float32 {
	sec := stats.storedSeconds()
	if sec == 0 {
		return 0
	}
	return float32(value) / sec
}

func (stats *Stats) PacketsPerSecond() float32 {
	var packets uint
	if stats.runningAverage {
		packets = uint(stats.packets.Len())
	} else {
		packets = stats.totalPackets
	}
	return stats.perSecond(packets)
}

func (stats *Stats) BytesPerSecond() float32 {
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

func (stats *Stats) Add(t time.Time, bytes uint) {
	stats.totalPackets++
	stats.totalBytes += bytes
	if stats.runningAverage {
		stats.incomingPackets <- packet{t, bytes}
	}
}

func (stats *Stats) AddNow(bytes uint) {
	stats.Add(time.Now(), bytes)
}

func (stats *Stats) AddPacket(t time.Time) {
	stats.Add(t, 0)
}

func (stats *Stats) AddPackets(t time.Time, num uint) {
	for i := uint(0); i < num; i++ {
		stats.AddPacket(t)
	}
}

func (stats *Stats) AddPacketNow() {
	stats.Add(time.Now(), 0)
}

func (stats *Stats) AddPacketsNow(num uint) {
	stats.AddPackets(time.Now(), num)
}

func (stats *Stats) Flush(secondsAge uint) {
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

func (stats *Stats) String() string {
	ps := fmt.Sprintf("%s: packets/s: %v (%v total)", stats.name, stats.PacketsPerSecond(), stats.totalPackets)
	if stats.totalBytes > 0 {
		ps += fmt.Sprintf(", byte/s: %v (%v total)", stats.BytesPerSecond(), stats.totalBytes)
	}
	return ps
}

func PrintStats(secondsPeriod uint, stats []*Stats) {
	fmt.Println("==============")
	for _, stats := range stats {
		stats.Flush(secondsPeriod)
		fmt.Println(stats.String())
	}
}

func (stats *Stats) storedSeconds() float32 {
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

func (stats *Stats) addPackets() {
	for p := range stats.incomingPackets {
		stats.packets.PushBack(p)
	}
}
