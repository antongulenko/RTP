package helpers

import (
	"log"
	"os"
)

var (
	ExitHook func()
	exiting  bool
)

func Checkerr(err error) {
	if err != nil {
		if exiting {
			log.Println("Recursive Checkerr:", err)
			return
		}
		exiting = true
		log.Println("Fatal Error:", err)
		if ExitHook != nil {
			ExitHook()
		}
		os.Exit(1)
	}
}

func Printerr(err error) {
	if err != nil {
		log.Println("Error:", err)
	}
}
