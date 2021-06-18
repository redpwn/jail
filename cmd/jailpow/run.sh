#!/bin/sh
# jailpow runner
# https://github.com/redpwn/jail/blob/master/cmd/jailpow/run.sh

set -e
version=v0.0.2
challenge=$1
run() {
  cache_root=$HOME/.cache/jailpow
  mkdir -p "$cache_root"
  cache_path="$cache_root/jailpow-$version"
  case $(uname | tr '[:upper:]' '[:lower:]') in
    linux*) release=linux-amd64;;
    darwin*) release=darwin-amd64;;
    msys*) release=windows-amd64.exe;;
    cygwin*) release=windows-amd64.exe;;
    *) echo unknown OS; exit 1
  esac
  [ -e "$cache_path" ] || curl -sSfLo "$cache_path" "https://github.com/redpwn/jail/releases/download/$version/jailpow-$release" && chmod u+x "$cache_path"
  "$cache_path" "$challenge"
}
run
