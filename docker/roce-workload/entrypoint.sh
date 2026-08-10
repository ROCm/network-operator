#!/bin/bash
set -e

if [ ! -f /root/.ssh/id_rsa ]; then
  ssh-keygen -t rsa -b 4096 -N "" -f /root/.ssh/id_rsa
  cat /root/.ssh/id_rsa.pub >> /root/.ssh/authorized_keys
fi

### Start SSH daemon ----
LD_PRELOAD= /usr/sbin/sshd

if [ "$#" -eq 0 ]; then
  exec sleep infinity
fi

exec "$@"
