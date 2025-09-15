# Boiler
This project is focused on providing boilerplate for a Gofiber app.

## Basic app
Add the following to your **go.mod** file:
```
go 1.25.1

require (
	github.com/gofiber/fiber/v2 v2.52.9
	github.com/joashgobin/boiler v0.0.29
)

replace github.com/joashgobin/boiler => ../boiler
```

Run the following:
```sh
go get github.com/gofiber/fiber/v2
go get github.com/joashgobin/boiler
go get github.com/joashgobin/boiler/core
go get github.com/joashgobin/boiler/email
```

Create a *main.go* file and paste the following code:

```go
package main

import (
    "flag"

	"github.com/joashgobin/boiler/core"
)

func main() {
	isProd := flag.Bool("prod", false, "production mode of app (dev vs. prod)")
	flag.Parse()

	config := core.AppConfig{
		User:      "myname",
		IP:        "myapp.example.com",
		Port:      "9911",
		AppName:   "appname",
		Templates: nil,
		SiteInfo:  &map[string]string{},
		IsProduction: *isProd,
	}
	app, base := core.NewApp(config)

	app.Listen(base.Anchor)
}

```


Note the following:
- User - the username of the linux user that will be used to log into the VPS the app is being deployed to
- IP - the domain name at which the app will be accessed via the internet when deployed
- Port - the port number the app will run on
- AppName - the name of the app will be used to create the database for the base app
- Templates - the set of view files to be embedded
- SiteInfo - general site information to be accessed in the templates using the "Get" function

We can then embed the view files into the app using go embed:
```sh
mkdir -p views/layouts
touch views/layouts/main.html
touch views/index.html
```

We now need to create the database for our app. Rename the Makefile and run the database migration:
```sh
cp Makefile.example Makefile
cp .gitignore.example .gitignore
cp .air.toml.example .air.toml
sudo make up
```

Update the main.go file:
```go
package main

import (
	"embed"
    "flag"

    "github.com/gofiber/fiber/v2"
	"github.com/joashgobin/boiler/core"
)

//go:embed views/*
var templates embed.FS

func main() {
	isProd := flag.Bool("prod", false, "production mode of app (dev vs. prod)")
	flag.Parse()

	config := core.AppConfig{
		User:      "myname",
		IP:        "myapp.example.com",
		Port:      "9911",
		AppName:   "appname",
		Templates: &templates,
		SiteInfo:  &map[string]string{},
		IsProduction: *isProd,
	}
    app, base := core.NewApp(config)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Welcome!")
	})

	app.Listen(base.Anchor)
}
```

If this is your first app using this project as your starter, run the command to create the fiber user:
```sh
sudo make user
```

## Deployment to VPS
Upload the first version of the app to the VPS:
```sh
make deploy/first
```
This will upload the static assets, deploy the database, build and upload the binary, create and run the app service

Next we can deploy the nginx configuration and certbot certification with the target:
```sh
make deploy/nginx
```

For successive deployments you will only have to run:
```sh
make deploy
```

To specifically deploy the static assets or the app binary run the following respectively:
```sh
make deploy/static
make deploy/app
```

## Features
- Favicon generation
- Image optimization
- CSS minification
- Efficient caching
- Deployments to VPS:
    - Database setup
    - Static file transfer
    - Binary transfer and service setup
    - Nginx configuration and certbot certification
- User profile creation and management
- Low memory usage
- Rate limiting
