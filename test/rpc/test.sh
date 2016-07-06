#! /bin/bash
set -e

#- dummy over socket, curl
#- counter over socket, curl
#- counter over grpc, curl
#- counter over grpc, grpc

# TODO: install everything

function dummy_over_socket(){
	dummy > /dev/null &
	tendermint node > tendermint.log &
	sleep 3

	bash dummy_test.sh "Dummy over Socket"

	killall dummy tendermint
}


function counter_over_socket() {
	counter --serial > /dev/null &
	tendermint node > tendermint.log &
	sleep 3

	bash counter_test.sh "Counter over Socket"

	killall counter tendermint
}

function counter_over_grpc() {
	counter --serial --tmsp grpc > /dev/null &
	tendermint node --tmsp grpc > tendermint.log &
	sleep 3

	bash counter_test.sh "Counter over GRPC"

	killall counter tendermint
}

case "$1" in 
	"dummy_over_socket")
		dummy_over_socket
		;;
	"counter_over_socket")
		counter_over_socket
		;;
	"counter_over_grpc")
		counter_over_grpc
		;;
	*)
		dummy_over_socket
		counter_over_socket
		counter_over_grpc
esac

