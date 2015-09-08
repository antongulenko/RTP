package helpers

import (
	"bufio"
	"log"
	"os"
	"os/signal"
	"reflect"
)

func Checkerr(err error) {
	if err != nil {
		log.Fatalln("Error:", err)
	}
}

func WaitForAny(channels []<-chan interface{}) int {
	// Use reflect package to wait for any of the given channels
	cases := make([]reflect.SelectCase, len(channels))
	for i, ch := range channels {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	choice, _, _ := reflect.Select(cases)
	return choice
}

// This must be done after starting any openRTSP subprocess that depensd
// the ignore-handler for SIGNIT provided by ./noint
func ExternalInterrupt() <-chan interface{} {
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

func WaitForUserInput() {
	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadString('\n')
	Checkerr(err)
}
