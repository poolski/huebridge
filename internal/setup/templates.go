package setup

import "html/template"

var passwordTemplate = template.Must(template.New("password").Parse(`
<!doctype html>
<html>
<head>
	<title>huebridge setup</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		:root { color-scheme: light dark; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
		body { max-width: 480px; margin: 48px auto; padding: 0 16px; }
		label { display: block; margin: 16px 0 4px; }
		input { width: 100%; padding: 8px; font-size: 1rem; }
		button { margin-top: 20px; padding: 10px 20px; font-size: 1rem; }
		.error { color: #d32f2f; }
	</style>
</head>
<body>
	<h1>Set up huebridge</h1>
	<p>Step 1 of 2 — choose an admin password. This protects the setup wizard and entity picker; huebridge has no Home Assistant ingress panel to rely on for that here.</p>
	{{if .Error}}<p class="error">{{.Error}}</p>{{end}}
	<form method="post" action="/setup/password">
		<label for="password">Password</label>
		<input type="password" id="password" name="password" required>
		<label for="confirm">Confirm password</label>
		<input type="password" id="confirm" name="confirm" required>
		<button type="submit">Continue</button>
	</form>
</body>
</html>
`))

type haFormData struct {
	Error         string
	DiscoveredURL string
}

var haTemplate = template.Must(template.New("homeassistant").Parse(`
<!doctype html>
<html>
<head>
	<title>huebridge setup</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		:root { color-scheme: light dark; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
		body { max-width: 480px; margin: 48px auto; padding: 0 16px; }
		label { display: block; margin: 16px 0 4px; }
		input { width: 100%; padding: 8px; font-size: 1rem; }
		button { margin-top: 20px; padding: 10px 20px; font-size: 1rem; }
		.error { color: #d32f2f; }
		.hint { color: #6c6c70; font-size: 0.9rem; }
	</style>
</head>
<body>
	<h1>Connect to Home Assistant</h1>
	<p>Step 2 of 2 — enter your Home Assistant base URL and a long-lived access token (Profile &rarr; Security &rarr; Long-Lived Access Tokens in Home Assistant).</p>
	{{if .Error}}<p class="error">{{.Error}}</p>{{end}}
	<form method="post" action="/setup/homeassistant">
		<label for="ha_url">Home Assistant URL</label>
		<input type="url" id="ha_url" name="ha_url" placeholder="http://homeassistant.local:8123" value="{{.DiscoveredURL}}" required>
		{{if .DiscoveredURL}}<p class="hint">Found via mDNS discovery — edit if this isn't right.</p>{{end}}
		<label for="ha_token">Long-lived access token</label>
		<input type="password" id="ha_token" name="ha_token" required>
		<button type="submit">Finish setup</button>
	</form>
</body>
</html>
`))

var completeTemplate = template.Must(template.New("complete").Parse(`
<!doctype html>
<html>
<head>
	<title>huebridge setup</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<meta http-equiv="refresh" content="4;url=/">
	<style>
		:root { color-scheme: light dark; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
		body { max-width: 480px; margin: 48px auto; padding: 0 16px; }
		.hint { color: #6c6c70; font-size: 0.9rem; }
	</style>
</head>
<body>
	<h1>Setup complete</h1>
	<p>huebridge is starting. This page will redirect to the entity picker shortly &mdash; if it doesn't, <a href="/">open it manually</a>.</p>
	<p class="hint">This restarts the server on a new certificate, so your browser will likely show a security warning once more &mdash; that's expected for a self-signed bridge; proceed as before.</p>
</body>
</html>
`))
