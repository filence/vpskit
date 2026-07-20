#!/usr/bin/env bash
set -Eeuo pipefail

cd /etc/vpskit/exports
sha256sum mihomo.yaml share-links.txt sing-box-hysteria2.json sing-box-reality.json
