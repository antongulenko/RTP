
.SILENT:

noint: noint.c
	gcc $< -o $@

client: noint
	go install github.com/antongulenko/RTP/RtpExperiment \
		&& echo ./noint RtpExperiment \
		|| echo false	
