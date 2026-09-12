#!/usr/bin/with-contenv bashio
set -e

export HUEBRIDGE_DATA_DIR="/data"
export HUEBRIDGE_HA_URL="http://supervisor/core"

exec /usr/bin/huebridge
