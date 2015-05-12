#!/bin/bash
# Run this as tmuser user
# This part is for installing Tendermint

if [ `whoami` != "root" ];
then
cd
source /home/tmuser/.bashrc
rm -rf $GOPATH/src/github.com/tendermint/tendermint
go get github.com/tendermint/tendermint

cd $GOPATH/src/github.com/tendermint/tendermint
git checkout develop
git pull 
make
else
echo "should not be root to run update_tendermint.sh"
fi

