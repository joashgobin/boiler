package core

import (
	"database/sql"
	"embed"
	"encoding/gob"
	"fmt"
	ht "html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/template/html/v2"
	"github.com/joashgobin/boiler/core/models"
	"github.com/joashgobin/boiler/helpers"
	"github.com/joashgobin/boiler/payments"

	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/valkey"
)

type Base struct {
	Users  models.UserModelInterface
	DB     *sql.DB
	Store  *session.Store
	Shelf  helpers.ShelfModelInterface
	Flash  helpers.FlashInterface
	Bank   *valkey.Storage
	MMG    payments.MMGInterface
	Anchor string
}

type AppConfig struct {
	User        string
	IP          string
	Port        string
	AppName     string
	Templates   *embed.FS
	StaticFiles *embed.FS
	SiteInfo    *map[string]string
}

// NewApp returns a configured fiber app with session, csrf and other middleware
func NewApp(config AppConfig) (*fiber.App, Base) {
	if config.User == "" {
		fmt.Println("config error: user not specified e.g. john")
		os.Exit(1)
	}
	if config.IP == "" {
		fmt.Println("config error: IP not specified e.g. example.com")
		os.Exit(1)
	}
	if config.Port == "" {
		fmt.Println("config error: port not specified e.g. 9910")
		os.Exit(1)
	}
	if config.AppName == "" {
		fmt.Println("config error: app name not specified e.g. myapp")
		os.Exit(1)
	}

	start := time.Now()
	gob.Register(map[string]string{})

	fingerprints := make(map[string]string, 3)
	optimizations := make(map[string]string, 3)

	// generate new minified style file with fingerprint in file name
	helpers.GenerateFingerprintsForFolder("static", "static/gen", ".css", &fingerprints)

	// convert all images to webp
	helpers.ConvertInFolderToWebp("static/img", "static/gen/img", ".jpeg", &optimizations)
	helpers.ConvertInFolderToWebp("static/img", "static/gen/img", ".jpg", &optimizations)
	helpers.ConvertInFolderToWebp("static/img", "static/gen/img", ".png", &optimizations)

	// only use parent process to do file operations
	if !fiber.IsChild() {
		// get core directory
		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			fmt.Println("could not get filename")
		}
		coreDir, err := filepath.Abs(filename)
		if err != nil {
			fmt.Println("could not get filename")
		}

		// create remote directory for adding migration scripts
		helpers.CreateDirectory("remote/")

		// create Makefile and service file for deployment on remote machine
		helpers.FileSubstitute(filepath.Dir(coreDir)+"/Makefile", "Makefile.example", map[string]string{
			"user":    config.User,
			"appName": config.AppName,
			"ip":      config.IP,
			"port":    config.Port,
		})
		helpers.FileSubstitute(filepath.Dir(coreDir)+"/example.nginx", fmt.Sprintf("remote/%s.nginx", config.IP), map[string]string{
			"user":    config.User,
			"appName": config.AppName,
			"ip":      config.IP,
			"port":    config.Port,
		})
		helpers.FileSubstitute(filepath.Dir(coreDir)+"/example.service", fmt.Sprintf("remote/%s.service", config.AppName), map[string]string{
			"user":    config.User,
			"appName": config.AppName,
			"ip":      config.IP,
		})
		helpers.FileSubstitute(filepath.Dir(coreDir)+"/air/.air.toml", ".air.toml.example", map[string]string{
			"port": config.Port,
		})
		helpers.SaveTextToDirectory(strings.ReplaceAll(`
CREATE DATABASE IF NOT EXISTS <appName>;
GRANT ALL PRIVILEGES ON <appName>.* TO 'fiber_user'@'localhost';
FLUSH PRIVILEGES;

-- Verify permissions
SHOW GRANTS FOR 'fiber_user'@'localhost';
	`, "<appName>", config.AppName), "remote/create_app_database.sql")

		/*
					helpers.SaveTextToDirectory(strings.ReplaceAll(`
			-- First, ensure the event scheduler is enabled
			SET GLOBAL event_scheduler = ON;

			-- Select database
			USE <appName>;

			-- Create the event
			CREATE EVENT IF NOT EXISTS cleanup_pending_mmg_purchases
			ON SCHEDULE EVERY 1 MINUTE
			DO
			DELETE FROM purchases WHERE status = 'pending' AND timestamp < NOW() - INTERVAL 5 MINUTE;
			`, "<appName>", config.AppName), "remote/create_mmg_events.sql")
		*/

		helpers.SaveTextToDirectory(`
	-- Create fiber user
CREATE USER IF NOT EXISTS 'fiber_user'@'localhost' IDENTIFIED BY 'USER_PWD';

-- Create fiber database
CREATE DATABASE IF NOT EXISTS fiber;
USE fiber;

-- Grant privileges to the fiber user
GRANT ALL PRIVILEGES ON fiber.* TO 'fiber_user'@'localhost';
FLUSH PRIVILEGES;

	`, "remote/create_fiber_user.sql")

		helpers.SaveTextToDirectory(`
tmp/
bin/
fiber.sqlite3
static/gen/
merchants/
	`,
			".gitignore.example")

		helpers.SaveTextToDirectory(`
			read -p "Enter password for user: " DB_PASSWORD
echo "Setting environment variable FIBER_USER_URI"
grep -q FIBER_USER_URI /etc/environment || echo "FIBER_USER_URI='fiber_user:${DB_PASSWORD}@tcp(localhost:3306)/'" | sudo tee -a /etc/environment
grep -q FIBER_USER_URI ~/.bashrc || echo "export FIBER_USER_URI='fiber_user:${DB_PASSWORD}@tcp(localhost:3306)/'" | sudo tee -a ~/.bashrc
cat ./remote/create_fiber_user.sql | sed "s/USER_PWD/$DB_PASSWORD/g" | sudo mysql
exec bash

			`, "remote/create_fiber_user.sh")
		helpers.CreateDirectory("views/layouts")
		helpers.CreateDirectory("views/partials")
		helpers.CreateDirectory("static/img")
		helpers.CreateDirectory("static/script")

		// copy partials from core
		helpers.CopyDir(filepath.Dir(coreDir)+"/partials/", "views/partials/")
		helpers.CopyDir(filepath.Dir(coreDir)+"/script/", "static/script/")
		helpers.CopyDir(filepath.Dir(coreDir)+"/img/", "static/img/")
		// helpers.CopyDir(filepath.Dir(coreDir)+"/air/", "")
	}

	// create template engine
	engine := html.New("views/", ".html")
	if config.Templates != nil {
		engine = html.NewFileSystem(http.FS(*config.Templates), ".html")
	}

	formPresets := helpers.FormPresets()
	externalPresets := helpers.ExternalPresets()

	// add functions to template engine
	engine.AddFuncMap(map[string]interface{}{
		"humanDate": func(t time.Time) string {
			return t.UTC().Format("Jan 02, 2006")
		},
		"gfont": func(fontName string, selector string) ht.HTML {
			return ht.HTML(`<style>
@import url('https://fonts.googleapis.com/css2?family=` + strings.ReplaceAll(fontName, " ", "+") + `&display=swap');
` + selector + `{
	font-family: ` + fontName + `, sans-serif;
}
</style>`)
		},
		"role": func(roles interface{}, role string) bool {
			if roles == nil {
				return false
			}
			return strings.Contains(roles.(string), "|"+role+"|")
		},
		"default": func(def string, value interface{}) interface{} {
			if value == nil {
				return def
			}
			return value
		},
		"svg": func(iconName string) ht.HTML {
			return ht.HTML(`
			<script
    class="script-tag"
    data-svg-src="/static/img/bootstrap-icons/` + iconName + `.svg"
    hx-get="/static/img/bootstrap-icons/` + iconName + `.svg"
    hx-swap="outerHTML"
    hx-trigger="load">
</script>
			`)
		},
		"icon": func(iconName string) ht.HTML {
			return ht.HTML(`
			<div style="min-width:20px;min-height:20px;display:flex;align-items:center;justify-content:center;">
			<script
    class="script-tag"
    data-svg-src="/static/img/bootstrap-icons/` + iconName + `.svg"
    hx-get="/static/img/bootstrap-icons/` + iconName + `.svg"
    hx-swap="outerHTML"
    hx-trigger="load">
</script>
</div>
			`)
		},
		"ct": func() time.Time {
			return time.Now()
		},
		"input": func(key string) ht.HTML {
			return ht.HTML(formPresets[key])
		},
		"preset": func(key string) ht.HTML {
			return ht.HTML(externalPresets[key])
		},
		"extern": func(key string) ht.HTML {
			return ht.HTML(externalPresets[key])
		},
		"Minify": func(s string) string {
			return "/" + fingerprints[s]
		},
		"Min": func(s string) string {
			return "/" + fingerprints[s]
		},
		"minify": func(s string) string {
			return "/" + fingerprints[s]
		},
		"min": func(s string) string {
			return "/" + fingerprints[s]
		},
		"Optimize": func(s string) string {
			return "/" + optimizations[s]
		},
		"Opt": func(s string) string {
			return "/" + optimizations[s]
		},
		"optimize": func(s string) string {
			return "/" + optimizations[s]
		},
		"opt": func(s string) string {
			return "/" + optimizations[s]
		},
		"ToUpper": func(s string) string {
			return strings.ToUpper(s)
		},
		"ToLower": func(s string) string {
			return strings.ToLower(s)
		},
		"in": func(outer string, inner string) bool {
			return strings.Contains(outer, inner)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"split": func(str, delim string) []string {
			return strings.Split(str, delim)
		},
		"Condense": func(str string) string {
			return helpers.ReplaceSpecial(str)
		},
		"condense": func(str string) string {
			return helpers.ReplaceSpecial(str)
		},
		"Get": func(key string) string {
			val, exists := (*config.SiteInfo)[key]
			if exists {
				return val
			}
			return "<" + key + ">"
		},
		"get": func(key string) string {
			val, exists := (*config.SiteInfo)[key]
			if exists {
				return val
			}
			return "<" + key + ">"
		},
		"Use": func(values map[string]string, key string) string {
			value, exists := values[key]
			if exists {
				return value
			}
			return ""
		},
		"use": func(values map[string]string, key string) string {
			value, exists := values[key]
			if exists {
				return value
			}
			return ""
		},
		"safeHTML": func(s string) ht.HTML {
			return ht.HTML(s)
		},
	})

	// declare database URIs
	var dbURI string = os.Getenv("FIBER_USER_URI")
	// var storageURI string = dbURI + appName + "?multiStatements=true"

	// initialize fiber storage middleware
	storage := valkey.New(valkey.Config{
		InitAddress: []string{"localhost:6379"},
		Username:    "",
		Password:    "",
		SelectDB:    0,
		Reset:       false,
		TLSConfig:   nil,
	})

	// create new fiber app with prefork enabled
	app := fiber.New(fiber.Config{
		Views:             engine,
		ViewsLayout:       "views/layouts/main",
		PassLocalsToViews: true,
		Prefork:           true,
	})

	// initialize fiber session middleware
	sessConfig := session.Config{
		Expiration: 30 * time.Minute,
		// KeyLookup:      "cookie:__Host-session", // Recommended to use the __Host- prefix when serving the app over TLS
		KeyLookup:    "cookie:" + config.AppName + "_fiber_session",
		CookieSecure: true,
		// CookieHTTPOnly: true,
		CookieHTTPOnly: false,
		CookieSameSite: "Lax",
		Storage:        storage,
	}
	store := session.New(sessConfig)

	// create csrf handler
	csrfErrorHandler := func(c *fiber.Ctx, err error) error {
		log.Infof("CSRF Error: %v Request: %v From: %v\n", err, c.OriginalURL(), c.IP())

		// check accepted content types
		switch c.Accepts("html", "json") {
		case "json":
			// return a 403 Forbidden response for JSON requests
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "403 Forbidden",
			})
		case "html":
			// return a 403 Forbidden response for HTML requests
			return c.Status(fiber.StatusForbidden).Render("views/error", fiber.Map{
				"Title":     "Error",
				"Error":     "403 Forbidden",
				"ErrorCode": "403",
			})
		default:
			// return a 403 Forbidden response for all other requests
			return c.Status(fiber.StatusForbidden).SendString("403 Forbidden")
		}
	}

	// initialize fiber csrf middleware
	csrfMiddleware := csrf.New(csrf.Config{
		Session:   store,
		KeyLookup: "form:csrf",
		// CookieName:     "__Host-csrf", // Recommended to use the __Host- prefix when serving the app over TLS
		CookieName:     "csrf", // Recommended to use the __Host- prefix when serving the app over TLS
		CookieSameSite: "Lax",  // Recommended to set this to Lax or Strict
		CookieSecure:   true,   // Recommended to set to true when serving the app over TLS
		// CookieHTTPOnly: true,   // Recommended, otherwise if using JS framework recomend: false and KeyLookup: "header:X-CSRF-Token"
		CookieHTTPOnly: false, // Recommended, otherwise if using JS framework recomend: false and KeyLookup: "header:X-CSRF-Token"
		ContextKey:     "csrf",
		ErrorHandler:   csrfErrorHandler,
		Expiration:     30 * time.Minute,
		Storage:        storage,
		SingleUseToken: true,
	})

	app.Use(csrfMiddleware)

	// configure fiber logger format
	app.Use(logger.New(logger.Config{
		Format: "[${ip}]:${port} ${status} - ${method} ${path}\n",
	}))
	f, err := os.OpenFile(config.AppName+".log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	iw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(iw)

	// serve static files when in development
	app.Static("/static/", "./static", fiber.Static{
		Compress:      true,
		ByteRange:     true,
		Browse:        true,
		Index:         "index.html",
		CacheDuration: 31536000 * time.Second,
		MaxAge:        31536000,
	})

	// embed static files if provided
	if config.StaticFiles != nil {
		app.Use("/static", filesystem.New(filesystem.Config{
			Root:       http.FS(*config.StaticFiles),
			PathPrefix: "static",
			Browse:     true,
		}))
	}

	if config.Templates == nil {
		if !helpers.FileExists("views/index.html") {
			helpers.TouchFile("views/index.html")
		}
		if !helpers.FileExists("views/layouts/main.html") {
			helpers.SaveTextToDirectory(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>App</title>
</head>
<body>
<header></header>
<main>
{{embed}}
</main>
<footer></footer>
</body>
</html>
			`, "views/layouts/main.html")
		}
	}

	// open database corresponding to app name
	db, err := helpers.OpenDB(dbURI + config.AppName + "?parseTime=true&multiStatements=true")
	if err != nil {
		log.Fatal(err)
		return app, Base{}
	}
	// attaching users to base
	base := Base{
		Users:  &models.UserModel{DB: db},
		DB:     db,
		Store:  store,
		Shelf:  &helpers.ShelfModel{DB: db},
		Flash:  &helpers.FlashModel{Store: store},
		Bank:   storage,
		MMG:    &payments.MMGModel{DB: db, Merchants: map[int]string{}, Products: map[string]string{}},
		Anchor: ":" + config.Port,
	}

	//
	payments.UseMMG(db, config.AppName)
	helpers.InitShelf(db, config.AppName)
	models.InitUsers(db, config.AppName)

	app.Use(pprof.New(pprof.Config{Prefix: "/profiler"}))
	app.Get("/metrics", monitor.New())

	app.Use(helpers.SessionInfoMiddleware(store))

	if !fiber.IsChild() {
		elapsed := time.Since(start)
		log.Infof("app startup time: %v\n", elapsed)
	}
	// return configured fiber app and database connection pool
	return app, base
}
