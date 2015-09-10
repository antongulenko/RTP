package rtpClient

import (
	"strconv"

	"github.com/antongulenko/RTP/helpers"
)

const (
	rtsp_exe    = "/home/anton/software/live555/testProgs/openRTSP"
	logfile_dir = "openRTSP-logs"
)

func StartRtspClient(rtspUrl string, port int, logfile string) (*helpers.Command, error) {
	rtsp_params := []string{"-v", "-r", "-p", strconv.Itoa(port), rtspUrl}
	return helpers.StartCommand(rtsp_exe, rtsp_params, "openRTSP", logfile_dir, logfile)
}
