#! /bin/bash
set -e

# Get the root directory.
export PATH="$GOBIN:$PATH"
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/../.." && pwd )"

# Change into that dir because we expect that.
cd "$DIR" || exit

function testExample() {
	N=$1
	INPUT=$2
	APP="$3 $4"

	echo "Example $N: $APP"
	$APP &> /dev/null &
	sleep 2
	abci-cli --log_level=error --verbose batch < "$INPUT" > "${INPUT}.out.new"
	killall "$3"

	pre=$(shasum < "${INPUT}.out")
	post=$(shasum < "${INPUT}.out.new")

	if [[ "$pre" != "$post" ]]; then
		echo "You broke the tutorial"
		echo "Got:"
		cat "${INPUT}.out.new"
		echo "Expected:"
		cat "${INPUT}.out"
		echo "Diff:"
		diff "${INPUT}.out" "${INPUT}.out.new"
		exit 1
	fi

	rm "${INPUT}".out.new
}

testExample 1 tests/test_cli/ex1.abci abci-cli kvstore

echo ""
echo "PASS"
