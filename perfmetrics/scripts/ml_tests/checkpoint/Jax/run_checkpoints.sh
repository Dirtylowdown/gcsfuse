#!/bin/bash
# Copyright 2025 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Fail on any error.
set -e
set -x
echo "Running JAX checkpoint tests"

sudo apt-get update
# Install Git.
echo "Installing git"
sudo apt-get install git
# Install Golang.
wget -O go_tar.tar.gz https://go.dev/dl/go1.24.0.linux-amd64.tar.gz -q
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go_tar.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Clone and build the gcsfuse master branch.
git clone https://github.com/GoogleCloudPlatform/gcsfuse.git
cd gcsfuse
CGO_ENABLED=0 go build .
cd -

function mount_gcsfuse_and_run_test() {
  # Function to mount GCSFuse.
  # Input:
  #   $1: Bucket name

  local BUCKET_NAME="$1"
  # Create a directory for gcsfuse logs.
  mkdir -p "${KOKORO_ARTIFACTS_DIR}/gcsfuse_logs"
#  local GCSFUSE_FLAGS=--log-severity=TRACE --log-file=${KOKORO_ARTIFACTS_DIR}/gcsfuse_logs/${BUCKET_NAME}.log --enable-streaming-writes
  local MOUNT_POINT="${HOME}/gcs/${BUCKET_NAME}"
  mkdir -p "${MOUNT_POINT}"

  COMMON_FLAGS=(--log-severity=TRACE --enable-streaming-writes --log-file="${KOKORO_ARTIFACTS_DIR}"/gcsfuse_logs/"${BUCKET_NAME}".log)
  if [[ $BUCKET_NAME == "jax-emulated-checkpoint-flat" ]]
  then
    go run . "${COMMON_FLAGS[@]}" --rename-dir-limit=-1  "${BUCKET_NAME}" "${MOUNT_POINT}"
  else
    go run . "${COMMON_FLAGS[@]}" "${BUCKET_NAME}" "${MOUNT_POINT}"
  fi
  python3.11 ./perfmetrics/scripts/ml_tests/checkpoint/Jax/emulated_checkpoints.py --checkpoint_dir "${MOUNT_POINT}"
}

# Enable python virtual environment.
# By default KOKORO VM installs python 3.8 which causes dependency issues.
# Following commands are to explicitly set up python 3.11.
sudo apt update
sudo apt install -y software-properties-common
sudo add-apt-repository -y ppa:deadsnakes/ppa
sudo apt update
sudo apt install -y python3.11 python3.11-dev python3.11-venv
# Install pip
curl https://bootstrap.pypa.io/get-pip.py -o get-pip.py
python3.11 get-pip.py
rm get-pip.py
python3.11 -m venv .venv
source .venv/bin/activate
# Install JAX dependencies.
pip install -r ./perfmetrics/scripts/ml_tests/checkpoint/Jax/requirements.txt

# Run tests on legacy flat bucket.
FLAT_BUCKET_NAME="jax-emulated-checkpoint-flat"
mount_gcsfuse_and_run_test "${FLAT_BUCKET_NAME}"

# Run tests on hns bucket.
HNS_BUCKET_NAME="jax-emulated-checkpoint-hns"
mount_gcsfuse_and_run_test "${HNS_BUCKET_NAME}"

# Clean up buckets after run.
gcloud storage rm -r gs://FLAT_BUCKET_NAME/**
gcloud storage rm -r gs://HNS_BUCKET_NAME/**
