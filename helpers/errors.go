package helpers

import "log"

func Checkerr(err error) {
	if err != nil {
		log.Fatalln("Error:", err)
	}
}

func Printerr(err error) {
	if err != nil {
		log.Println("Error: ", err)
	}
}
