#!/bin/bash
HOME=/home/ubuntu
hosts="$1"
test -n "$hosts" || hosts=all
parallel-ssh -i -h "$HOME/scripts/$hosts" /home/ubuntu/scripts/restart.sh
