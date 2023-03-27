#!/usr/bin/env bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
IMAGE_NAME=six_ssh_workspace_test

docker build -f $DIR/Dockerfile -t $IMAGE_NAME $DIR/../../../
