package helpers

import (
	"bytes"
	"fmt"
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

type MultiError []error

func (err MultiError) NilOrError() error {
	if len(err) == 0 {
		return nil
	}
	return err
}

func (err *MultiError) Add(errOrNil error) {
	if err != nil && errOrNil != nil {
		*err = append(*err, errOrNil)
	}
}

func (err MultiError) Error() string {
	switch len(err) {
	case 0:
		return "No error"
	case 1:
		return err[0].Error()
	default:
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "Multiple errors:\n")
		for i, e := range err {
			fmt.Fprintf(&buf, "\t%v. %v\n", i+1, e)
		}
		return buf.String()
	}
}
