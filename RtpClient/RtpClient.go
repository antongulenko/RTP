package main

import (
	"fmt"
	"github.com/cloudvm/gortp"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var _ = time.Sleep
var _ = syscall.SIGABRT
var _ = signal.Notify

const (
	rtp_port = 9000
	rtp_ip   = "127.0.0.1"
	rtsp_exe = "/home/anton/software/live555/testProgs/openRTSP"
	rtsp_url = "rtsp://127.0.1.1:8554/Sample.264"
)

var rtsp_params = []string{"-v", "-r", "-p", strconv.Itoa(rtp_port), rtsp_url}

func checkerr(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func printCtrlEvents(input rtp.CtrlEventChan, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for signals := range input {
			for _, signal := range signals {
				fmt.Printf("Ctrl event (type %v): %s\n", signal.EventType, signal.Reason)
			}
		}
	}()
}

func printDataPackets(input rtp.DataReceiveChan, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for data := range input {
			fmt.Printf("Data nr. %v (length %v)\n", data.Sequence(), data.InUse())
		}
	}()
}

func waitForInterrupt() {
	// This must be done after starting the openRTSP subprocess.
	// Otherwise openRTSP will not inherit the ignore-handler for
	// the SIGINT signal provided by the external ./noint program.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	signal.Stop(c)
}

func startRTSP() *exec.Cmd {
	fmt.Printf("Running %s %s\n", rtsp_exe, rtsp_params)
	output, err := os.OpenFile("openRTSP.log", os.O_CREATE|os.O_RDWR, 0664)
	checkerr(err)
	cmd := exec.Command(rtsp_exe, rtsp_params...)
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.Start()
	return cmd
}

func stopRTSP(cmd *exec.Cmd) {
	cmd.Process.Signal(syscall.SIGHUP)
}

func main() {
	localAddr, err := net.ResolveIPAddr("ip", rtp_ip)
	checkerr(err)
	rtpTransport, err := rtp.NewTransportUDP(localAddr, rtp_port)
	checkerr(err)
	session := rtp.NewSession(rtpTransport, rtpTransport)

	var wg sync.WaitGroup
	ctrlChan := session.CreateCtrlEventChan()
	dataChan := session.CreateDataReceiveChan()
	printCtrlEvents(ctrlChan, &wg)
	printDataPackets(dataChan, &wg)

	err = session.StartSession()
	checkerr(err)
	fmt.Printf("Listening on %v UDP ports %v and %v for rtp/rtcp\n", rtp_ip, rtp_port, rtp_port+1)

	cmd := startRTSP()
	pid := strconv.Itoa(cmd.Process.Pid)
	fmt.Println("STARTED ", pid)

	fmt.Println("Press Ctrl-C to interrupt")
	waitForInterrupt()
	session.CloseSession()
	fmt.Println("Stopping...")
	close(dataChan)
	close(ctrlChan)
	stopRTSP(cmd)
	if err = cmd.Wait(); err != nil {
		fmt.Println("openRTSP exit:", err)
	}
	wg.Wait()
}
