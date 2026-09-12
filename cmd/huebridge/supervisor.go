package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// fetchIngressPort asks Supervisor which port it assigned this add-on's
// ingress server. config.yaml sets ingress_port: 0 so Supervisor picks a
// free port at install time, but it never hands that port to the container
// via an environment variable — per Supervisor's own docs, an add-on using
// dynamic ingress ports on the host network has to "read the port later via
// the API" instead.
func fetchIngressPort(ctx context.Context, client *http.Client, supervisorURL, token string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, supervisorURL+"/addons/self/info", nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("call supervisor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("supervisor returned %s", resp.Status)
	}

	var body struct {
		Data struct {
			IngressPort int `json:"ingress_port"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	if body.Data.IngressPort == 0 {
		return 0, fmt.Errorf("supervisor reported ingress_port 0")
	}
	return body.Data.IngressPort, nil
}
