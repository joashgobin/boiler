package helpers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/google/uuid"
)

type FlashInterface interface {
	Push(c *fiber.Ctx, messages ...any) error
	// Retain(c *fiber.Ctx, keys []string)
	// RequireFields(c *fiber.Ctx, redirectRoute string, fields []string) (string, error)
	ClearOld(c *fiber.Ctx)
	Redirect(c *fiber.Ctx, route string, messages ...any) error
	// RetainKeys(keys []string) fiber.Handler
	// RequireKeys(keys []string, redirectRoute string) fiber.Handler
	Retain(keys ...string) fiber.Handler
	Require(keys ...string) fiber.Handler
	RequireRedirect(redirectRoute string, keys ...string) fiber.Handler
	Get(c *fiber.Ctx, key string, defaultValue ...any) any
	// GetUser(c *fiber.Ctx) interface{}
	Set(c *fiber.Ctx, key string, value any) error
	SetMany(c *fiber.Ctx, pairs map[string]any) error
	DeleteSession(c *fiber.Ctx)
	UploadImage(c *fiber.Ctx, imageFormName string) (string, error)
}

func GetUser[T any](c *fiber.Ctx, flash FlashInterface) T {
	user := flash.Get(c, "user")
	data, ok := user.(T)
	if ok {
		return data
	}
	var emptyUser T
	return emptyUser
}

func (flash *FlashModel) GetUser(c *fiber.Ctx) interface{} {
	sess, err := flash.Store.Get(c)
	if err != nil {
		return nil
	}
	value := sess.Get("user")
	return value
}

func (flash *FlashModel) UploadImage(c *fiber.Ctx, imageFormName string) (string, error) {
	file, err := c.FormFile(imageFormName)
	if err != nil {
		return "", err
	}
	filename := strings.Replace(uuid.New().String(), "-", "", -1)
	fileExt := strings.Split(file.Filename, ".")[1]
	image := fmt.Sprintf("%s.%s", filename, fileExt)

	err = c.SaveFile(file, fmt.Sprintf("./uploads/%s", image))
	if err != nil {
		return "", err
	}
	return image, nil
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

func (flash *FlashModel) Get(c *fiber.Ctx, key string, defaultValue ...any) any {
	sess, err := flash.Store.Get(c)
	if err != nil {
		// panic(err)
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return nil
	}
	value := sess.Get(key)
	if value == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
	}
	return value
}

func (flash *FlashModel) Set(c *fiber.Ctx, key string, value any) error {
	sess, err := flash.Store.Get(c)
	if err != nil {
		return err
	}
	sess.Set(key, value)
	if err := sess.Save(); err != nil {
		return err
	}
	return nil
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

/*
func (flash *FlashModel) RequireFields(c *fiber.Ctx, redirectRoute string, fields []string) (string, error) {
	warning, err := EnsureFiberFormFields(c, fields)
	flash.Push(c, warning)
	return redirectRoute + "?show=retained", err
}
*/

func (flash *FlashModel) Redirect(c *fiber.Ctx, route string, messages ...any) error {
	var message string
	if len(messages) > 1 {
		message = fmt.Sprintf(messages[0].(string), messages[1:]...)
	} else {
		message = messages[0].(string)
	}
	flash.Push(c, message)
	return c.Redirect(route + "?show=retained")
}

func (flash *FlashModel) Push(c *fiber.Ctx, messages ...any) error {
	sess, err := flash.Store.Get(c)
	if err != nil {
		return err
	}
	var message string
	if len(messages) > 1 {
		message = fmt.Sprintf(messages[0].(string), messages[1:]...)
	} else {
		message = messages[0].(string)
	}

	sess.Set("flashMessage", message)

	// skip clearing flash message via locals
	sess.Set("delayFlashClear", true)

	if err := sess.Save(); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return nil
}

/*
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
*/

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

func SessionLocalsMiddleware(store *session.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return err
		}

		// add session to locals
		c.Locals("session", sess)

		// add user to locals
		c.Locals("user", sess.Get("user"))

		// add old values to locals
		if c.Query("show") == "retained" {
			c.Locals("old", sess.Get("old"))
		} else {
			c.Locals("old", map[string]string{})
		}

		// pass flash message to locals if indicated by Push()
		if sess.Get("delayFlashClear") != nil {
			sess.Delete("delayFlashClear")
			c.Locals("flash", sess.Get("flashMessage"))
			if err := sess.Save(); err != nil {
				log.Infof("error resetting flash: %v", err)
			}
		} else {
			c.Locals("flash", nil)
		}

		return c.Next()
	}
}

func SessionOldValuesMiddleware(store *session.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Method() != "POST" {
			return c.Next()
		}
		sess, err := store.Get(c)
		if err != nil {
			log.Errorf("error getting session: %v", err)
		}
		oldValues := MapFromFormBody(c, true)
		sess.Set("old", oldValues)
		if err := sess.Save(); err != nil {
			log.Errorf("error saving session: %v", err)
		}
		return c.Next()
	}
}

func (flash *FlashModel) Retain(keys ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		/*
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
		*/
		return c.Next()
	}
}

func (flash *FlashModel) Require(keys ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// log.Infof("route: %v", c.OriginalURL())
		redirectRoute := c.OriginalURL()
		warning, err := EnsureFiberFormFields(c, keys)
		if err != nil {
			flash.Push(c, warning)
			return c.Redirect(redirectRoute + "?show=retained")
		}
		return c.Next()
	}
}

func (flash *FlashModel) RequireRedirect(redirectRoute string, keys ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		warning, err := EnsureFiberFormFields(c, keys)
		if err != nil {
			flash.Push(c, warning)
			return c.Redirect(redirectRoute + "?show=retained")
		}
		return c.Next()
	}
}
