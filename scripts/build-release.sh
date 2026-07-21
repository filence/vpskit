#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

fail() {
    printf 'release build failed: %s\n' "$1" >&2
    exit 1
}

usage() {
    printf 'usage: scripts/build-release.sh <version> <owner/repository> <new-output-directory>\n'
}

[ "$#" -eq 3 ] || { usage >&2; exit 2; }
readonly version="$1"
readonly repository="$2"
readonly requested_output="$3"
project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
readonly project_root

for command_name in go curl sha256sum tar unzip jq stat install mktemp realpath grep awk; do
    command -v "$command_name" >/dev/null 2>&1 || fail "missing command: $command_name"
done
printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$' || fail 'invalid release version'
printf '%s\n' "$repository" | grep -Eq '^[0-9A-Za-z_.-]+/[0-9A-Za-z_.-]+$' || fail 'invalid GitHub repository'
[ -n "${VPSKIT_RELEASE_PRIVATE_KEY:-}" ] || fail 'VPSKIT_RELEASE_PRIVATE_KEY is required'
[ -n "${VPSKIT_RELEASE_PUBLIC_KEY:-}" ] || fail 'VPSKIT_RELEASE_PUBLIC_KEY is required'
[ -n "${VPSKIT_RELEASE_NEXT_PUBLIC_KEY:-}" ] || fail 'VPSKIT_RELEASE_NEXT_PUBLIC_KEY is required'

output_directory="$(realpath -m "$requested_output")"
[ ! -e "$output_directory" ] || fail 'output directory already exists'
output_parent="$(dirname "$output_directory")"
[ -d "$output_parent" ] || fail 'output parent directory does not exist'

temporary_root="$(mktemp -d /tmp/vpskit-release.XXXXXX)"
cleanup() {
    unset VPSKIT_RELEASE_PRIVATE_KEY VPSKIT_RELEASE_PUBLIC_KEY VPSKIT_RELEASE_NEXT_PUBLIC_KEY || true
    case "$temporary_root" in
        /tmp/vpskit-release.*) rm -rf -- "$temporary_root" ;;
    esac
}
trap cleanup EXIT HUP INT TERM

readonly tools="$temporary_root/tools"
readonly downloads="$temporary_root/downloads"
readonly bundle="$temporary_root/$version"
mkdir -m 0700 -- "$tools" "$downloads" "$bundle" "$output_directory"

download_and_verify() {
    local url="$1"
    local expected_sha256="$2"
    local destination="$3"
    curl --fail --location --proto '=https' --tlsv1.2 --retry 3 --retry-delay 2 --output "$destination" "$url"
    printf '%s  %s\n' "$expected_sha256" "$destination" | sha256sum --check --strict >/dev/null
}

readonly release_tool="$tools/vpskit-release"
readonly private_key="$tools/release-private.key"
readonly public_key="$tools/release-public.key"
readonly next_public_key="$tools/release-next-public.key"
printf '%s\n' "$VPSKIT_RELEASE_PRIVATE_KEY" >"$private_key"
printf '%s\n' "$VPSKIT_RELEASE_PUBLIC_KEY" >"$public_key"
printf '%s\n' "$VPSKIT_RELEASE_NEXT_PUBLIC_KEY" >"$next_public_key"
chmod 0600 "$private_key" "$public_key" "$next_public_key"

(
    cd "$project_root"
    go mod verify
    go test ./...
    go build -trimpath -o "$release_tool" ./cmd/vpskit-release
)

current_key_id="$($release_tool keyid --public "$public_key")"
next_key_id="$($release_tool keyid --public "$next_public_key")"
[ "$current_key_id" != "$next_key_id" ] || fail 'current and next release public keys must differ'
trust_policy="$($release_tool trust --public "$public_key" --public "$next_public_key")"

readonly sing_archive="$downloads/sing-box-1.13.14-linux-amd64.tar.gz"
readonly xray_archive="$downloads/Xray-linux-64.zip"
readonly lego_archive="$downloads/lego_v5.2.2_linux_amd64.tar.gz"
readonly syft_archive="$downloads/syft_1.44.0_linux_amd64.tar.gz"
download_and_verify 'https://github.com/SagerNet/sing-box/releases/download/v1.13.14/sing-box-1.13.14-linux-amd64.tar.gz' \
    'f48703461a15476951ac4967cdad339d986f4b8096b4eb3ff0829a500502d697' "$sing_archive"
download_and_verify 'https://github.com/XTLS/Xray-core/releases/download/v26.3.27/Xray-linux-64.zip' \
    '23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae' "$xray_archive"
download_and_verify 'https://github.com/go-acme/lego/releases/download/v5.2.2/lego_v5.2.2_linux_amd64.tar.gz' \
    '018de6d3f2da09630caa2fbbe8c6aa459323ad0ac0a053d0e808268914b38a8b' "$lego_archive"
download_and_verify 'https://github.com/anchore/syft/releases/download/v1.44.0/syft_1.44.0_linux_amd64.tar.gz' \
    '0e91737aee2b5baf1d255b959630194a302335d848ff97bb07921eb6205b5f5a' "$syft_archive"

mkdir -m 0700 -- "$temporary_root/sing" "$temporary_root/xray" "$temporary_root/lego" "$temporary_root/syft"
tar -xzf "$sing_archive" -C "$temporary_root/sing"
unzip -q "$xray_archive" -d "$temporary_root/xray"
tar -xzf "$lego_archive" -C "$temporary_root/lego"
tar -xzf "$syft_archive" -C "$temporary_root/syft" syft

readonly linker_flags="-s -w -X main.version=$version -X main.releasePublicKeyBase64=$trust_policy"
(
    cd "$project_root"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$linker_flags" -o "$bundle/vpskit" ./cmd/vpskit
)
install -m 0755 "$temporary_root/sing/sing-box-1.13.14-linux-amd64/sing-box" "$bundle/sing-box"
install -m 0755 "$temporary_root/xray/xray" "$bundle/xray"
install -m 0755 "$temporary_root/lego/lego" "$bundle/lego"

mkdir -m 0755 -- "$bundle/licenses"
install -m 0644 "$project_root/LICENSE" "$bundle/LICENSE"
install -m 0644 "$project_root/NOTICE.md" "$bundle/NOTICE.md"
install -m 0644 "$project_root/THIRD_PARTY_LICENSES.md" "$bundle/THIRD_PARTY_LICENSES.md"
install -m 0644 "$project_root/licenses/go-qrcode.LICENSE" "$bundle/licenses/go-qrcode.LICENSE"
install -m 0644 "$project_root/licenses/go-yaml.LICENSE" "$bundle/licenses/go-yaml.LICENSE"
install -m 0644 "$project_root/licenses/go-yaml.NOTICE" "$bundle/licenses/go-yaml.NOTICE"
install -m 0644 "$temporary_root/sing/sing-box-1.13.14-linux-amd64/LICENSE" "$bundle/licenses/sing-box.LICENSE"
install -m 0644 "$temporary_root/xray/LICENSE" "$bundle/licenses/xray.LICENSE"
install -m 0644 "$temporary_root/lego/LICENSE" "$bundle/licenses/lego.LICENSE"

"$temporary_root/syft/syft" "file:$bundle/vpskit" -o "spdx-json=$bundle/vpskit.spdx.json"
sing_binary_sha256="$(sha256sum "$bundle/sing-box" | awk '{print $1}')"
xray_binary_sha256="$(sha256sum "$bundle/xray" | awk '{print $1}')"
lego_binary_sha256="$(sha256sum "$bundle/lego" | awk '{print $1}')"
generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
jq -n \
    --arg generated_at "$generated_at" \
    --arg sing_sha "$sing_binary_sha256" \
    --arg xray_sha "$xray_binary_sha256" \
    --arg lego_sha "$lego_binary_sha256" \
    '{
      schema_version: 1,
      generated_at: $generated_at,
      template_revision: 2,
      assets: [
        {
          id: "sing-box", version: "1.13.14", channel: "stable",
          source_repo: "SagerNet/sing-box", source_ref: "v1.13.14",
          source_commit: "25a600db24f7680ad9806ce5427bd0ab8afe1114",
          source_url: "https://github.com/SagerNet/sing-box/releases/tag/v1.13.14",
          source_archive: {name: "sing-box-1.13.14-linux-amd64.tar.gz", size: 23832905, sha256: "f48703461a15476951ac4967cdad339d986f4b8096b4eb3ff0829a500502d697"},
          installed_file: "sing-box", binary_sha256: $sing_sha, verified_at: "2026-07-18", state_schema_min: 1
        },
        {
          id: "xray", version: "26.3.27", channel: "stable",
          source_repo: "XTLS/Xray-core", source_ref: "v26.3.27",
          source_commit: "d2758a023cd7f4174a5a5fa4ff66e487d4342ba0",
          source_url: "https://github.com/XTLS/Xray-core/releases/tag/v26.3.27",
          source_archive: {name: "Xray-linux-64.zip", size: 21136402, sha256: "23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae"},
          installed_file: "xray", binary_sha256: $xray_sha, verified_at: "2026-07-19", state_schema_min: 4
        },
        {
          id: "lego", version: "5.2.2", channel: "stable",
          source_repo: "go-acme/lego", source_ref: "v5.2.2",
          source_commit: "3d5a6695e027d625bd34334d516d77f578d43f11",
          source_url: "https://github.com/go-acme/lego/releases/tag/v5.2.2",
          source_archive: {name: "lego_v5.2.2_linux_amd64.tar.gz", size: 21076129, sha256: "018de6d3f2da09630caa2fbbe8c6aa459323ad0ac0a053d0e808268914b38a8b"},
          installed_file: "lego", binary_sha256: $lego_sha, verified_at: "2026-07-18", state_schema_min: 1
        }
      ]
    }' >"$bundle/versions.lock"
chmod 0644 "$bundle/versions.lock" "$bundle/vpskit.spdx.json"

readonly manifest="$bundle/release-manifest.json"
readonly signature="$bundle/release-manifest.sig"
"$release_tool" manifest --dir "$bundle" --output "$manifest" --release-id "$version" --signing-key-id "$current_key_id"
"$release_tool" sign --manifest "$manifest" --private "$private_key" --output "$signature"
"$release_tool" verify --dir "$bundle" --public "$public_key"
"$bundle/vpskit" bundle verify --dir "$bundle" >/dev/null

readonly archive_name="$version-linux-amd64.tar.gz"
readonly archive_path="$output_directory/$archive_name"
"$release_tool" pack --dir "$bundle" --output "$archive_path"
"$release_tool" bootstrap \
    --template "$project_root/scripts/install.sh.in" \
    --archive "$archive_path" \
    --output "$output_directory/install.sh" \
    --version "$version" \
    --repository "$repository"

install -m 0644 "$manifest" "$output_directory/release-manifest.json"
install -m 0644 "$signature" "$output_directory/release-manifest.sig"
install -m 0644 "$bundle/versions.lock" "$output_directory/versions.lock"
install -m 0644 "$bundle/vpskit.spdx.json" "$output_directory/vpskit.spdx.json"
(
    cd "$output_directory"
    sha256sum "$archive_name" install.sh release-manifest.json release-manifest.sig versions.lock vpskit.spdx.json >checksums.txt
)
chmod 0644 "$output_directory"/*
printf 'RELEASE_BUILD=PASS version=%s output=%s\n' "$version" "$output_directory"
