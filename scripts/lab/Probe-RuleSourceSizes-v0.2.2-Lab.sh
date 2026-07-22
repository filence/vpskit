#!/usr/bin/env bash
set -euo pipefail

sources=(
  'ACL4SSR-LocalAreaNetwork|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/LocalAreaNetwork.list'
  'ACL4SSR-UnBan|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/UnBan.list'
  'ACL4SSR-Gemini|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/Gemini.list'
  'ACL4SSR-SteamCN|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/SteamCN.list'
  'ACL4SSR-Telegram|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Telegram.list'
  'ACL4SSR-AI|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/AI.list'
  'ACL4SSR-OpenAi|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/OpenAi.list'
  'ACL4SSR-Github|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/Github.list'
  'ACL4SSR-YouTube|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Ruleset/YouTube.list'
  'ACL4SSR-ProxyMedia|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/ProxyMedia.list'
  'ACL4SSR-Bing|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Bing.list'
  'ACL4SSR-OneDrive|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/OneDrive.list'
  'ACL4SSR-Microsoft|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Microsoft.list'
  'ACL4SSR-Apple|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/Apple.list'
  'ACL4SSR-ChinaDomain|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/ChinaDomain.list'
  'ACL4SSR-ChinaCompanyIp|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/ChinaCompanyIp.list'
  'ACL4SSR-ProxyGFWlist|https://raw.githubusercontent.com/ACL4SSR/ACL4SSR/master/Clash/ProxyGFWlist.list'
  'Google-All-Domain|https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/google.mrs'
  'Google-All-IP|https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/google.mrs'
  'anti-AD|https://anti-ad.net/mihomo.mrs'
)

printf 'RULE_SOURCE_SIZE_PROBE=BEGIN\n'
total=0
for source in "${sources[@]}"; do
  name=${source%%|*}
  url=${source#*|}
  size=$(curl --fail --location --silent --show-error --max-time 30 --output /dev/null --write-out '%{size_download}' "$url")
  type=$(curl --fail --location --silent --show-error --max-time 30 --output /dev/null --write-out '%{content_type}' "$url")
  total=$((total + size))
  printf 'source=%s bytes=%s content_type=%s\n' "$name" "$size" "${type:-unknown}"
done
printf 'total_bytes=%s estimated_base64_payload_bytes=%s\n' "$total" "$(( (total + 2) / 3 * 4 ))"
printf 'RULE_SOURCE_SIZE_PROBE=PASS\n'
