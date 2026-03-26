#!/bin/sh
set -e
set -x

mkdir -p ~/.ssh
echo "$GIT_SSH_KEY" > ~/.ssh/id_rsa
chmod 600 ~/.ssh/id_rsa
chmod 700 ~/.ssh
ssh-keyscan -t rsa git.unstable.build >> ~/.ssh/known_hosts
ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts
git config --global url.ssh://git@github.com/.insteadOf https://github.com/
