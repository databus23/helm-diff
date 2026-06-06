#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 2 ]; then
  echo "Usage: $0 <artifact> <signature> [plugin.yaml path]"
  exit 1
fi

artifact="$1"
signature="$2"
plugin_yaml="${3:-plugin.yaml}"

if [ -z "${GPG_FINGERPRINT:-}" ]; then
  echo "ERROR: GPG_FINGERPRINT is not set. Cannot sign provenance artifact."
  exit 1
fi

filename="$(basename "$artifact")"
digest="$(sha256sum "$artifact" 2>/dev/null | cut -d' ' -f1 || shasum -a 256 "$artifact" | cut -d' ' -f1)"

passphrase_file="$(mktemp)"
trap 'rm -f "$passphrase_file"' EXIT
printf '%s' "${GPG_PASSPHRASE:-}" > "$passphrase_file"
chmod 600 "$passphrase_file"

{
  cat "$plugin_yaml"
  printf '...\n'
  printf 'files:\n  %s: "sha256:%s"\n' "$filename" "$digest"
} | gpg --batch --yes --armor --pinentry-mode loopback \
    --passphrase-file "$passphrase_file" \
    --local-user "$GPG_FINGERPRINT" \
    --clearsign --output "$signature"
