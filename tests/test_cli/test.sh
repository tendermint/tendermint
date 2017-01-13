#! /bin/bash

cd $GOPATH/src/github.com/tendermint/abci

function testExample() {
	N=$1
	INPUT=$2
	APP=$3

	echo "Example $N"
	$APP &> /dev/null &
	sleep 2
	abci-cli --verbose batch < $INPUT > "${INPUT}.out.new"
	killall "$APP" 

	pre=`shasum < "${INPUT}.out"`
	post=`shasum < "${INPUT}.out.new"`

	if [[ "$pre" != "$post" ]]; then
		echo "You broke the tutorial"
		echo "Got:" 
		cat "${INPUT}.out.new"
		echo "Expected:" 
		cat "${INPUT}.out"
		exit 1
	fi

	rm "${INPUT}".out.new
}

testExample 1 tests/test_cli/ex1.abci dummy
testExample 2 tests/test_cli/ex2.abci counter

echo ""
echo "PASS"
