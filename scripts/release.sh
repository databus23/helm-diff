#!/usr/bin/env bash
set -x

if [ ! -f bin/github-release ]; then
  OS=$(uname)
  curl -L https://github.com/aktau/github-release/releases/download/v0.7.2/$OS-amd64-github-release.tar.bz2 | tar -C bin/ -jvx --strip-components=3
fi

user=ksa-real
repo=helm-diff
tag=$1
commit=$2

bin/github-release release -u $user -r $repo -t $tag -c $commit -n $tag

for f in $(ls release); do
  bin/github-release upload -u $user -r $repo -t $tag -n $f -f release/$f
done
