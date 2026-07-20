#!/usr/bin/env bash
set -eu

binary='/root/vpskit-v0.1.0-lab.5-doctor'
expected_sha256='8f0a73c024e8055c16720248386e2d7afc19527beb49b28a5ceaf2402965f662'
printf '%s  %s\n' "$expected_sha256" "$binary" | sha256sum -c -
"$binary" doctor
