package core

import (
	"database/sql"
	"embed"
	ht "html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/template/html/v2"
	"github.com/joashgobin/boiler/helpers"

	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/session"
	mysql "github.com/gofiber/storage/mysql/v2"
)

// GetApp returns a configured fiber app with session, csrf and other middleware
func GetApp(templates *embed.FS, staticFiles *embed.FS, siteInfo *map[string]string, appName string) (*fiber.App, *sql.DB, *session.Store) {
	fingerprints := make(map[string]string, 3)
	optimizations := make(map[string]string, 3)

	// generate new minified style file with fingerprint in file name
	helpers.GenerateFingerprint("static/style.css", &fingerprints)

	// convert all images to webp
	helpers.ConvertInFolderToWebp("static/img", "static/gen/img", ".jpeg", &optimizations)
	helpers.ConvertInFolderToWebp("static/img", "static/gen/img", ".jpg", &optimizations)
	helpers.ConvertInFolderToWebp("static/img", "static/gen/img", ".png", &optimizations)

	// create template engine
	engine := html.NewFileSystem(http.FS(*templates), ".html")

	// add functions to template engine
	engine.AddFuncMap(map[string]interface{}{
		"Minify": func(s string) string {
			return "/" + fingerprints[s]
		},
		"Optimize": func(s string) string {
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
		"Get": func(key string) string {
			return (*siteInfo)[key]
		},
		"safeHTML": func(s string) ht.HTML {
			return ht.HTML(s)
		},
	})

	// declare database URIs
	var dbURI string = os.Getenv("FIBER_USER_URI")
	var storageURI string = dbURI + dbName + "?multiStatements=true"

	// initialize fiber storage middleware
	storage := mysql.New(mysql.Config{
		ConnectionURI: storageURI,
		Reset:         false,
		GCInterval:    10 * time.Second,
	})

	// create new fiber app with prefork enabled
	app := fiber.New(fiber.Config{
		Views:             engine,
		ViewsLayout:       "internal/core/layouts/main",
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
	db, err := helpers.OpenDB(dbURI + dbName + "?parseTime=true")
	if err != nil {
		log.Fatal(err)
		return app, nil, store
	}

	// return configured fiber app and database connection pool
	return app, db, store
}
