#!/bin/bash
set -e
HOME=/home/ubuntu
type gvm &> /dev/null || { source $HOME/.bash_env; source $HOME/.bash_localrc; }
if [ -s "$HOME/repos.txt" ]; then
    while IFS='' read -r line || [[ -n "$line" ]]; do
        cd "$line"
        git fetch origin
        git merge origin/master -m "merged origin/master"
        # Check if we are tracking a remote branch from 'origin'
        ( git bra -vv | grep '*' | grep '\[origin/.*: ' ) && git merge -m "merged remote"
    done < "$HOME/repos.txt"
fi
test -x "$HOME/build.sh" && "$HOME/build.sh"
if [ -s "$HOME/services.txt" ]; then
    while IFS='' read -r line || [[ -n "$line" ]]; do
        sudo initctl restart "$line" || sudo initctl start "$line"
    done < "$HOME/services.txt"
fi
