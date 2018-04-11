#!/usr/bin/env sh

##
## Input parameters
##
BINARY=/tendermint/${BINARY:-tendermint}
ID=${ID:-0}
LOG=${LOG:-tendermint.log}

##
## Assert linux binary
##
if ! [ -f "${BINARY}" ]; then
	echo "The binary `basename ${BINARY}` cannot be found. Please add the binary to the shared folder. E.g.: -e BINARY=tendermint_my_test_version"
	exit 1
fi
BINARY_CHECK="`file $BINARY | grep 'ELF 64-bit LSB executable, x86-64'`"
if [ -z "${BINARY_CHECK}" ]; then
	echo "Binary needs to be OS linux, ARCH amd64"
	exit 1
fi

##
## Assert network IP last octet is {$ID+2}
##
if [ -n "${ID}" ]; then
  IPLAST="`ifconfig eth0 | grep inet | cut -d\. -f4 | cut -d\  -f1`"
  if [ -z "${IPLAST}" ]; then
    echo "Could not parse IP address."
    exit 1
  fi
fi
IPID=$((ID + 2))
if [ "${IPID}" -ne "${IPLAST}" ]; then
  echo "Network ID last octet '$IPLAST' should be '$IPID'. Are you running another testnet on the docker network?"
  exit 1
fi


##
## Run binary with all parameters
##
export TMHOME="/tendermint/mach${ID}"

if [ "$LOG" == "stdout" ]; then
  "$BINARY" $@
else
  "$BINARY" $@ > "${TMHOME}/${LOG}"
fi

