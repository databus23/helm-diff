#!/usr/bin/env bash
set -x
apt-get update
apt-get install bzip2

if [ ! -f bin/github-release ]; then
  OS=$(uname)
  mkdir -p bin
  curl -L https://github.com/aktau/github-release/releases/download/v0.10.0/$OS-amd64-github-release.bz2 | bzcat >bin/github-release
  chmod +x bin/github-release
fi

user=databus23
repo=helm-diff
tag=$1
commit=$2

bin/github-release release -u $user -r $repo -t $tag -c $commit -n $tag

for f in $(ls release); do
  bin/github-release upload -u $user -r $repo -t $tag -n $f -f release/$f
done
