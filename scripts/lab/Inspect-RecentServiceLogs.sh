#!/usr/bin/env bash
set -eu

journalctl -u vpskit-sing-box.service --since '-10 minutes' --no-pager -o short-iso | tail -n 120
