#! /bin/bash
set -u

N=$1 # number of nodes

N_=$((N-1))

# start
terraforce ssh --user root --ssh-key $HOME/.ssh/id_rsa --machines "[0-$N_]" SEEDS=$(terraform output seeds) bash run.sh
