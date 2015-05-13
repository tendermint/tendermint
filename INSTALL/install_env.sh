#!/bin/bash
# Run this as super user
# This part is for installing go language and setting up a user account
apt-get update -y
apt-get upgrade -y
apt-get install -y make screen gcc git mercurial libc6-dev pkg-config libgmp-dev
useradd tmuser -d /home/tmuser
usermod -aG sudo tmuser
mkdir /home/tmuser
chown -R tmuser:tmuser /home/tmuser
su tmuser
