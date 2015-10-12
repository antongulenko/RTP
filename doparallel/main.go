package main

// Small program to execute a shell command multiple times in parallel

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

var (
	print_output = false
)

func main() {
	command := flag.String("cmd", "", "Program to execute in parallel")
	args := flag.String("args", "", "Parameters to pass to the program. 'XXX' substrings will be replaced with a number")
	times := flag.Uint("t", 1, "Number of times to execute the command in parallel")
	flag.BoolVar(&print_output, "print", print_output, "Print output of processes when they finish")
	flag.Parse()
	if *command == "" {
		log.Fatalln("cmd parameter required")
	}
	var commands []*exec.Cmd
	var wg sync.WaitGroup
	var i uint
	for i = 0; i < *times; i++ {
		realargs := strings.Replace(*args, "XXX", strconv.Itoa(int(i)), -1)
		splitargs := strings.Split(realargs, " ")
		cmd := exec.Command(*command, splitargs...)
		commands = append(commands, cmd)
		wg.Add(1)
		go observe(i, cmd, &wg)
	}
	wg.Wait()
}

func observe(num uint, cmd *exec.Cmd, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println("Executing process", num)
	out, err := cmd.Output()
	if err == nil {
		log.Printf("Process %v finished successfully\n", num)
	} else {
		log.Printf("Process %v finished: %v\n", num, err)
	}
	if print_output {
		var buf bytes.Buffer
		_, _ = buf.WriteString(fmt.Sprintf("============ Output of Process %v\n", num))
		buf.Write(out)
		buf.WriteString("==================================\n")
		fmt.Printf("%s", buf.String())
	}
}
