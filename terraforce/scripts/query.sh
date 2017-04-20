#! /bin/bash
set -u

N=$1 # number of nodes
QUERY=$2

N_=$((N-1))

# start all tendermint nodes
terraforce ssh --user root --ssh-key $HOME/.ssh/id_rsa --machines "[0-$N_]" curl -s localhost:46657/$QUERY

