cd ~
sudo tar -C /usr/local -xzf go1.17.8.linux-amd64.tar.gz
echo "export PATH=\$PATH:/usr/local/go/bin" >> .bashrc
source .bashrc
go version
go env -w  GOPROXY=https://goproxy.cn,direct
