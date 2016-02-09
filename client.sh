noint rtpExperiment \
-amp_url=balancer:7777 \
-rtp=client-1 \
-pcp_url=proxy-1:7778 \
-proxy_port 9500 \
-rtsp_url rtsp://media-1:554/Sample.264 \
$@
