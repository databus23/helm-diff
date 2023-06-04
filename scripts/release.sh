#!/usr/bin/env sh
set -e

# == would end up with: scripts/release.sh: 5: [: v3.8.1: unexpected operator
if [ "$1" = "" ]; then
  echo usage: "$0 VERSION"
fi

git tag $1
git push origin $1
gh release create $1 --draft --generate-notes --title "$1" release/*.tgz
