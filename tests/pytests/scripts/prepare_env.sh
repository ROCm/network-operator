#!/usr/bin/bash

#
# Copyright (c) Advanced Micro Devices, Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the \"License\");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an \"AS IS\" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -x
SCRIPT_PATH=$(realpath ${BASH_SOURCE})
PKG_DIR=${PWD}

if [ "$#" -gt 2 ]; then
	echo "Usage: prepare_env.sh <path-to-venv>(optional)"
	echo "if <path-to-venv> is not supplied, ${PKG_DIR}/venv will be created"
	exit 2
fi

VENV=$1

if [ ! -d "${VENV}" ];
then
    echo "No venv folder available. Creating one in ${VENV}"
    mkdir -p ${VENV}
    python3 -m venv ${VENV}
fi

echo "Activating the venv ..."
source ${VENV}/bin/activate
pip install --upgrade pip
echo "Installing dependencies ..."
pip install -U -r $(dirname ${SCRIPT_PATH})/requirements.txt
echo "Exporting ${PKG_DIR} to PYTHONPATH"
export PYTHONPATH=${PKG_DIR}:${PYTHONPATH}

