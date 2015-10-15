package helpers

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"reflect"
	"sync"
)

// ========= Observee interface

type Observee interface {
	Observe(wg *sync.WaitGroup) <-chan interface{}
	Stop()
}

type NoopObservee struct {
	Chan        <-chan interface{}
	Description string
}

func (obs *NoopObservee) Observe(*sync.WaitGroup) <-chan interface{} {
	return obs.Chan
}
func (obs *NoopObservee) Stop() {
}
func (obs *NoopObservee) String() string {
	return fmt.Sprintf("Observee(%v)", obs.Description)
}

type cleanupObservee struct {
	cleanup func()
	once    sync.Once
}

func CleanupObservee(cleanup func()) Observee {
	return &cleanupObservee{
		cleanup: cleanup,
	}
}
func (obs *cleanupObservee) Observe(*sync.WaitGroup) <-chan interface{} {
	return make(chan interface{}, 1) // Never triggered
}
func (obs *cleanupObservee) Stop() {
	obs.once.Do(func() {
		obs.cleanup()
	})
}

func LoopObservee(loop func()) Observee {
	cond := NewOneshotCondition()
	go func() {
		for !cond.Enabled() {
			loop()
		}
	}()
	return cond
}

// ========= Helpers to implement Observee

func Observe(wg *sync.WaitGroup, wait func()) <-chan interface{} {
	if wg != nil {
		wg.Add(1)
	}
	finished := make(chan interface{}, 1)
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		if wait != nil {
			wait()
		}
		finished <- nil
		close(finished)
	}()
	return finished
}

func ObserveCondition(wg *sync.WaitGroup, cond *OneshotCondition) <-chan interface{} {
	if cond == nil {
		return nil
	}
	return Observe(wg, func() {
		cond.Wait()
	})
}

func WaitForAny(channels []<-chan interface{}) int {
	if len(channels) < 1 {
		return -1
	}
	// Use reflect package to wait for any of the given channels
	var cases []reflect.SelectCase
	for _, ch := range channels {
		if ch != nil {
			refCase := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
			cases = append(cases, refCase)
		}
	}
	choice, _, _ := reflect.Select(cases)
	return choice
}

func WaitForAnyObservee(wg *sync.WaitGroup, observees []Observee) Observee {
	channels := make([]<-chan interface{}, 0, len(observees))
	for _, observee := range observees {
		if channel := observee.Observe(wg); channel != nil {
			channels = append(channels, channel)
		}
	}
	choice := WaitForAny(channels)
	return observees[choice]
}

// ========= Observee Group

type ObserveeGroup struct {
	names  []string              // Track order of added new groups
	groups map[string][]Observee // Groups will be stopped sequentially, but Observees in one group in parallel
	all    []Observee
}

func NewObserveeGroup(observees ...Observee) *ObserveeGroup {
	group := &ObserveeGroup{
		groups: make(map[string][]Observee),
	}
	for _, o := range observees {
		group.Add(o)
	}
	return group
}

func (group *ObserveeGroup) Add(observee ...Observee) {
	group.AddNamed("default", observee...)
}

func (group *ObserveeGroup) AddNamed(name string, observee ...Observee) {
	if list, ok := group.groups[name]; ok {
		group.groups[name] = append(list, observee...)
	} else {
		group.groups[name] = observee
		group.names = append(group.names, name)
	}
	group.all = append(group.all, observee...)
}

func (group *ObserveeGroup) WaitForAny(wg *sync.WaitGroup) Observee {
	return WaitForAnyObservee(wg, group.all)
}

func (group *ObserveeGroup) ReverseStop() {
	for i := len(group.names) - 1; i >= 0; i-- {
		// Stop groups in reverse order
		var wg sync.WaitGroup
		observees := group.groups[group.names[i]]
		for _, observee := range observees {
			// Stop observees in one group in parallel
			wg.Add(1)
			go func(observee Observee) {
				defer wg.Done()
				observee.Stop()
			}(observee)
		}
		wg.Wait()
	}
}

func (group *ObserveeGroup) WaitAndStop(wg *sync.WaitGroup) Observee {
	choice := group.WaitForAny(wg)
	group.ReverseStop()
	return choice
}

// ========= Sources of interrupts by the user

func ExternalInterrupt() <-chan interface{} {
	// This must be done after starting any openRTSP subprocess that depensd
	// the ignore-handler for SIGNIT provided by ./noint
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	stop := make(chan interface{})
	go func() {
		defer signal.Stop(interrupt)
		<-interrupt
		stop <- nil
	}()
	return stop
}

func UserInput() <-chan interface{} {
	userinput := make(chan interface{}, 1)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		_, err := reader.ReadString('\n')
		if err != nil {
			log.Println("Error reading user input:", err)
		}
		userinput <- nil
	}()
	return userinput
}

func StdinClosed() <-chan interface{} {
	closed := make(chan interface{}, 1)
	go func() {
		_, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Println("Error reading stdin:", err)
		}
		closed <- nil
	}()
	return closed
}
