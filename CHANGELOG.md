# Changelog

## 0.1.1

- Add an `api_port` option to change the port the Hue Bridge API listens on
  (default `8299`), for hosts where 443 is already taken by another
  `host_network` add-on.
- Ingress now picks a free port automatically instead of a fixed `8099`,
  avoiding a conflict with Zigbee2MQTT's default port.

## 0.1.0

- Initial release: Hue Bridge emulation (CLIP v1 API, SSDP/mDNS discovery),
  ingress entity picker and pairing control.
