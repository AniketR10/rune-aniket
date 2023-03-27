#!/usr/bin/env bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

IMAGE_NAME=six_ssh_workspace_test
IMAGE_VERSION=latest

CONTAINER=$(docker run -d \
  -e TZ=America/Los_Angeles \
  -e PUBLIC_KEY_FILE="/id_ed25519.pub" \
  -e SUDO_ACCESS=true \
  -e USER_NAME=test \
  -e PUID=1000 \
  -e PGID=1000 \
  -ti \
  -P \
  --rm \
  "$IMAGE_NAME:$IMAGE_VERSION")

PORT=$(docker inspect -f '{{ (index (index .NetworkSettings.Ports "2222/tcp") 0).HostPort }}' $CONTAINER)

while ! ssh -q -o StrictHostKeyChecking=no -i $DIR/id_ed25519 test@127.0.0.1 -p $PORT which six > /dev/null
do
    sleep 1
done

echo "$CONTAINER:127.0.0.1:$PORT"
