package helpers

import (
	"bufio"
	"io/ioutil"
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
