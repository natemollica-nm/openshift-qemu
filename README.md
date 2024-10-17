# OpenShift on QEMU CLI Tool

`openshift-qemu` CLI tool for deploying and managing libvirt enabled OpenShift clusters locally for 
testing and development.



## Prerequisites 

* Homebrew Installed on RHEL9 Host
* RHEL9 Host Machine Configured for Go Development

* `libvirt-libs` Package Installed
* `libvirt-devel` Package installed

**_Manual RPM Installation of the 10.5.x Versions (no 10.6.x currently available)_**
```shell
LIBVIRT_LIBS=https://rpmfind.net/linux/centos-stream/9-stream/AppStream/x86_64/os/Packages/libvirt-libs-10.5.0-5.el9.x86_64.rpm
curl -s "${LIBVIRT_LIBS}" -o libvirt-libs.rpm

LIBVIRT_RPM=https://rpmfind.net/linux/centos-stream/9-stream/CRB/x86_64/os/Packages/libvirt-devel-10.5.0-5.el9.x86_64.rpm
curl -s "$LIBVIRT_RPM" -o libvirt-devel.rpm
```

pkg-

* GO Variables configured
```shell
PKG_CONFIG_PATH=/home/linuxbrew/.linuxbrew/lib/pkgconfig:/usr/share/libvirt:/usr/lib64/pkgconfig
GOSUMDB=sum.golang.org
GOROOT=/home/linuxbrew/.linuxbrew/opt/go@1.22/libexec
GOPROXY=https://proxy.golang.org,direct
GOPATH=/home/ec2-user/go
```

#### Installing Homebrew

Download/Install
```shell
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

Update Brew to be sourced from login/user terminal
```shell
test -d ~/.linuxbrew && eval "$(~/.linuxbrew/bin/brew shellenv)"
test -d /home/linuxbrew/.linuxbrew && eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
echo "eval \"\$($(brew --prefix)/bin/brew shellenv)\"" >> ~/.bashrc
```