# huebridge

A Home Assistant add-on that emulates a Philips Hue Bridge over a curated set
of your HA entities. Point the real Hue app (or an Alexa/Google Home
integration that speaks Hue) at your Home Assistant box, and it discovers
your chosen lights, groups, scenes, and schedules as if they were a genuine
Hue Bridge — no physical bridge required.

It speaks the Hue CLIP v1 API, advertises itself over SSDP and mDNS so the
Hue app finds it automatically, and gives you an ingress web UI inside Home
Assistant to pick which entities are exposed and to pair new clients.

## Requirements

- A running Home Assistant instance with the **Supervisor** (Home Assistant
  OS or Supervised install — this is an add-on, so a Core-only or Container
  install won't work).
- Network access from your Hue app/Alexa/Google Home to the Home Assistant
  host, since discovery relies on SSDP/mDNS broadcasts reaching the same
  network segment.

## Installation

1. In Home Assistant, go to **Settings → Add-ons → Add-on Store**.
2. Open the **⋮** menu (top right) → **Repositories**.
3. Add `https://github.com/poolski/huebridge` and click **Add**.
4. Find **huebridge** in the store list (you may need to refresh) and click
   **Install**.
5. Once installed, go to the add-on's **Info** tab and turn on **Start on
   boot** if you want it to come up automatically.
6. Click **Start**.

The add-on needs `host_network: true` (for SSDP/mDNS) and access to the
Supervisor API — both are requested automatically; there's nothing to
configure by hand.

## Setup

huebridge has two options, set on the add-on's **Configuration** tab:

- **`api_port`** (default `8299`) — the port the emulated Hue Bridge API,
  and its SSDP/mDNS discovery, listens on. Real Hue bridges use 443, but on
  a Home Assistant host that port is normally already owned by something
  else (Home Assistant's own web server, a reverse proxy, an SSL
  terminator, ...), so huebridge defaults to a free port instead of
  competing for it. The Hue app discovers whatever port is advertised, so
  there's no need to change this unless `8299` itself collides with
  something on your network.
- **`log_level`** (default `info`) — `info` logs each request's method,
  path, status, and duration. `debug` additionally logs request headers
  and request/response bodies, for diagnosing what the Hue app actually
  sent and got back.

Everything else is done through its ingress panel.

1. With the add-on running, click **Open Web UI** (or the ingress icon) on
   the add-on's page.
2. Use the **entity picker** to choose which Home Assistant entities to
   expose as Hue lights, and optionally group them.
3. In the Hue app (or your Alexa/Google Home skill setup), start bridge
   discovery — it should find huebridge automatically via SSDP/mDNS.
4. When the app asks you to press the physical link button, go back to the
   ingress UI and click **Allow pairing** instead — this opens a short
   window during which the app's pairing request is accepted.
5. Once paired, your exposed entities show up in the Hue app as lights,
   ready to control or add to Alexa/Google Home.

Entities can be added, removed, or regrouped at any time from the same UI —
changes take effect immediately, no restart needed.

## Running standalone

huebridge can also run without Home Assistant's Supervisor — e.g. on
separate hardware from HA, or against HA Core without the Supervisor. It's
the same binary; it switches into standalone mode automatically whenever
`SUPERVISOR_TOKEN` isn't set (i.e. whenever it's not started as a
Supervisor add-on).

SSDP/mDNS discovery (how the Hue app finds a bridge) is LAN multicast — it
doesn't cross Docker's default bridge network, and a container on it would
advertise its own internal IP anyway, which the Hue app could never reach.
**Standalone containers need `--network host`**, the same reason the Home
Assistant add-on requires `host_network: true`. Bind the bridge to `443`
while you're at it: the official Hue app's local search never follows the
SSDP-advertised port, only probing 80/443 directly, and a standalone host
(unlike the add-on's) usually doesn't already have HA's own web server or
a reverse proxy sitting on it.

There's no published image for this yet — build one from the `Dockerfile`
in this repo, then run it:

```bash
docker build -t huebridge .
docker run -d \
  --network host \
  -e HUEBRIDGE_API_PORT=443 \
  -v huebridge-data:/data \
  huebridge
```

Or, with the `docker-compose.yml` in this repo (also importable as a stack
in Portainer, and works with Podman's `podman-compose`/`podman compose`):

```bash
docker compose up -d --build
```

### Running on Proxmox (LXC)

An LXC is arguably the better fit for standalone huebridge than a Docker
container: it gets its own address directly on the LAN bridge, so there's
nothing to configure for SSDP/mDNS discovery or binding `443` — no
`--network host` equivalent needed, it's just how LXC networking works.

Run this on the Proxmox host itself, as root:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/poolski/huebridge/main/scripts/proxmox/create-lxc.sh)"
```

It walks through the same default-vs-advanced-settings prompts as the
[Proxmox VE Community Scripts](https://github.com/community-scripts/ProxmoxVE)
you may already be used to (this doesn't depend on that project — it's a
self-contained script in this repo, `scripts/proxmox/create-lxc.sh`, at the
default settings: Debian 12, 1 vCPU, 512MB RAM, 4GB disk, DHCP), creates an
unprivileged LXC, and installs huebridge into it as a systemd service
listening on `443`, built from source since there's no published binary
release yet. Every setting is also overridable non-interactively via
environment variables — see the script's header comment.

- **`HUEBRIDGE_API_PORT`** (default `8299`) — same as the add-on's
  `api_port` option: the emulated Hue Bridge API and its SSDP/mDNS
  discovery.
- **`HUEBRIDGE_ADMIN_PORT`** (default `8300`) — the setup wizard and,
  afterwards, the entity-picker UI. Both are served over HTTPS with a
  self-signed cert (browsers will warn once) and, after setup, gated by
  HTTP Basic Auth (`admin` / the password you set during setup).
- **`HUEBRIDGE_DATA_DIR`** (default `/data`) — where `standalone.json`
  (your HA URL, token, and admin password hash), the entity registry, and
  scenes/schedules are stored.

The first-run setup wizard itself is unauthenticated — it has to be, since
there's no password yet for it to check — and listens on all interfaces.
Anyone who can reach `HUEBRIDGE_ADMIN_PORT` before you complete setup can
claim the bridge and its Home Assistant connection. Complete the wizard
promptly on a trusted network (right after `docker run`, before exposing
the port beyond your LAN).

On first run, open `https://<host>:8300/` and follow the two-step wizard:
set an admin password, then enter your Home Assistant URL (huebridge tries
to find it via mDNS and prefills the field if it does) and a [long-lived
access token](https://www.home-assistant.io/docs/authentication/#your-account-profile)
from your HA profile. huebridge verifies the token against HA before
accepting it. After that, the same entity picker the add-on's ingress
panel offers is available at the admin URL, behind that password.

## Troubleshooting

- **App doesn't find the bridge**: confirm the add-on is running and that
  your phone/speaker is on the same network as Home Assistant (SSDP/mDNS
  don't cross VLANs or most guest networks).
- **Third-party apps (e.g. Hue Essentials) don't find the bridge, or fail
  to pair**: these generally respect the port SSDP/mDNS advertise, so
  `api_port` alone should work. Set `log_level: debug` and check the
  add-on's logs for the incoming request — it shows exactly what the app
  sent and what huebridge replied.
- **The official Hue app never finds the bridge, and no request for it
  ever shows up in the logs at all**: the official app's local search
  doesn't follow the SSDP-advertised port — it only probes the
  conventional bridge ports, 80 and 443. If those are already in use on
  your Home Assistant host (a reverse proxy, HA's own web server, ...),
  huebridge can't bind them via `api_port`, and the official app will
  never even attempt a connection to whatever port it's actually on.

  There's no good fix for this if that reverse proxy is also fronting
  Home Assistant itself on the same hostname/IP: Home Assistant's own
  REST API already lives at `/api/*` on that host, so a path-based rule
  forwarding `/api` to huebridge would shadow HA's own API rather than
  add a side channel for huebridge — the two can't share a path on the
  same (host, port) pair. Path-based routing only works when the paths
  are actually distinct between the two backends, which isn't the case
  here.

  In practice, the two workable options are:
  - Free up 80/443 for huebridge specifically (move whatever else is
    using them, or give huebridge's host a second IP that nothing else
    listens on — provided your reverse proxy isn't itself bound to
    "all interfaces" on that port, which would still collide).
  - Use a third-party app that respects the SSDP-advertised port
    instead. Hue Essentials and similar apps do, and work with
    huebridge on whatever `api_port` you've set.
- **"Press the link button" never succeeds**: you need to click **Allow
  pairing** in the ingress UI *before* (or while) the app is trying — the
  window is time-limited.
- **A light doesn't respond or is missing**: check it's selected in the
  entity picker; huebridge only exposes entities you've explicitly added.

## Building from source

Requires Go 1.27+.

```bash
go build -o huebridge ./cmd/huebridge
```

Run the test suite with:

```bash
go test ./...
```
