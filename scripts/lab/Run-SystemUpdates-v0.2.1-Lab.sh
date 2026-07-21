#!/usr/bin/env bash
set -euo pipefail

/usr/local/bin/vpskit version
/usr/local/bin/vpskit system updates
/usr/local/bin/vpskit status
