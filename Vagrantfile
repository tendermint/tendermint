# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/xenial64"

  config.vm.provider "virtualbox" do |v|
    v.memory = 4096
    v.cpus = 2
  end

  config.vm.provision "shell", inline: <<-SHELL
    # add docker repo
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
    add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu xenial stable"

    # and golang 1.9 support
    # official repo doesn't have race detection runtime...
    # add-apt-repository ppa:gophers/archive
    add-apt-repository ppa:longsleep/golang-backports

    # install base requirements
    apt-get update
    apt-get install -y --no-install-recommends wget curl jq \
        make shellcheck bsdmainutils psmisc
    apt-get install -y docker-ce golang-1.9-go

    # needed for docker
    usermod -a -G docker vagrant

    # use "EOF" not EOF to avoid variable substitution of $PATH
    cat << "EOF" >> /home/vagrant/.bash_profile
export PATH=$PATH:/usr/lib/go-1.9/bin:/home/vagrant/go/bin
export GOPATH=/home/vagrant/go
export LC_ALL=en_US.UTF-8
cd go/src/github.com/tendermint/tendermint
EOF

    mkdir -p /home/vagrant/go/bin
    mkdir -p /home/vagrant/go/src/github.com/tendermint
    ln -s /vagrant /home/vagrant/go/src/github.com/tendermint/tendermint

    chown -R vagrant:vagrant /home/vagrant/go
    chown vagrant:vagrant /home/vagrant/.bash_profile

    # get all deps and tools, ready to install/test
    su - vagrant -c 'cd /home/vagrant/go/src/github.com/tendermint/tendermint && make get_tools && make get_vendor_deps'
  SHELL
end
