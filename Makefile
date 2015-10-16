 
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

pcp:
	go install github.com/antongulenko/RTP/proxies/UdpPcp \
		&& echo UdpPcp \
		|| echo false

balancer:
	go install github.com/antongulenko/RTP/proxies/AmpBalancer \
		&& echo AmpBalancer \
		|| echo false

latency:
	go install github.com/antongulenko/RTP/latency_test \
		&& echo latency_test \
		|| echo false

load:
	go install github.com/antongulenko/RTP/proxies/Load \
		&& echo Load \
		|| echo false
