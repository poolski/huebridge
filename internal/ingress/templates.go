package ingress

import "html/template"

var indexTemplate = template.Must(template.New("index").Parse(`
<!doctype html>
<html>
<head><title>huebridge</title></head>
<body>
	<h1>huebridge</h1>

	<section>
		<h2>Pairing</h2>
		<form method="POST" action="/pairing/allow">
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
		<form method="POST" action="/entities">
			<select name="entity_id">
				{{range .Available}}<option value="{{.}}">{{.}}</option>{{end}}
			</select>
			<input type="text" name="name" placeholder="Display name" required>
			<button type="submit">Add</button>
		</form>
	</section>
</body>
</html>
`))
