#! /bin/bash
set -e

# set glide.lock path
if [[ "$GLIDE" == "" ]]; then
	GLIDE=$GOPATH/src/github.com/tendermint/tendermint/glide.lock
fi

# get vendored commit for given lib

####################
# libs we depend on
####################

# some libs are tested with go, others with make
# TODO: should be all make (post repo merge)
LIBS_GO_TEST=(tmlibs/clist tmlibs/common go-config go-crypto tmlibs/db tmlibs/events go-merkle tendermint/p2p)
LIBS_MAKE_TEST=(tendermint/rpc go-wire abci)

for lib in "${LIBS_GO_TEST[@]}"; do

	# checkout vendored version of lib
	bash scripts/glide/checkout.sh "$GLIDE" "$lib"

	echo "Testing $lib ..."
	go test -v --race "github.com/tendermint/$lib/..."
	if [[ "$?" != 0 ]]; then
		echo "FAIL"
		exit 1
	fi
done

DIR=$(pwd)
for lib in "${LIBS_MAKE_TEST[@]}"; do

	# checkout vendored version of lib
	bash scripts/glide/checkout.sh "$GLIDE" "$lib"

	echo "Testing $lib ..."
	cd "$GOPATH/src/github.com/tendermint/$lib"
	make test
	if [[ "$?" != 0 ]]; then
		echo "FAIL"
		exit 1
	fi
	cd "$DIR"
done

echo ""
echo "PASS"
