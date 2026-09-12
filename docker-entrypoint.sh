#!/bin/sh
# Home Assistant Supervisor add-on base images provide with-contenv (part of
# s6-overlay), which run.sh's "#!/usr/bin/with-contenv bashio" shebang needs
# to translate config.yaml options into env vars. A plain standalone image
# (built without --build-arg BUILD_FROM=<ha-base-image>) has neither, so
# fall straight through to the binary, which reads its configuration from
# env vars directly.
set -e

if command -v with-contenv >/dev/null 2>&1; then
	exec /run.sh
fi

exec /usr/bin/huebridge
