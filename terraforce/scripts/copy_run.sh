#! /bin/bash
set -u

N=$1 # number of nodes
RUN=$2 # path to run script

N_=$((N-1))

# stop all tendermint
terraforce scp --user root --ssh-key $HOME/.ssh/id_rsa --machines "[0-$N_]" $RUN run.sh
