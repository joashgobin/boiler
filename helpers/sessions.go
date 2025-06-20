package helpers

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/session"
)

type FlashInterface interface {
	Push(c *fiber.Ctx, message string) error
	SetOld(c *fiber.Ctx, keys []string)
	ClearOld(c *fiber.Ctx)
}

type FlashModel struct {
	Store *session.Store
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

func (flash *FlashModel) SetOld(c *fiber.Ctx, keys []string) {
	sess, err := flash.Store.Get(c)
	if err != nil {
		log.Errorf("error getting session: %v", err)
		return
	}
	oldValues := make(map[string]string, 1)
	for _, key := range keys {
		fmt.Printf("storing old: %s -> %s\n", key, c.FormValue(key))
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
		c.Locals("session", sess)
		if c.Query("error") == "" {
			c.Locals("old", sess.Get("old"))
		}
		return c.Next()
	}
}
