#! /bin/bash
set -e

#- dummy over socket, curl
#- counter over socket, curl
#- counter over grpc, curl
#- counter over grpc, grpc

# TODO: install everything

export TMROOT=$HOME/.tendermint_app

function dummy_over_socket(){
	rm -rf $TMROOT
	tendermint init
	echo "Starting dummy and tendermint"
	dummy > /dev/null &
	pid_dummy=$!
	tendermint node > tendermint.log &
	pid_tendermint=$!
	sleep 5

	echo "running test"
	bash dummy_test.sh "Dummy over Socket"

	kill -9 $pid_dummy $pid_tendermint
}

# start tendermint first
function dummy_over_socket_reorder(){
	rm -rf $TMROOT
	tendermint init
	echo "Starting tendermint and dummy"
	tendermint node > tendermint.log &
	pid_tendermint=$!
	sleep 2
	dummy > /dev/null &
	pid_dummy=$!
	sleep 5

	echo "running test"
	bash dummy_test.sh "Dummy over Socket"

	kill -9 $pid_dummy $pid_tendermint
}


function counter_over_socket() {
	rm -rf $TMROOT
	tendermint init
	echo "Starting counter and tendermint"
	counter --serial > /dev/null &
	pid_counter=$!
	tendermint node > tendermint.log &
	pid_tendermint=$!
	sleep 5

	echo "running test"
	bash counter_test.sh "Counter over Socket"

	kill -9 $pid_counter $pid_tendermint
}

function counter_over_grpc() {
	rm -rf $TMROOT
	tendermint init
	echo "Starting counter and tendermint"
	counter --serial --tmsp grpc > /dev/null &
	pid_counter=$!
	tendermint node --tmsp grpc > tendermint.log &
	pid_tendermint=$!
	sleep 5

	echo "running test"
	bash counter_test.sh "Counter over GRPC"

	kill -9 $pid_counter $pid_tendermint
}

function counter_over_grpc_grpc() {
	rm -rf $TMROOT
	tendermint init
	echo "Starting counter and tendermint"
	counter --serial --tmsp grpc > /dev/null &
	pid_counter=$!
	sleep 1
	GRPC_PORT=36656
	tendermint node --tmsp grpc --grpc_laddr tcp://localhost:$GRPC_PORT > tendermint.log &
	pid_tendermint=$!
	sleep 5

	echo "running test"
	GRPC_BROADCAST_TX=true bash counter_test.sh "Counter over GRPC via GRPC BroadcastTx"

	kill -9 $pid_counter $pid_tendermint
}

cd $GOPATH/src/github.com/tendermint/tendermint/test/app

case "$1" in 
	"dummy_over_socket")
		dummy_over_socket
		;;
	"dummy_over_socket_reorder")
		dummy_over_socket_reorder
		;;
	"counter_over_socket")
		counter_over_socket
		;;
	"counter_over_grpc")
		counter_over_grpc
		;;
	"counter_over_grpc_grpc")
		counter_over_grpc_grpc
		;;
	*)
		echo "Running all"
		dummy_over_socket
		echo ""
		dummy_over_socket_reorder
		echo ""
		counter_over_socket
		echo ""
		counter_over_grpc
		echo ""
		counter_over_grpc_grpc
esac

