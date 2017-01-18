#! /bin/bash
set -u

N=$1 # number of nodes
TESTNET=$2 # path to folder containing testnet info
CONFIG=$3 # path to folder containing `bins` and `run.sh` files

if [[ ! -f $CONFIG/bins ]]; then
	echo "config folder ($CONFIG) must contain bins file"
	exit 1
fi
if [[ ! -f $CONFIG/run.sh ]]; then
	echo "config folder ($CONFIG) must contain run.sh file"
	exit 1
fi

KEY=$HOME/.ssh/id_rsa

FLAGS="-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no"

N_=$((N-1)) # 0-based index

MACH_ROOT="$TESTNET/mach?"


# mkdir
terraforce ssh --user root --ssh-key $KEY  --machines "[0-$N_]" mkdir .tendermint

# copy over  genesis/priv_val
terraforce scp --user root --ssh-key $KEY  --iterative --machines "[0-$N_]" "$MACH_ROOT/priv_validator.json" .tendermint/priv_validator.json
terraforce scp --user root --ssh-key $KEY  --iterative --machines "[0-$N_]" "$MACH_ROOT/genesis.json" .tendermint/genesis.json

# copy the run script
terraforce scp --user root --ssh-key $KEY --machines "[0-$N_]" $CONFIG/run.sh run.sh 

# copy the binaries
while read line; do
	local_bin=$(eval echo $line)
  	remote_bin=$(basename $local_bin)
  	echo $local_bin
 	terraforce scp --user root --ssh-key $KEY --machines "[0-$N_]" $local_bin $remote_bin
 	terraforce ssh --user root --ssh-key $KEY --machines "[0-$N_]" chmod +x $remote_bin
done <$CONFIG/bins
