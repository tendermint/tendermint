#!/bin/bash
# Run this as root user
# This part is for hardening the server and setting up a user account

if [ `whoami` != "root" ];
then
  echo "You must run this script as root"
  exit 1
fi

USER="tmuser"
ADMIN_EMAIL="ENTER_ADMIN_EMAIL"
OPEN_PORTS=(46656 46657 46658 46659 46660 46661 46662 46663 46664 46665 46666 46667 46668 46669 46670 46671)
SSH_PORT=20
SSH_CONFIG="Port 20
Protocol 2
HostKey /etc/ssh/ssh_host_rsa_key
HostKey /etc/ssh/ssh_host_dsa_key
HostKey /etc/ssh/ssh_host_ecdsa_key
HostKey /etc/ssh/ssh_host_ed25519_key
UsePrivilegeSeparation yes
KeyRegenerationInterval 3600
ServerKeyBits 1024
SyslogFacility AUTH
LogLevel INFO
LoginGraceTime 120
PermitRootLogin no
StrictModes yes
RSAAuthentication yes
PubkeyAuthentication yes
IgnoreRhosts yes
RhostsRSAAuthentication no
HostbasedAuthentication no
PermitEmptyPasswords no
ChallengeResponseAuthentication no
PasswordAuthentication no
"
WHITELIST=()

# update and upgrade
apt-get update -y
apt-get upgrade -y

# fail2ban for monitoring logins
apt-get install -y fail2ban

# set up the network time daemon
apt-get install -y ntp

# install dependencies
apt-get install -y make screen gcc git mercurial libc6-dev pkg-config libgmp-dev

# set up firewall
echo "ENABLE FIREWALL ..."
# copy in the ssh config with locked down settings
if [ "$SSH_CONFIG" != "" ]; then
  echo "$SSH_CONFIG" > /etc/ssh/sshd_config
  service ssh restart
else
  echo "Skipping over sshd_config rewrite"
fi
# white list ssh access 
for ip in "${WHITELIST[@]}"; do
	ufw allow from $ip to any port $SSH_PORT
done
if [ ${#WHITELIST[@]} -eq 0 ]; then
	ufw allow $SSH_PORT
fi
# open ports
for port in "${OPEN_PORTS[@]}"; do
	ufw allow $port
done
# apply
ufw enable

# watch the logs and have them emailed to me
apt-get install -y logwatch
echo "/usr/sbin/logwatch --output mail --mailto $ADMIN_EMAIL --detail high" >> /etc/cron.daily/00logwatch

# set up user account
echo "CREATE USER $USER ..."
useradd $USER -d /home/$USER
# This user should not have root access.
# usermod -aG sudo $USER
mkdir /home/$USER
cp /etc/skel/.bashrc .
cp /etc/skel/.profile .
chown -R $USER:$USER /home/$USER

echo "Done setting env.  Switching to $USER..."
su $USER
