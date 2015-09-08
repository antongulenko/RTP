package rtpClient

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
)

const (
	rtsp_exe    = "/home/anton/software/live555/testProgs/openRTSP"
	log_file    = "openRSTP.log"
	logfile_dir = "openRTSP-logs"
)

type RtspClient struct {
	Proc    *os.Process
	Logfile string

	State       *os.ProcessState
	StateString string
}

func openLogfile(filename string) (*os.File, error) {
	err := os.MkdirAll(logfile_dir, os.FileMode(0775))
	if err != nil {
		return nil, err
	}
	logfile, err := ioutil.TempFile(logfile_dir, filename)
	if err != nil {
		return nil, err
	}
	err = logfile.Truncate(0)
	if err != nil {
		return nil, err
	}
	return logfile, nil
}

func StartRtspClient(rtspUrl string, port int, logfile string) (*RtspClient, error) {
	rtsp_params := []string{"-v", "-r", "-p", strconv.Itoa(port), rtspUrl}

	cmd := exec.Command(rtsp_exe, rtsp_params...)
	if logfile != "" {
		logF, err := openLogfile(logfile)
		if err != nil {
			return nil, err
		}
		logfile = logF.Name()
		cmd.Stdout = logF
		cmd.Stderr = logF
	} else {
		cmd.Stdout = nil
		cmd.Stderr = nil
	}

	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	return &RtspClient{
		Proc:    cmd.Process,
		Logfile: logfile}, nil
}

func (client *RtspClient) Stop() {
	client.Proc.Signal(syscall.SIGHUP)
}

func (client *RtspClient) Observe(wg *sync.WaitGroup) <-chan interface{} {
	wg.Add(1)
	subprocessDied := make(chan interface{}, 1)
	go func() {
		defer wg.Done()
		state, err := client.Proc.Wait()
		client.State = state
		if !state.Success() {
			client.StateString = fmt.Sprintf("openRTSP (%v) exit: %s", client.Proc.Pid, state.String())
		} else if err != nil {
			client.StateString = fmt.Sprintf("openRTSP (%v) exit error: %s", client.Proc.Pid, err.Error())
		} else {
			client.StateString = fmt.Sprintf("openRTSP (%v) successful exit", client.Proc.Pid)
		}
		subprocessDied <- nil
	}()
	return subprocessDied
}
