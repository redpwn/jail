#!/bin/sh

set -e

version=v0.0.1

cache_root=~/.cache/jailpow
mkdir -p "$cache_root"
cache_path="$cache_root/$version"
[ -e "$cache_path" ] || curl -sSfLo "$cache_path" "https://github.com/redpwn/jail/releases/download/$version/jailpow-linux-amd64" && chmod u+x "$cache_path"
"$cache_path" "$1"
