#!/bin/bash

# Exit if not run without reqd args
if [ $# -eq 0 ]; then
    echo "Error: No arguments provided."
    echo "Usage: $0 <host1_ip> <host2_ip>"
    echo "/tmp/ssh_setup.sh 10.244.0.34 10.244.0.52"
    exit 1
fi

set -x 
mkdir -p /root/.ssh && \
cd /root/.ssh &&  \
yes | ssh-keygen -t rsa -b 4096 -N "" -f ~/.ssh/id_rsa && \
touch ~/.ssh/authorized_keys && \
chmod 600 ~/.ssh/authorized_keys && \
sshpass -p "docker" ssh-copy-id -o StrictHostKeyChecking=no root@${1}
sshpass -p "docker" ssh-copy-id -o StrictHostKeyChecking=no root@${2}
echo " !! Success with ssh setup  !!!" && \
set +x
