#!/bin/bash

apt-get update
apt-get -y upgrade
apt-get -y install make screen

GOFILE=go1.7.linux-amd64.tar.gz

wget https://storage.googleapis.com/golang/${GOFILE}
tar xzf ${GOFILE}
mv go /usr/local/go1.7
ln -s /usr/local/go1.7 /usr/local/go
