#!/usr/bin/env python3
"""Run audited VPS lab operations without exposing credentials in argv."""

from __future__ import annotations

import argparse
import base64
import hashlib
import pathlib
import re
import sys

import paramiko


PROJECT_ROOT = pathlib.Path(__file__).resolve().parents[2]
ENVIRONMENT_DOCUMENT = PROJECT_ROOT / "前期环境须知.md"
KNOWN_HOSTS = PROJECT_ROOT / ".build" / "paramiko_known_hosts"


def read_field(document: str, label: str) -> str:
    match = re.search(rf"^{re.escape(label)}[：:]\s*(.+?)\s*$", document, re.MULTILINE)
    if not match:
        raise RuntimeError(f"Required environment field is missing: {label}")
    return match.group(1).strip()


def load_connection_fields() -> tuple[str, int, str, str]:
    document = ENVIRONMENT_DOCUMENT.read_text(encoding="utf-8")
    host = read_field(document, "IPV4地址")
    port = int(read_field(document, "端口"))
    username = read_field(document, "用户名")
    password = read_field(document, "Root密码")
    if not re.fullmatch(r"\d{1,3}(?:\.\d{1,3}){3}", host):
        raise RuntimeError("The SSH host is not an IPv4 address")
    if not 1 <= port <= 65535:
        raise RuntimeError("The SSH port is invalid")
    return host, port, username, password


def connect() -> paramiko.SSHClient:
    host, port, username, password = load_connection_fields()
    client = paramiko.SSHClient()
    first_connection = not KNOWN_HOSTS.exists()
    if not first_connection:
        client.load_host_keys(str(KNOWN_HOSTS))
        client.set_missing_host_key_policy(paramiko.RejectPolicy())
    else:
        KNOWN_HOSTS.parent.mkdir(parents=True, exist_ok=True)
        client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    client.connect(
        hostname=host,
        port=port,
        username=username,
        password=password,
        timeout=15,
        banner_timeout=15,
        auth_timeout=15,
        allow_agent=False,
        look_for_keys=False,
    )
    if first_connection:
        client.save_host_keys(str(KNOWN_HOSTS))
    return client


def print_host_fingerprint(client: paramiko.SSHClient) -> None:
    transport = client.get_transport()
    if transport is None:
        raise RuntimeError("SSH transport is unavailable")
    key = transport.get_remote_server_key()
    digest = hashlib.sha256(key.asbytes()).digest()
    fingerprint = base64.b64encode(digest).decode("ascii").rstrip("=")
    print(f"ssh_host_key={key.get_name()} SHA256:{fingerprint}")


def run_script(client: paramiko.SSHClient, script_path: pathlib.Path) -> int:
    script = script_path.read_bytes()
    stdin, stdout, stderr = client.exec_command("bash -s", timeout=900)
    stdin.write(script)
    stdin.channel.shutdown_write()
    stdout_data = stdout.read()
    stderr_data = stderr.read()
    exit_code = stdout.channel.recv_exit_status()
    if stdout_data:
        sys.stdout.buffer.write(stdout_data)
    if stderr_data:
        sys.stderr.buffer.write(stderr_data)
    return exit_code


def upload(client: paramiko.SSHClient, local_path: pathlib.Path, remote_path: str, mode: int) -> int:
    with client.open_sftp() as sftp:
        sftp.put(str(local_path), remote_path, confirm=True)
        sftp.chmod(remote_path, mode)
        attributes = sftp.stat(remote_path)
    print(f"uploaded={remote_path} size={attributes.st_size} mode={mode:o}")
    return 0


def download(client: paramiko.SSHClient, remote_path: str, local_path: pathlib.Path) -> int:
    local_path.parent.mkdir(parents=True, exist_ok=True)
    with client.open_sftp() as sftp:
        sftp.get(remote_path, str(local_path))
        attributes = sftp.stat(remote_path)
    print(f"downloaded={remote_path} size={attributes.st_size}")
    return 0


def upload_field(client: paramiko.SSHClient, label: str, remote_path: str, mode: int) -> int:
    document = ENVIRONMENT_DOCUMENT.read_text(encoding="utf-8")
    value = read_field(document, label)
    encoded = (value + "\n").encode("utf-8")
    with client.open_sftp() as sftp:
        with sftp.file(remote_path, "wb") as remote_file:
            remote_file.write(encoded)
            remote_file.flush()
        sftp.chmod(remote_path, mode)
        attributes = sftp.stat(remote_path)
    print(f"uploaded_field={label} remote={remote_path} size={attributes.st_size} mode={mode:o}")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)

    exec_parser = subparsers.add_parser("exec-script")
    exec_parser.add_argument("script", type=pathlib.Path)

    upload_parser = subparsers.add_parser("upload")
    upload_parser.add_argument("local", type=pathlib.Path)
    upload_parser.add_argument("remote")
    upload_parser.add_argument("--mode", type=lambda value: int(value, 8), default=0o600)

    download_parser = subparsers.add_parser("download")
    download_parser.add_argument("remote")
    download_parser.add_argument("local", type=pathlib.Path)

    field_parser = subparsers.add_parser("upload-field")
    field_parser.add_argument("label")
    field_parser.add_argument("remote")
    field_parser.add_argument("--mode", type=lambda value: int(value, 8), default=0o600)
    return parser


def main() -> int:
    arguments = build_parser().parse_args()
    client = connect()
    try:
        print_host_fingerprint(client)
        if arguments.command == "exec-script":
            return run_script(client, arguments.script)
        if arguments.command == "upload":
            return upload(client, arguments.local, arguments.remote, arguments.mode)
        if arguments.command == "download":
            return download(client, arguments.remote, arguments.local)
        if arguments.command == "upload-field":
            return upload_field(client, arguments.label, arguments.remote, arguments.mode)
        raise RuntimeError(f"Unsupported command: {arguments.command}")
    finally:
        client.close()


if __name__ == "__main__":
    raise SystemExit(main())
