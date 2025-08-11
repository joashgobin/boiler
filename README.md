# Boiler
This project is focused on providing boilerplate for a Gofiber app.

## Basic app
Create a *main.go* file and paste the following code:

```
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/joashgobin/boiler/core"
)

func main() {
	config := core.AppConfig{
		User:      "myname",
		IP:        "myapp.example.com",
		Port:      "9911",
		AppName:   "appname",
		Templates: nil,
		SiteInfo:  &map[string]string{},
	}
	app, base := core.NewApp(config)

	app.Listen(base.Anchor)
}

```

We can then embed the view files using go embed:
```
package main

import (
	"embed"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/joashgobin/boiler/core"
	"github.com/joashgobin/boiler/helpers"
)

//go:embed views/*
var templates embed.FS

func main() {
	config := core.AppConfig{
		User:      "myname",
		IP:        "myapp.example.com",
		Port:      "9911",
		AppName:   "appname",
		Templates: &templates,
		SiteInfo:  &map[string]string{},
	}
	app, base := core.NewApp(config)

	app.Listen(base.Anchor)
}

```
