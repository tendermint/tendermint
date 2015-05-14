#!/bin/bash
# Run this as tmuser user
# This part is for installing go

if [ `whoami` == "root" ];
then
  echo "You should not run this script as root"
  exit 1
fi

USER=`whoami`
PWD=`pwd`

# get dependencies
# sudo apt-get install -y make screen gcc git mercurial libc6-dev pkg-config libgmp-dev

# install golang
cd /home/$USER
mkdir gocode
wget https://storage.googleapis.com/golang/go1.4.2.src.tar.gz
tar -xzvf go*.tar.gz
cd go/src
./make.bash
mkdir -p /home/$USER/go/src
echo 'export GOROOT=/home/$USER/go' >> /home/$USER/.bashrc
echo 'export GOPATH=/home/$USER/gocode' >> /home/$USER/.bashrc
echo 'export PATH=$PATH:$GOROOT/bin:$GOPATH/bin' >> /home/$USER/.bashrc
source /home/$USER/.bashrc
cd $PWD
