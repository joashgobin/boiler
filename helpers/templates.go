package helpers

import (
	"github.com/gofiber/fiber/v2"
)

func HTMLMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return nil
	}
}
