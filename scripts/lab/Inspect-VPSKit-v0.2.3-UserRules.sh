#!/usr/bin/env bash
set -euo pipefail

/usr/local/bin/vpskit version
/usr/local/bin/vpskit rules show
/usr/local/bin/vpskit rules whitelist list
/usr/local/bin/vpskit rules custom list
/usr/local/bin/vpskit subscription status
systemctl is-active vpskit-xray.service
systemctl is-active vpskit-sing-box.service
