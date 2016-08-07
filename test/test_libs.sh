#! /bin/bash

# set glide.lock path
if [[ "$GLIDE" == "" ]]; then
	GLIDE=$GOPATH/src/github.com/tendermint/tendermint/glide.lock
fi

# get vendored commit for given lib
function parseGlide() {
	cat $1 | grep -A1 $2 | grep -v $2 | awk '{print $2}'
}

# fetch and checkout vendored dep
function getDep() {
	lib=$1
	echo "----------------------------------"
	echo "Getting $lib ..."
	go get -t github.com/tendermint/$lib/...

	VENDORED=$(parseGlide $GLIDE $lib) 
	cd $GOPATH/src/github.com/tendermint/$lib
	MASTER=$(git rev-parse origin/master)

	if [[ "$VENDORED" != "$MASTER" ]]; then
		echo "... VENDORED != MASTER ($VENDORED != $MASTER)"
		echo "... Checking out commit $VENDORED"
		git checkout $VENDORED &> /dev/null
	fi
}

####################
# libs we depend on
####################

LIBS_GO_TEST=(go-clist go-common go-config go-crypto go-db go-events go-merkle go-p2p)
LIBS_MAKE_TEST=(go-rpc go-wire tmsp)

for lib in "${LIBS_GO_TEST[@]}"; do
	getDep $lib

	echo "Testing $lib ..."
	go test --race github.com/tendermint/$lib/...
	if [[ "$?" != 0 ]]; then
		echo "FAIL"
		exit 1
	fi
done


for lib in "${LIBS_MAKE_TEST[@]}"; do
	getDep $lib

	echo "Testing $lib ..."
	cd $GOPATH/src/github.com/tendermint/$lib
	make test
	if [[ "$?" != 0 ]]; then
		echo "FAIL"
		exit 1
	fi
done

echo ""
echo "PASS"
