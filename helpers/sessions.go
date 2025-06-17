package helpers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/session"
)

type FlashInterface interface {
	Push(c *fiber.Ctx, message string) error
}

type FlashModel struct {
	Store *session.Store
}

func (flash *FlashModel) Push(c *fiber.Ctx, message string) error {
	log.Infof("pushing to session: %s", message)
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

func SessionInfoMiddleware(store *session.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}
		c.Locals("session", sess)
		return c.Next()
	}
}
