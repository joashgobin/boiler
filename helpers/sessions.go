package helpers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/session"
)

type FlashInterface interface {
	Push(c *fiber.Ctx, message string) error
	Retain(c *fiber.Ctx, keys []string)
	ClearOld(c *fiber.Ctx)
	Redirect(c *fiber.Ctx, route, message string) error
	RequireFields(c *fiber.Ctx, redirectRoute string, fields []string) (string, error)
	RetainKeys(keys []string) fiber.Handler
	RequireKeys(keys []string, redirectRoute string) fiber.Handler
	Get(c *fiber.Ctx, key string) string
	Set(c *fiber.Ctx, key string, value string)
	SetMany(c *fiber.Ctx, pairs map[string]any) error
	DeleteSession(c *fiber.Ctx)
}

func (flash *FlashModel) DeleteSession(c *fiber.Ctx) {
	sess, err := flash.Store.Get(c)
	if err != nil {
		log.Errorf("session delete error: %v", err)
	}
	if err := sess.Destroy(); err != nil {
		log.Errorf("session delete error: %v", err)
	}

}

type FlashModel struct {
	Store *session.Store
}

func (flash *FlashModel) Get(c *fiber.Ctx, key string) string {
	sess, err := flash.Store.Get(c)
	if err != nil {
		return ""
	}
	value := sess.Get(key)
	return value.(string)
}

func (flash *FlashModel) Set(c *fiber.Ctx, key string, value string) {
	sess, err := flash.Store.Get(c)
	if err != nil {
		panic(err)
	}
	sess.Set(key, value)
	if err := sess.Save(); err != nil {
		panic(err)
	}
}

func (flash *FlashModel) SetMany(c *fiber.Ctx, pairs map[string]any) error {
	sess, err := flash.Store.Get(c)
	if err != nil {
		return err
	}
	for key, value := range pairs {
		sess.Set(key, value)
	}
	if err := sess.Save(); err != nil {
		return err
	}
	return nil
}

func (flash *FlashModel) RequireFields(c *fiber.Ctx, redirectRoute string, fields []string) (string, error) {
	warning, err := EnsureFiberFormFields(c, fields)
	flash.Push(c, warning)
	return redirectRoute + "?show=retained", err
}

func (flash *FlashModel) Redirect(c *fiber.Ctx, route, message string) error {
	flash.Push(c, message)
	return c.Redirect(route + "?show=retained")
}

func (flash *FlashModel) Push(c *fiber.Ctx, message string) error {
	sess, err := flash.Store.Get(c)
	if err != nil {
		return err
	}
	sess.Set("flashMessage", message)
	sess.Set("flashTime", time.Now().UTC().Format("2006-01-02 15:04:05"))
	if err := sess.Save(); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return nil
}

func (flash *FlashModel) Retain(c *fiber.Ctx, keys []string) {
	sess, err := flash.Store.Get(c)
	if err != nil {
		log.Errorf("error getting session: %v", err)
		return
	}
	oldValues := make(map[string]string, 1)
	for _, key := range keys {
		// fmt.Printf("storing old: %s -> %s\n", key, c.FormValue(key))
		oldValues[key] = c.FormValue(key)
	}
	sess.Set("old", oldValues)
	if err := sess.Save(); err != nil {
		log.Errorf("error saving session: %v", err)
		return
	}
}

func (flash *FlashModel) ClearOld(c *fiber.Ctx) {
	sess, err := flash.Store.Get(c)
	if err != nil {
		return
	}
	sess.Set("old", nil)
	if err := sess.Save(); err != nil {
		return
	}
}

func SessionInfoMiddleware(store *session.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		c.Locals("roles", sess.Get("userRoles"))
		c.Locals("session", sess)
		if c.Query("show") == "retained" {
			c.Locals("old", sess.Get("old"))
		} else {
			c.Locals("old", map[string]string{})
		}
		return c.Next()
	}
}

func (flash *FlashModel) RetainKeys(keys []string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := flash.Store.Get(c)
		if err != nil {
			log.Errorf("error getting session: %v", err)
		}
		oldValues := make(map[string]string, 1)
		for _, key := range keys {
			oldValues[key] = c.FormValue(key)
		}
		sess.Set("old", oldValues)
		if err := sess.Save(); err != nil {
			log.Errorf("error saving session: %v", err)
		}
		return c.Next()
	}
}

func (flash *FlashModel) RequireKeys(keys []string, redirectRoute string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		warning, err := EnsureFiberFormFields(c, keys)
		if err != nil {
			flash.Push(c, warning)
			return c.Redirect(redirectRoute + "?show=retained")
		}
		return c.Next()
	}
}
