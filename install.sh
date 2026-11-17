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

echo "[+] Updating package list..."
apt update

echo "[+] Installing Go..."
wget https://go.dev/dl/go1.17.5.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.17.5.linux-amd64.tar.gz
echo "export PATH=$PATH:/usr/local/go/bin" >> /etc/profile
source /etc/profile

echo "[+] Building sarpedon..."
go build

echo "[+] Installing mongoDB..."
apt-get install -y mongodb

echo "[+] Installation complete! Place your config in ./sarpedon.conf and run ./sarpedon."
