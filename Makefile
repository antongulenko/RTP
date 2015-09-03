
noint: noint.c
	gcc $< -o $@

client: noint
	go install github.com/antongulenko/RTP/RtpClient
	./noint RtpClient
