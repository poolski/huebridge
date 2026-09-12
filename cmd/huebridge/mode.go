package main

// isStandalone reports whether huebridge is running outside a Home
// Assistant Supervisor add-on container. Supervisor always injects
// SUPERVISOR_TOKEN into add-ons it starts, so its absence reliably means
// there's no Supervisor API to call for the ingress port and no ingress
// proxy providing auth for the admin UI — i.e. standalone mode.
func isStandalone(env func(string) string) bool {
	return env("SUPERVISOR_TOKEN") == ""
}
