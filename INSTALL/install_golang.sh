#!/bin/bash
# Run this as tmuser user
# This part is for installing go

if [ `whoami` != "root" ];
then
  cd /home/tmuser
  mkdir gocode
  wget https://storage.googleapis.com/golang/go1.4.2.src.tar.gz
  tar -xzvf go*.tar.gz
  cd go/src
  ./make.bash
  cd /home/tmuser
  cp /etc/skel/.bashrc .
  mkdir -p /home/tmuser/go/src
  echo 'export GOROOT=/home/tmuser/go' >> /home/tmuser/.bashrc
  echo 'export GOPATH=/home/tmuser/gocode' >> /home/tmuser/.bashrc
  echo 'export PATH=$PATH:$GOROOT/bin:$GOPATH/bin' >> /home/tmuser/.bashrc
else
  echo "should not be root to run install_golang.sh"
fi
