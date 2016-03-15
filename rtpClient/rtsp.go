package rtpClient

import (
	"strconv"

	"github.com/antongulenko/golib"
)

const (
	rtsp_exe    = "/home/anton/software/live555/testProgs/openRTSP"
	logfile_dir = "openRTSP-logs"
)

func StartRtspClient(rtspUrl string, port int, logfile string) (*golib.Command, error) {
	rtsp_params := []string{"-v", "-r", "-p", strconv.Itoa(port), rtspUrl}
	return golib.StartCommand(rtsp_exe, rtsp_params, "openRTSP", logfile_dir, logfile)
}
