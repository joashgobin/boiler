# Boiler
This project is focused on providing boilerplate for a Gofiber app.

## Basic app
Create a *main.go* file and paste the following code:

```go
package main

import (
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
Note the following:
- User - the username of the linux user that will be used to log into the VPS the app is being deployed to
- IP - the domain name at which the app will be accessed via the internet when deployed
- Port - the port number the app will run on
- AppName - the name of the app will be used to create the database for the base app
- Templates - the set of view files to be embedded
- SiteInfo - general site information to be accessed in the templates using the "Get" function

We can then embed the view files into the app using go embed:

```go
package main

import (
	"embed"

	"github.com/joashgobin/boiler/core"
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

The app will likely throw an error about the database not existing. Rename the Makefile and run the database migration:
```sh
mv Makefile.example Makefile
sudo make up
```

If this is your first app using this project as your starter, run the command to create the fiber user:
```sh
sudo make user
```
