#! /bin/bash

function testExample() {
	N=$1
	INPUT=$2
	APP=$3

	echo "Example $N"
	$APP &> /dev/null &
	sleep 2
	tmsp-cli --verbose batch < $INPUT > "${INPUT}.out.new"
	killall "$APP" &> /dev/null

	pre=`shasum < "${INPUT}.out"`
	post=`shasum < "${INPUT}.out.new"`

	if [[ "$pre" != "$post" ]]; then
		echo "You broke the tutorial"
		exit 1
	fi

	rm "${INPUT}".out.new
}

testExample 1 tests/test_cli/ex1.tmsp dummy
testExample 2 tests/test_cli/ex2.tmsp counter

echo ""
echo "PASS"
