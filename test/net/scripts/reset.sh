#! /bin/bash
set -u

N=$1 # number of nodes

N_=$((N-1))

# stop all tendermint
terraforce ssh --user root --ssh-key $HOME/.ssh/id_rsa --machines "[0-$N_]" rm -rf .tendermint/data
terraforce ssh --user root --ssh-key $HOME/.ssh/id_rsa --machines "[0-$N_]" ./tendermint unsafe_reset_priv_validator
