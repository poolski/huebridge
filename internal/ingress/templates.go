package ingress

import "html/template"

// Form actions are deliberately relative: Home Assistant serves this UI
// under a per-session ingress path prefix, so an absolute "/entities" would
// post outside the add-on entirely.
var indexTemplate = template.Must(template.New("index").Parse(`
<!doctype html>
<html>
<head><title>huebridge</title></head>
<body>
	<h1>huebridge</h1>

	<section>
		<h2>Pairing</h2>
		<form method="POST" action="pairing/allow">
			<button type="submit">Allow next pairing (30s)</button>
		</form>
	</section>

	<section>
		<h2>Exposed entities</h2>
		<ul>
			{{range .Entries}}<li>{{.Name}} ({{.EntityID}})</li>{{end}}
		</ul>
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
		<h2>Groups</h2>
		<ul>
			{{range .Groups}}<li>{{.Name}} ({{.Class}}): {{range $i, $n := .MemberNames}}{{if $i}}, {{end}}{{$n}}{{end}}</li>{{end}}
		</ul>
	</section>

	<section>
		<h2>Add a group</h2>
		<p>A group is what the Hue app creates scenes against, so add one before making scenes.</p>
		<form method="POST" action="groups">
			<input type="text" name="name" placeholder="Group name" required>
			<input type="text" name="class" placeholder="Room class (e.g. Living room)">
			<ul>
				{{range .Entries}}<li><label><input type="checkbox" name="entity_id" value="{{.EntityID}}"> {{.Name}} ({{.EntityID}})</label></li>{{end}}
			</ul>
			<button type="submit">Create group</button>
		</form>
	</section>
</body>
</html>
`))
