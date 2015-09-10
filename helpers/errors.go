package helpers

import (
	"log"
	"os"
)

var (
	ExitHook func()
)

func Checkerr(err error) {
	if err != nil {
		log.Println("Error:", err)
		if ExitHook != nil {
			ExitHook()
		}
		os.Exit(1)
	}
}

func Printerr(err error) {
	if err != nil {
		log.Println("Error: ", err)
	}
}
