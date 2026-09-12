#!/usr/bin/with-contenv bashio
set -e

export HUEBRIDGE_DATA_DIR="/data"
export HUEBRIDGE_HA_URL="http://supervisor/core"
export HUEBRIDGE_API_PORT="$(bashio::config 'api_port')"
export HUEBRIDGE_LOG_LEVEL="$(bashio::config 'log_level')"

exec /usr/bin/huebridge
