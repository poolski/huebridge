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
