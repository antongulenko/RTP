package rtpClient

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

const (
	rtsp_exe = "/home/anton/software/live555/testProgs/openRTSP"
)

func (client *RtpClient) startRTSP(rtspUrl string, port int) error {
	rtsp_params := []string{"-v", "-r", "-p", strconv.Itoa(port), rtspUrl}

	output, err := os.OpenFile("openRTSP.log", os.O_CREATE|os.O_RDWR, 0664)
	if err != nil {
		return err
	}
	err = output.Truncate(0)
	if err != nil {
		return err
	}
	cmd := exec.Command(rtsp_exe, rtsp_params...)
	cmd.Stdout = output
	cmd.Stderr = output

	log.Printf("Running %s %s\n", rtsp_exe, rtsp_params)
	err = cmd.Start()
	client.rtspProc = cmd.Process
	if err != nil {
		return err
	}
	log.Printf("Pid: %v\n", cmd.Process.Pid)
	return nil
}

func (client *RtpClient) stopRTSP() {
	client.rtspProc.Signal(syscall.SIGHUP)
}

func (client *RtpClient) observeRTSP() {
	defer client.wg.Done()
	state, err := client.rtspProc.Wait()
	if !state.Success() {
		log.Println("openRTSP exit:", state.String())
	} else if err != nil {
		log.Println("openRTSP exit error:", err)
	}
	client.subprocessDied <- nil
}
