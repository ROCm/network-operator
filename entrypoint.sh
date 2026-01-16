#!/usr/bin/env bash

set -x 
set -euo pipefail
dir=/usr/src/github.com/pensando/network-operator
externaldir=/usr/src/github.com/ROCm/network-operator
netns=/var/run/netns

term() {
    killall dockerd
    wait
}

PATH=/usr/local/go/bin:$PATH

dockerd -s vfs &

trap term INT TERM

mkdir -p ${dir}
mkdir -p ${externaldir}
mkdir -p ${netns}
mount -o bind /network-operator ${dir}
mount -o bind /network-operator ${externaldir}
rm -f $dir/.container_ready
export GOFLAGS=-mod=vendor
sysctl -w vm.max_map_count=262144

touch $dir/.container_ready
exec "$@"
