package core

import (
	"database/sql"
	"embed"
	"encoding/gob"
	"fmt"
	ht "html/template"
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
	"github.com/gofiber/fiber/v2/middleware/session"
	// mysql "github.com/gofiber/storage/mysql/v2"
	"github.com/gofiber/storage/redis/v3"
)

type Base struct {
	Users models.UserModelInterface
	DB    *sql.DB
	Store *session.Store
	Shelf helpers.ShelfModelInterface
	Flash helpers.FlashInterface
}

// NewApp returns a configured fiber app with session, csrf and other middleware
func NewApp(templates *embed.FS, staticFiles *embed.FS, siteInfo *map[string]string, appName string) (*fiber.App, Base) {
	start := time.Now()
	gob.Register(map[string]string{})

	fingerprints := make(map[string]string, 3)
	optimizations := make(map[string]string, 3)

	// generate new minified style file with fingerprint in file name
	helpers.GenerateFingerprint("static/style.css", &fingerprints)

	// convert all images to webp
	helpers.ConvertInFolderToWebp("static/img", "static/gen/img", ".jpeg", &optimizations)
	helpers.ConvertInFolderToWebp("static/img", "static/gen/img", ".jpg", &optimizations)
	helpers.ConvertInFolderToWebp("static/img", "static/gen/img", ".png", &optimizations)

	// only use parent process to do file operations
	if !fiber.IsChild() {
		// create remote directory for adding migration scripts
		helpers.CreateDirectory("remote/")
		helpers.SaveTextToDirectory(strings.ReplaceAll(`
CREATE DATABASE IF NOT EXISTS <appName>;
GRANT ALL PRIVILEGES ON <appName>.* TO 'fiber_user'@'localhost';
FLUSH PRIVILEGES;

-- Verify permissions
SHOW GRANTS FOR 'fiber_user'@'localhost';
	`, "<appName>", appName), "remote/create_app_database.sql")

		helpers.SaveTextToDirectory(`
tmp/
bin/
fiber.sqlite3
static/gen/
merchants/
	`,
			".gitignore")

		helpers.CreateDirectory("views/layouts")
		helpers.CreateDirectory("views/partials")
		helpers.CreateDirectory("static/img")
		helpers.CreateDirectory("static/script")

		// get core directory
		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			fmt.Println("could not get filename")
		}
		coreDir, err := filepath.Abs(filename)
		if err != nil {
			fmt.Println("could not get filename")
		}

		// copy partials from core
		helpers.CopyDir(filepath.Dir(coreDir)+"/partials/", "views/partials/")
		helpers.CopyDir(filepath.Dir(coreDir)+"/script/", "static/script/")
		helpers.CopyDir(filepath.Dir(coreDir)+"/img/", "static/img/")
		helpers.CopyDir(filepath.Dir(coreDir)+"/air/", "")
	}

	// create template engine
	engine := html.NewFileSystem(http.FS(*templates), ".html")

	formPresets := helpers.FormPresets()
	externalPresets := helpers.ExternalPresets()

	// add functions to template engine
	engine.AddFuncMap(map[string]interface{}{
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
		"svg": func(iconName string) string {
			return "/static/img/bootstrap-icons/" + iconName + ".svg"
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
			return (*siteInfo)[key]
		},
		"get": func(key string) string {
			return (*siteInfo)[key]
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
	storage := redis.New(redis.Config{
		Host:      "127.0.0.1",
		Port:      6379,
		Username:  "",
		Password:  "",
		Database:  0,
		Reset:     false,
		TLSConfig: nil,
		PoolSize:  10 * runtime.GOMAXPROCS(0),
	})
	/*storage := mysql.New(mysql.Config{
		ConnectionURI: storageURI,
		Reset:         false,
		GCInterval:    10 * time.Second,
	})*/

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
		KeyLookup:    "cookie:fiber_session",
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
	if staticFiles != nil {
		app.Use("/static", filesystem.New(filesystem.Config{
			Root:       http.FS(*staticFiles),
			PathPrefix: "static",
			Browse:     true,
		}))
	}

	// open database corresponding to app name
	db, err := helpers.OpenDB(dbURI + appName + "?parseTime=true&multiStatements=true")
	if err != nil {
		log.Fatal(err)
		return app, Base{}
	}
	// attaching users to base
	base := Base{
		Users: &models.UserModel{DB: db},
		DB:    db,
		Store: store,
		Shelf: &helpers.ShelfModel{DB: db},
		Flash: &helpers.FlashModel{Store: store},
	}

	//
	payments.InitMMG(db, appName)
	helpers.InitShelf(db, appName)
	models.InitUsers(db, appName)

	app.Use(helpers.SessionInfoMiddleware(store))

	if !fiber.IsChild() {
		elapsed := time.Since(start)
		log.Infof("app startup time: %v\n", elapsed)
	}
	// return configured fiber app and database connection pool
	return app, base
}
