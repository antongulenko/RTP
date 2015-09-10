
.SILENT:

noint: noint.c
	gcc $< -o $@

client: noint
	go install github.com/antongulenko/RTP/rtpExperiment \
		&& echo ./noint rtpExperiment \
		|| echo false	

amp: noint
	go install github.com/antongulenko/RTP/proxies/AmpProxy \
		&& echo ./noint AmpProxy \
		|| echo false

pcp: noint
	go install github.com/antongulenko/RTP/proxies/UdpPcp \
		&& echo UdpPcp \
		|| echo false

balancer: noint
	go install github.com/antongulenko/RTP/proxies/AmpBalancer \
		&& echo AmpBalancer \
		|| echo false
