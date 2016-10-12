#! /bin/bash

# for every github.com/tendermint dependency, warn is if its not synced with origin/master

if [[ "$GLIDE" == "" ]]; then
	GLIDE=$GOPATH/src/github.com/tendermint/tendermint/glide.lock
fi

# make list of libs
LIBS=($(grep "github.com/tendermint" $GLIDE  | awk '{print $3}'))


UPTODATE=true
for lib in "${LIBS[@]}"; do
	# get vendored commit
	VENDORED=`grep -A1 $lib $GLIDE | grep -v $lib | awk '{print $2}'`
	PWD=`pwd`
	cd $GOPATH/src/$lib
	MASTER=`git rev-parse origin/master`
	HEAD=`git rev-parse HEAD`
	cd $PWD
	
	if [[ "$VENDORED" != "$MASTER" ]]; then
		UPTODATE=false
		echo ""
		if [[ "$VENDORED" != "$HEAD" ]]; then
			echo "Vendored version of $lib differs from origin/master and HEAD"
			echo "Vendored: $VENDORED"
			echo "Master: $MASTER"
			echo "Head: $HEAD"
		else
			echo "Vendored version of $lib differs from origin/master but matches HEAD"
			echo "Vendored: $VENDORED"
			echo "Master: $MASTER"
		fi
	elif [[ "$VENDORED" != "$HEAD" ]]; then
			echo ""
			echo "Vendored version of $lib matches origin/master but differs from HEAD"
			echo "Vendored: $VENDORED"
			echo "Head: $HEAD"
	fi
done

if [[ "$UPTODATE" == "true" ]]; then
	echo "All vendored versions up to date"
fi

