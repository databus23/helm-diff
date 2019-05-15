#!/usr/bin/env bash

go get github.com/aktau/github-release

user=databus23
repo=helm-diff
tag=$1
commit=$2

github-release release -u $user -r $repo -t $tag -c $commit -n $tag

for f in $(ls release); do
  github-release upload -u $user -r $repo -t $tag -n $f -f release/$f
done
