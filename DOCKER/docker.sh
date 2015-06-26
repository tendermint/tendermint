#! /bin/bash

# don't build if you're impatient
if [[ ! $NO_BUILD ]]; then
	if [ `basename $(pwd)` = DOCKER ]; then
		docker build -t mint -f Dockerfile ..
	else
		docker build -t mint -f DOCKER/Dockerfile .
	fi
fi

# create the data-only container 
if [[ ! $VD ]]; then
	docker run --name mintdata --entrypoint /bin/echo mint Data-only container for mint
fi

# copy a directory from host to data-only volume
if [[ $VC ]]; then
	cd $VC
	tar cf - . | docker run -i --volumes-from mintdata mint tar xvf - -C /data/tendermint
fi

# run tendermint 
docker run --name mint --volumes-from mintdata -d -p 46656:46656 -p 46657:46657 -e FAST_SYNC=$FAST_SYNC mint
