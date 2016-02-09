#!/bin/bash
HOME=/home/ubuntu
hosts="$1"
test -n "$hosts" || hosts=all
parallel-ssh -i -p 1 -h "$HOME/scripts/$hosts" sudo tail "/var/log/upstart/anton-*.log"
