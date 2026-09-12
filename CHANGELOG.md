# Changelog

## 0.1.2

- Fix ingress being unreachable: with `ingress_port: 0`, Supervisor never
  told the container which port it assigned (there's no environment
  variable for it), so the ingress server was listening on the wrong port.
  It's now fetched from Supervisor's own `/addons/self/info` API at
  startup, as Supervisor's docs describe.

## 0.1.1

- Add an `api_port` option to change the port the Hue Bridge API listens on
  (default `8299`), for hosts where 443 is already taken by another
  `host_network` add-on.
- Ingress now picks a free port automatically instead of a fixed `8099`,
  avoiding a conflict with Zigbee2MQTT's default port.

## 0.1.0

- Initial release: Hue Bridge emulation (CLIP v1 API, SSDP/mDNS discovery),
  ingress entity picker and pairing control.
