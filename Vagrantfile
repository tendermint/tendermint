# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure("2") do |config|
  config.vm.box = "ubuntu/xenial64"

  config.vm.provider "virtualbox" do |v|
    v.memory = 4096
    v.cpus = 2
  end

  config.vm.provision "shell", inline: <<-SHELL
    apt-get update

    # install base requirements
    apt-get install -y --no-install-recommends wget curl jq zip \
        make shellcheck bsdmainutils psmisc
    apt-get install -y language-pack-en

    # install docker
    apt-get install -y --no-install-recommends apt-transport-https \
      ca-certificates curl software-properties-common
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
    add-apt-repository \
      "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
      $(lsb_release -cs) \
      stable"
    apt-get install -y docker-ce
    usermod -a -G docker vagrant

    # install go
    wget -q https://dl.google.com/go/go1.12.linux-amd64.tar.gz
    tar -xvf go1.12.linux-amd64.tar.gz
    mv go /usr/local
    rm -f go1.12.linux-amd64.tar.gz

    # install nodejs (for docs)
    curl -sL https://deb.nodesource.com/setup_11.x | bash -
    apt-get install -y nodejs

    # cleanup
    apt-get autoremove -y

    # set env variables
    echo 'export GOROOT=/usr/local/go' >> /home/vagrant/.bash_profile
    echo 'export GOPATH=/home/vagrant/go' >> /home/vagrant/.bash_profile
    echo 'export PATH=$PATH:$GOROOT/bin:$GOPATH/bin' >> /home/vagrant/.bash_profile
    echo 'export LC_ALL=en_US.UTF-8' >> /home/vagrant/.bash_profile
    echo 'cd go/src/github.com/tendermint/tendermint' >> /home/vagrant/.bash_profile

    mkdir -p /home/vagrant/go/bin
    mkdir -p /home/vagrant/go/src/github.com/tendermint
    ln -s /vagrant /home/vagrant/go/src/github.com/tendermint/tendermint

    chown -R vagrant:vagrant /home/vagrant/go
    chown vagrant:vagrant /home/vagrant/.bash_profile

    # get all deps and tools, ready to install/test
    su - vagrant  -c 'source /home/vagrant/.bash_profile'
    su - vagrant -c 'cd /home/vagrant/go/src/github.com/tendermint/tendermint && make tools'
  SHELL
end
