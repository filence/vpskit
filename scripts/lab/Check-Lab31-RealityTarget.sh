#!/usr/bin/env bash
set -euo pipefail

/usr/local/lib/vpskit/bin/xray tls ping www.amazon.com
/usr/local/bin/vpskit reality scan --targets www.amazon.com
