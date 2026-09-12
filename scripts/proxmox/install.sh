#!/bin/bash
# Installs huebridge (standalone mode) inside a Debian LXC as a systemd
# service. Meant to be run INSIDE the container — create-lxc.sh handles
# that. Safe to run standalone too, e.g. against an existing LXC/VM.
#
#   HUEBRIDGE_REPO       Git repo to build from (default: https://github.com/poolski/huebridge.git)
#   HUEBRIDGE_REF        Git ref to build (default: main)
#   HUEBRIDGE_LOG_LEVEL  Log level the installed service runs with (default: info; set to
#                        "debug" to log request/response bodies and headers while troubleshooting)
set -euo pipefail

HUEBRIDGE_REPO="${HUEBRIDGE_REPO:-https://github.com/poolski/huebridge.git}"
HUEBRIDGE_REF="${HUEBRIDGE_REF:-main}"
HUEBRIDGE_LOG_LEVEL="${HUEBRIDGE_LOG_LEVEL:-info}"

YW="\033[33m"
GN="\033[1;92m"
RD="\033[01;31m"
CL="\033[m"

msg_info() { echo -e " ${YW}○${CL} $1"; }
msg_ok() { echo -e " ${GN}✓${CL} $1"; }
msg_error() { echo -e " ${RD}✗${CL} $1" >&2; }

msg_info "Installing dependencies"
apt-get update -qq
apt-get install -y --no-install-recommends git ca-certificates curl >/dev/null
msg_ok "Installed dependencies"

ARCH="$(dpkg --print-architecture)"
case "$ARCH" in
	amd64) GOARCH=amd64 ;;
	arm64) GOARCH=arm64 ;;
	*)
		msg_error "unsupported architecture $ARCH"
		exit 1
		;;
esac

msg_info "Fetching huebridge source ($HUEBRIDGE_REF)"
rm -rf /opt/huebridge-src
git clone --quiet --branch "$HUEBRIDGE_REF" --depth 1 "$HUEBRIDGE_REPO" /opt/huebridge-src
cd /opt/huebridge-src
msg_ok "Fetched huebridge source"

GOVERSION="$(grep -oP '^go \K[0-9.]+' go.mod)"
if [ -z "$GOVERSION" ]; then
	msg_error "could not read go version from go.mod"
	exit 1
fi

if ! /usr/local/go/bin/go version 2>/dev/null | grep -q "go$GOVERSION"; then
	msg_info "Installing Go $GOVERSION"
	curl -fsSL "https://go.dev/dl/go${GOVERSION}.linux-${GOARCH}.tar.gz" -o /tmp/go.tar.gz
	rm -rf /usr/local/go
	tar -C /usr/local -xzf /tmp/go.tar.gz
	rm /tmp/go.tar.gz
	msg_ok "Installed Go $GOVERSION"
fi

msg_info "Building huebridge (this takes a few minutes)"
CGO_ENABLED=0 /usr/local/go/bin/go build -o /usr/local/bin/huebridge ./cmd/huebridge
msg_ok "Built huebridge"

mkdir -p /var/lib/huebridge

cat >/etc/systemd/system/huebridge.service <<UNIT
[Unit]
Description=huebridge (standalone)
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/huebridge
Environment=HUEBRIDGE_DATA_DIR=/var/lib/huebridge
Environment=HUEBRIDGE_API_PORT=443
Environment=HUEBRIDGE_LOG_LEVEL=$HUEBRIDGE_LOG_LEVEL
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT

msg_info "Starting huebridge"
systemctl daemon-reload
systemctl enable huebridge >/dev/null
# `enable --now` only starts the unit if it isn't already active — on a
# re-run against an already-running install (picking up a fix, a new
# HUEBRIDGE_REF), that's a no-op and the old binary keeps running despite
# just being overwritten. restart always replaces the running process.
systemctl restart huebridge
msg_ok "Started huebridge"
