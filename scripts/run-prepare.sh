export HOME=/home/ubuntu
export GOMAXPROCS=16
type gvm &> /dev/null || { source $HOME/.bash_env; source $HOME/.bash_localrc; }
echo -e "\n============= $(date +"(%F %T)") Starting"
