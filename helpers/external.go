package helpers

func ExternalPresets() map[string]string {
	presets := make(map[string]string)
	presets["bootstrap-css"] = `
	<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.6/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-4Q6Gf2aSP4eDXB8Miphtr37CMZZQ5oXLH2yaXMJ2w8e2ZtHTl7GptT4jmndRuHDT" crossorigin="anonymous">
	`
	presets["bootstrap-js"] = `
	<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.6/dist/js/bootstrap.bundle.min.js" integrity="sha384-j1CDi7MgGQ12Z7Qab0qlWQ/Qqz24Gc6BM0thvEMVjHnfYGF0rmFCozFSxQBxwHKO" crossorigin="anonymous"></script>
	`
	presets["pico-css"] = `
	<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
	`
	presets["pico-classless-css"] = `
	<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.classless.min.css">
	`

	presets["simple-css"] = `
	<link rel="stylesheet" href="https://unpkg.com/simpledotcss/simple.min.css">
	`

	presets["htmx"] = `<script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.6/dist/htmx.min.js"></script>`

	presets["htmx-preload"] = `<script defer src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.6/dist/htmx.min.js" integrity="sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+" crossorigin="anonymous"></script>
    <script defer src="https://cdn.jsdelivr.net/npm/htmx-ext-preload@2.1.0/preload.js" integrity="sha384-fkzubQiTB69M7XTToqW6tplvxAOJkqPl5JmLAbumV2EacmuJb8xEP9KnJafk/rg8" crossorigin="anonymous"></script>`

	return presets
}
