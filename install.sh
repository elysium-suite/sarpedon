#!/bin/bash
if [ "$EUID" -ne 0 ]; then
  echo "You need to be root!"
  exit 1
fi

cat <<EOF
#############################################################

███████╗ █████╗ ██████╗ ██████╗ ███████╗██████╗  ██████╗ ███╗   ██╗
██╔════╝██╔══██╗██╔══██╗██╔══██╗██╔════╝██╔══██╗██╔═══██╗████╗  ██║
███████╗███████║██████╔╝██████╔╝█████╗  ██║  ██║██║   ██║██╔██╗ ██║
╚════██║██╔══██║██╔══██╗██╔═══╝ ██╔══╝  ██║  ██║██║   ██║██║╚██╗██║
███████║██║  ██║██║  ██║██║     ███████╗██████╔╝╚██████╔╝██║ ╚████║
╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝     ╚══════╝╚═════╝  ╚═════╝ ╚═╝  ╚═══╝

#############################################################
EOF

apt update

echo "[+] Installing golang..."
wget -O ~/go1.14.5.linux-amd64.tar.gz https://golang.org/dl/go1.14.5.linux-amd64.tar.gz
tar -C /usr/local -xzf ~/go1.14.5.linux-amd64.tar.gz
echo "export PATH=$PATH:/usr/local/go/bin" >>/etc/profile
source /etc/profile

echo "[+] Building sarpedon..."
go build

echo "[+] Installing mongoDB..."
apt-get install gnupg
wget -qO - https://www.mongodb.org/static/pgp/server-4.4.asc | sudo apt-key add -
echo "deb [ arch=amd64,arm64 ] https://repo.mongodb.org/apt/ubuntu focal/mongodb-org/4.4 multiverse" | sudo tee /etc/apt/sources.list.d/mongodb-org-4.4.list
apt-get update
apt-get install -y mongodb-org

echo "[+] Installation complete!"
