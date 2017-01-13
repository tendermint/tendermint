# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/trusty64"

  config.vm.provider "virtualbox" do |v|
    v.memory = 2048
    v.cpus = 2
  end

  config.vm.provision "shell", inline: <<-SHELL
    apt-get update
    apt-get install -y --no-install-recommends wget curl jq shellcheck bsdmainutils psmisc

    wget -qO- https://get.docker.com/ | sh
    usermod -a -G docker vagrant

    curl -O https://storage.googleapis.com/golang/go1.6.linux-amd64.tar.gz
    tar -xvf go1.7.linux-amd64.tar.gz
    mv go /usr/local
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /home/vagrant/.profile
    mkdir -p /home/vagrant/go/bin
    chown -R vagrant:vagrant /home/vagrant/go
    echo 'export GOPATH=/home/vagrant/go' >> /home/vagrant/.profile

    mkdir -p /home/vagrant/go/src/github.com/tendermint
    ln -s /vagrant /home/vagrant/go/src/github.com/tendermint/tendermint

    su - vagrant -c 'curl https://glide.sh/get | sh'
    su - vagrant -c 'cd /vagrant/ && glide install && make test'
  SHELL
end
