package ingress

import "html/template"

// Form actions are deliberately relative: Home Assistant serves this UI
// under a per-session ingress path prefix, so an absolute "/entities" would
// post outside the add-on entirely.
var indexTemplate = template.Must(template.New("index").Parse(`
<!doctype html>
<html>
<head>
	<title>huebridge</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		:root {
			color-scheme: light dark;
			--bg: #f0f1f5;
			--card-bg: #ffffff;
			--text: #1c1c1e;
			--text-muted: #6c6c70;
			--border: #e2e2e6;
			--accent: #ff9800;
			--accent-text: #1c1c1e;
		}
		@media (prefers-color-scheme: dark) {
			:root {
				--bg: #111214;
				--card-bg: #1d1e21;
				--text: #f2f2f2;
				--text-muted: #a0a0a5;
				--border: #313235;
				--accent: #ffb74d;
				--accent-text: #1c1c1e;
			}
		}
		* { box-sizing: border-box; }
		body {
			margin: 0;
			padding: 24px 16px 48px;
			background: var(--bg);
			color: var(--text);
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
		}
		main {
			max-width: 640px;
			margin: 0 auto;
			display: flex;
			flex-direction: column;
			gap: 16px;
		}
		h1 {
			font-size: 1.5rem;
			margin: 0 0 8px;
		}
		section {
			background: var(--card-bg);
			border: 1px solid var(--border);
			border-radius: 12px;
			padding: 20px;
			box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
		}
		h2 {
			font-size: 1.05rem;
			margin: 0 0 12px;
		}
		p.hint {
			color: var(--text-muted);
			font-size: 0.9rem;
			margin: -4px 0 12px;
		}
		ul.plain {
			list-style: none;
			margin: 0;
			padding: 0;
			display: flex;
			flex-direction: column;
			gap: 8px;
		}
		ul.plain li {
			display: flex;
			justify-content: space-between;
			gap: 12px;
			padding: 10px 12px;
			background: var(--bg);
			border-radius: 8px;
			font-size: 0.95rem;
		}
		ul.plain li span.id {
			color: var(--text-muted);
			font-size: 0.85rem;
		}
		ul.checklist {
			list-style: none;
			margin: 0 0 16px;
			padding: 0;
			display: flex;
			flex-direction: column;
			gap: 4px;
			max-height: 220px;
			overflow-y: auto;
		}
		ul.checklist label {
			display: flex;
			align-items: center;
			gap: 8px;
			padding: 6px 4px;
			font-size: 0.9rem;
			border-radius: 6px;
		}
		ul.checklist label:hover {
			background: var(--bg);
		}
		form {
			display: flex;
			flex-wrap: wrap;
			gap: 8px;
			align-items: center;
		}
		input[type="text"], select {
			flex: 1 1 160px;
			padding: 10px 12px;
			border: 1px solid var(--border);
			border-radius: 8px;
			background: var(--card-bg);
			color: var(--text);
			font-size: 0.95rem;
		}
		button {
			padding: 10px 18px;
			border: none;
			border-radius: 8px;
			background: var(--accent);
			color: var(--accent-text);
			font-size: 0.95rem;
			font-weight: 600;
			cursor: pointer;
		}
		button:hover {
			filter: brightness(0.95);
		}
		.empty {
			color: var(--text-muted);
			font-size: 0.9rem;
			margin: 0;
		}
		ul.plain li form {
			gap: 0;
			flex: 0 0 auto;
		}
		button.delete {
			padding: 0;
			width: 24px;
			height: 24px;
			line-height: 1;
			border-radius: 50%;
			background: transparent;
			color: var(--text-muted);
			font-size: 1.1rem;
			font-weight: 400;
		}
		button.delete:hover {
			background: #e5484d33;
			color: #e5484d;
			filter: none;
		}
	</style>
</head>
<body>
	<main>
		<h1>huebridge</h1>

		<section>
			<h2>Pairing</h2>
			<form method="POST" action="pairing/allow">
				<button type="submit">Allow next pairing (30s)</button>
			</form>
		</section>

		<section>
			<h2>Exposed entities</h2>
			{{if .Entries}}
			<ul class="plain">
				{{range .Entries}}<li>
					<span>{{.Name}} <span class="id">{{.EntityID}}</span></span>
					<form method="POST" action="entities/{{.EntityID}}/delete">
						<button type="submit" class="delete" title="Remove {{.Name}}" aria-label="Remove {{.Name}}">&times;</button>
					</form>
				</li>{{end}}
			</ul>
			{{else}}
			<p class="empty">No entities exposed yet.</p>
			{{end}}
		</section>

		<section>
			<h2>Add an entity</h2>
			<form method="POST" action="entities">
				<select name="entity_id">
					{{range .Available}}<option value="{{.}}">{{.}}</option>{{end}}
				</select>
				<input type="text" name="name" placeholder="Display name" required>
				<button type="submit">Add</button>
			</form>
		</section>

		<section>
			<h2>Bridge version</h2>
			<p class="hint">Reported to the Hue app via GET /api/config. Only versions real Signify bridges have shipped are offered — an unrecognized combination makes the official app nag for an update it can never deliver.</p>
			<p>Current: <strong>datastoreversion {{.CurrentVersion.DatastoreVersion}}, swversion {{.CurrentVersion.SwVersion}}, apiversion {{.CurrentVersion.APIVersion}}</strong></p>
			<form method="POST" action="version">
				<select name="version">
					{{$current := .CurrentVersion}}{{range .KnownVersions}}<option value="{{.DatastoreVersion}}|{{.SwVersion}}|{{.APIVersion}}"{{if and (eq .DatastoreVersion $current.DatastoreVersion) (eq .SwVersion $current.SwVersion) (eq .APIVersion $current.APIVersion)}} selected{{end}}>apiversion {{.APIVersion}} (swversion {{.SwVersion}}, datastoreversion {{.DatastoreVersion}})</option>{{end}}
				</select>
				<button type="submit">Set version</button>
			</form>
		</section>

		<section>
			<h2>Groups</h2>
			{{if .Groups}}
			<ul class="plain">
				{{range .Groups}}<li>{{.Name}} <span class="id">({{.Class}}): {{range $i, $n := .MemberNames}}{{if $i}}, {{end}}{{$n}}{{end}}</span></li>{{end}}
			</ul>
			{{else}}
			<p class="empty">No groups yet.</p>
			{{end}}
		</section>

		<section>
			<h2>Add a group</h2>
			<p class="hint">A group is what the Hue app creates scenes against, so add one before making scenes.</p>
			<form method="POST" action="groups">
				<input type="text" name="name" placeholder="Group name" required>
				<input type="text" name="class" placeholder="Room class (e.g. Living room)">
				<ul class="checklist">
					{{range .Entries}}<li><label><input type="checkbox" name="entity_id" value="{{.EntityID}}"> {{.Name}} ({{.EntityID}})</label></li>{{end}}
				</ul>
				<button type="submit">Create group</button>
			</form>
		</section>
	</main>
</body>
</html>
`))
