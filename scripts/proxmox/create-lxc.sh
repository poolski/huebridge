#!/bin/bash
# Creates a Debian LXC on Proxmox VE and installs huebridge (standalone
# mode) into it. Run this ON THE PROXMOX HOST, as root:
#
#   bash -c "$(curl -fsSL https://raw.githubusercontent.com/poolski/huebridge/main/scripts/proxmox/create-lxc.sh)"
#
# or from a local checkout:
#
#   ./scripts/proxmox/create-lxc.sh
#
# An LXC gets its own address directly on the LAN bridge, unlike a Docker
# container on the default bridge network — so SSDP/mDNS discovery and a
# direct bind on :443 both work with nothing else to fight for either.
#
# Everything below can also be set non-interactively via environment
# variables (VMID, LXC_HOSTNAME, BRIDGE, IP, GATEWAY, STORAGE,
# TEMPLATE_STORAGE, DISK_SIZE, MEMORY, CORES, HUEBRIDGE_REPO,
# HUEBRIDGE_REF) — Advanced Settings just prompts for each with the
# current value (env var or built-in default) preselected.
set -euo pipefail

YW="\033[33m"
GN="\033[1;92m"
RD="\033[01;31m"
CL="\033[m"

msg_info() { echo -e " ${YW}○${CL} $1"; }
msg_ok() { echo -e " ${GN}✓${CL} $1"; }
msg_error() { echo -e " ${RD}✗${CL} $1" >&2; }

header_info() {
	clear
	cat <<"EOF"
  _               _          _     _
 | |__  _   _  ___| |__  _ __(_) __| | __ _  ___
 | '_ \| | | |/ _ \ '_ \| '__| |/ _` |/ _` |/ _ \
 | | | | |_| |  __/ |_) | |  | | (_| | (_| |  __/
 |_| |_|\__,_|\___|_.__/|_|  |_|\__,_|\__, |\___|
                                      |___/
             standalone LXC installer
EOF
}

command -v pct >/dev/null 2>&1 || { msg_error "pct not found — this must run on a Proxmox VE host"; exit 1; }
[ "$(id -u)" -eq 0 ] || { msg_error "must run as root"; exit 1; }

if ! command -v whiptail >/dev/null 2>&1; then
	msg_info "Installing whiptail"
	apt-get update -qq && apt-get install -y --no-install-recommends whiptail >/dev/null
	msg_ok "Installed whiptail"
fi

VMID_DEFAULT="${VMID:-$(pvesh get /cluster/nextid)}"
LXC_HOSTNAME="${LXC_HOSTNAME:-huebridge}"
BRIDGE="${BRIDGE:-vmbr0}"
IP="${IP:-dhcp}"
GATEWAY="${GATEWAY:-}"
STORAGE="${STORAGE:-local-lvm}"
TEMPLATE_STORAGE="${TEMPLATE_STORAGE:-local}"
DISK_SIZE="${DISK_SIZE:-4}"
MEMORY="${MEMORY:-512}"
CORES="${CORES:-1}"
HUEBRIDGE_REPO="${HUEBRIDGE_REPO:-https://github.com/poolski/huebridge.git}"
HUEBRIDGE_REF="${HUEBRIDGE_REF:-main}"
VMID="$VMID_DEFAULT"

header_info

# whiptail needs a real terminal; fall back to defaults under curl | bash
# with stdin already consumed, or any other non-interactive invocation.
if [ -t 0 ] && [ -t 1 ]; then
	CHOICE=$(whiptail --backtitle "huebridge" --title "huebridge LXC" \
		--menu "\nRun with default settings, or customise resources/network?" 12 70 2 \
		"1" "Default Settings ($LXC_HOSTNAME, ${CORES}vCPU/${MEMORY}MB/${DISK_SIZE}GB, DHCP on $BRIDGE)" \
		"2" "Advanced Settings" \
		3>&1 1>&2 2>&3) || { msg_error "cancelled"; exit 1; }

	if [ "$CHOICE" = "2" ]; then
		VMID=$(whiptail --backtitle "huebridge" --title "Container ID" --inputbox "" 8 60 "$VMID" 3>&1 1>&2 2>&3) || exit 1
		LXC_HOSTNAME=$(whiptail --backtitle "huebridge" --title "Hostname" --inputbox "" 8 60 "$LXC_HOSTNAME" 3>&1 1>&2 2>&3) || exit 1
		CORES=$(whiptail --backtitle "huebridge" --title "CPU cores" --inputbox "" 8 60 "$CORES" 3>&1 1>&2 2>&3) || exit 1
		MEMORY=$(whiptail --backtitle "huebridge" --title "RAM (MB)" --inputbox "" 8 60 "$MEMORY" 3>&1 1>&2 2>&3) || exit 1
		DISK_SIZE=$(whiptail --backtitle "huebridge" --title "Disk size (GB)" --inputbox "" 8 60 "$DISK_SIZE" 3>&1 1>&2 2>&3) || exit 1
		STORAGE=$(whiptail --backtitle "huebridge" --title "Storage pool (rootfs)" --inputbox "" 8 60 "$STORAGE" 3>&1 1>&2 2>&3) || exit 1
		BRIDGE=$(whiptail --backtitle "huebridge" --title "Network bridge" --inputbox "" 8 60 "$BRIDGE" 3>&1 1>&2 2>&3) || exit 1
		if whiptail --backtitle "huebridge" --title "Networking" --yesno "Use DHCP?" 8 60; then
			IP="dhcp"
		else
			IP=$(whiptail --backtitle "huebridge" --title "Static IP" --inputbox "CIDR, e.g. 192.168.1.50/24" 8 60 "" 3>&1 1>&2 2>&3) || exit 1
			GATEWAY=$(whiptail --backtitle "huebridge" --title "Gateway" --inputbox "" 8 60 "" 3>&1 1>&2 2>&3) || exit 1
		fi
	fi
else
	msg_info "No TTY detected, using default/env-var settings without prompting"
fi

if [ "$IP" != "dhcp" ] && [ -z "$GATEWAY" ]; then
	msg_error "GATEWAY is required when IP is static (got IP=$IP)"
	exit 1
fi

if pct status "$VMID" >/dev/null 2>&1; then
	msg_error "container $VMID already exists — set VMID to an unused id"
	exit 1
fi

msg_info "Finding latest Debian 12 LXC template"
TEMPLATE=$(pveam available --section system | awk '{print $2}' | grep '^debian-12-standard' | sort -V | tail -1)
if [ -z "$TEMPLATE" ]; then
	msg_error "no debian-12-standard template found in 'pveam available' — run 'pveam update' first"
	exit 1
fi
msg_ok "Using $TEMPLATE"

if ! pveam list "$TEMPLATE_STORAGE" | grep -q "$TEMPLATE"; then
	msg_info "Downloading $TEMPLATE to $TEMPLATE_STORAGE"
	pveam download "$TEMPLATE_STORAGE" "$TEMPLATE" >/dev/null
	msg_ok "Downloaded $TEMPLATE"
fi

NET_CONF="name=eth0,bridge=$BRIDGE,firewall=1"
if [ "$IP" = "dhcp" ]; then
	NET_CONF="$NET_CONF,ip=dhcp"
else
	NET_CONF="$NET_CONF,ip=$IP,gw=$GATEWAY"
fi

msg_info "Creating LXC $VMID ($LXC_HOSTNAME)"
pct create "$VMID" "$TEMPLATE_STORAGE:vztmpl/$TEMPLATE" \
	--hostname "$LXC_HOSTNAME" \
	--net0 "$NET_CONF" \
	--rootfs "$STORAGE:$DISK_SIZE" \
	--memory "$MEMORY" \
	--cores "$CORES" \
	--unprivileged 1 \
	--features nesting=0 \
	--onboot 1 >/dev/null
msg_ok "Created LXC $VMID"

msg_info "Starting LXC $VMID"
pct start "$VMID" >/dev/null
msg_ok "Started LXC $VMID"

msg_info "Waiting for network"
for _ in $(seq 1 30); do
	if pct exec "$VMID" -- sh -c 'command -v ip >/dev/null && ip -4 -o addr show scope global' 2>/dev/null | grep -q .; then
		break
	fi
	sleep 2
done
msg_ok "Network is up"

msg_info "Installing huebridge inside the container (this builds from source, so it takes a few minutes)"
# BASH_SOURCE is unset (not just empty) when this script runs via
# `bash -c "$(curl ...)"`, as the README's one-liner does — there's no
# script file to speak of, so guard the array access under set -u.
INSTALL_SCRIPT_LOCAL=""
if [ -n "${BASH_SOURCE[0]:-}" ]; then
	INSTALL_SCRIPT_LOCAL="$(dirname "${BASH_SOURCE[0]}")/install.sh"
fi
# Pipe install.sh's content into the container over stdin rather than
# having the guest fetch it itself — a fresh Debian LXC template doesn't
# ship curl/wget, so a guest-side "curl | bash" can't even fetch the
# script that would go on to install curl. The host fetches it instead.
if [ -n "$INSTALL_SCRIPT_LOCAL" ] && [ -f "$INSTALL_SCRIPT_LOCAL" ]; then
	pct exec "$VMID" -- env HUEBRIDGE_REPO="$HUEBRIDGE_REPO" HUEBRIDGE_REF="$HUEBRIDGE_REF" bash -s <"$INSTALL_SCRIPT_LOCAL"
else
	RAW_BASE="$(echo "$HUEBRIDGE_REPO" | sed -E 's#github\.com#raw.githubusercontent.com#; s#\.git$##')"
	INSTALL_URL="${RAW_BASE}/${HUEBRIDGE_REF}/scripts/proxmox/install.sh"
	curl -fsSL "$INSTALL_URL" | pct exec "$VMID" -- env HUEBRIDGE_REPO="$HUEBRIDGE_REPO" HUEBRIDGE_REF="$HUEBRIDGE_REF" bash -s
fi
msg_ok "Installed huebridge"

CONTAINER_IP=$(pct exec "$VMID" -- sh -c "ip -4 -o addr show scope global | awk '{print \$4}' | cut -d/ -f1 | head -1")

echo
msg_ok "Done. huebridge is running in LXC $VMID at $CONTAINER_IP."
msg_ok "Open https://$CONTAINER_IP:8300/ to complete first-run setup."
