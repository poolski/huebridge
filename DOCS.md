# Home Assistant Add-on: huebridge

Emulates a Philips Hue Bridge over a curated set of Home Assistant entities,
so the real Hue app (or anything that speaks Hue, like Alexa or Google Home)
can discover and control them without an actual Hue Bridge.

## Options

| Option     | Default | Description                                                             |
| ---------- | ------- | ----------------------------------------------------------------------- |
| `api_port` | `8299`  | Port the emulated Hue Bridge API and its SSDP/mDNS discovery listen on. |

Real Hue bridges use 443, but on a Home Assistant host that's usually
already taken (by Home Assistant itself, a reverse proxy, ...), so huebridge
defaults elsewhere instead of competing for it. The Hue app discovers
whatever port is advertised, so there's no need to change this unless
`8299` itself collides with something on your network.

Everything else is done through the add-on's ingress panel — there are no
other YAML options.

## Setup

1. Start the add-on.
2. Click **Open Web UI** on the add-on's page.
3. Use the entity picker to choose which Home Assistant entities to expose
   as Hue lights, and optionally group them.
4. In the Hue app (or your Alexa/Google Home skill setup), start bridge
   discovery — it should find huebridge automatically via SSDP/mDNS.
5. When the app asks you to press the physical link button, go back to the
   ingress UI and click **Allow pairing** instead — this opens a short
   window during which the app's pairing request is accepted.
6. Once paired, your exposed entities show up in the Hue app as lights.

Entities can be added, removed, or regrouped at any time from the same UI —
changes take effect immediately, no restart needed.

## Notes

- Requires `host_network: true` for SSDP/mDNS discovery to work — this is
  configured automatically, nothing to change.
- Discovery relies on broadcast traffic, so the Hue app/speaker must be on
  the same network segment as Home Assistant (no VLANs or guest networks in
  between).

## Standalone mode

huebridge can run without Home Assistant's Supervisor — on separate hardware,
or against Home Assistant Core without the Supervisor. It automatically enters
standalone mode when `SUPERVISOR_TOKEN` is not set.

See the [Running standalone](README.md#running-standalone) section in the
README for installation and setup instructions.

| Environment Variable | Default | Description |
| -------------------- | ------- | ----------- |
| `HUEBRIDGE_API_PORT` | `8299` | Port for the emulated Hue Bridge API and SSDP/mDNS discovery. |
| `HUEBRIDGE_ADMIN_PORT` | `8300` | Port for the setup wizard and entity-picker UI (HTTPS with self-signed cert). |
| `HUEBRIDGE_DATA_DIR` | `/data` | Directory for configuration, entity registry, and scenes/schedules. |
| `HUEBRIDGE_LOG_LEVEL` | `info` | Log level: `info` for request summaries, `debug` for detailed request/response logging. |
