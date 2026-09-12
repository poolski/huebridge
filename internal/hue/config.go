package hue

import (
	"encoding/json"
	"net"
	"net/http"
)

func handleGetConfig(bridgeID string, mac net.HardwareAddr, win *PairingWindow) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := BridgeConfig{
			Name:             "huebridge",
			DatastoreVersion: "126",
			SwVersion:        "1000000000",
			APIVersion:       "1.61.0",
			Mac:              mac.String(),
			BridgeID:         bridgeID,
			FactoryNew:       false,
			ModelID:          "BSB002",
			ZigbeeChannel:    25,
			LinkButton:       win.IsOpen(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	}
}
