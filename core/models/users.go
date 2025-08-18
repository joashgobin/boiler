package models

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/joashgobin/boiler/helpers"
	"golang.org/x/crypto/bcrypt"
)

type UserModelInterface interface {
	Insert(name, email, password string) error
	Authenticate(email, password string) (int, string, error)
	Exists(id int) (bool, error)
	AssignRole(id int, role string) error
	RemoveRole(id int, role string) error
}

type User struct {
	ID             int
	Name           string
	Email          string
	Roles          string
	HashedPassword []byte
	Created        time.Time
}

type UserModel struct {
	DB *sql.DB
}

func InitUsers(db *sql.DB, appName string) {
	helpers.RunMigration(strings.ReplaceAll(`
	USE <appName>;

CREATE TABLE IF NOT EXISTS users (
    id INTEGER NOT NULL PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    roles VARCHAR(255) NOT NULL,
    hashed_password CHAR(60) NOT NULL,
    created DATETIME NOT NULL,
	UNIQUE KEY users_uc_email (email)
);

	`, "<appName>", appName), db)
}

func (m *UserModel) Insert(name, email, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	stmt := `INSERT INTO users (name, email, roles, hashed_password, created)
    VALUES(?, ?, ?, ?, UTC_TIMESTAMP())`
	_, err = m.DB.Exec(stmt, name, email, "|user|", string(hashedPassword))
	if err != nil {
		var mySQLError *mysql.MySQLError
		if errors.As(err, &mySQLError) {
			if mySQLError.Number == 1062 && strings.Contains(mySQLError.Message, "users_uc_email") {
				return ErrDuplicateEmail
			}
		}
		return err
	}
	return nil
}

func (m *UserModel) AssignRole(id int, role string) error {
	selectStmt := `SELECT email, roles FROM users WHERE id = ?`

	var roles string
	var email string
	err := m.DB.QueryRow(selectStmt, id).Scan(&email, &roles)
	if err != nil {
		return err
	}

	newRoles := roles
	if !strings.Contains(roles, "|"+role+"|") {
		newRoles += "|" + role + "|"
		log.Infof("setting new roles to: %s", newRoles)
		updateStmt := `UPDATE users
		SET roles = ?
		WHERE id = ?
		`
		result, err := m.DB.Exec(updateStmt, newRoles, id)
		if err != nil {
			return err
		}
		_, err = result.RowsAffected()
		if err != nil {
			return err
		}
		log.Infof("updated roles to: %s", newRoles)
	}

	return nil
}

func (m *UserModel) RemoveRole(id int, role string) error {
	selectStmt := `SELECT email, roles FROM users WHERE id = ?`

	var roles string
	var email string
	err := m.DB.QueryRow(selectStmt, id).Scan(&email, &roles)
	if err != nil {
		return err
	}

	log.Infof("%s has roles: %s", email, roles)

	newRoles := roles
	if strings.Contains(roles, "|"+role+"|") {
		newRoles = strings.ReplaceAll(newRoles, "|"+role+"|", "")
		log.Infof("setting new roles to: %s", newRoles)
		updateStmt := `UPDATE users
		SET roles = ?
		WHERE id = ?
		`
		result, err := m.DB.Exec(updateStmt, newRoles, id)
		if err != nil {
			return err
		}
		_, err = result.RowsAffected()
		if err != nil {
			return err
		}
		log.Infof("updated roles to: %s", newRoles)
	}

	return nil
}

func (m *UserModel) Authenticate(email, password string) (int, string, error) {
	var id int
	var hashedPassword []byte
	var roles string
	stmt := "SELECT id, roles, hashed_password FROM users WHERE email = ?"
	err := m.DB.QueryRow(stmt, email).Scan(&id, &roles, &hashedPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "", ErrInvalidCredentials
		} else {
			return 0, "", err
		}
	}
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return 0, "", ErrInvalidCredentials
		} else {
			return 0, "", err
		}
	}
	return id, roles, nil
}

func (m *UserModel) Exists(id int) (bool, error) {
	var exists bool
	stmt := "SELECT EXISTS(SELECT true FROM users WHERE id = ?)"
	err := m.DB.QueryRow(stmt, id).Scan(&exists)
	return exists, err
}

func RequireRoleMiddleware(store *session.Store, flash helpers.FlashInterface, role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		user := sess.Get("user")
		roles := sess.Get("userRoles")

		// redirect if user value is not set in session
		if user == nil {
			flash.Push(c, "You need to be logged in")
			return c.Redirect("/login")
		}

		// redirect if user roles are not defined in session
		if roles == nil {
			flash.Push(c, "You need to be logged in")
			return c.Redirect("/login")
		}

		// redirect if user session does not specify the required role
		if !strings.Contains(roles.(string), "|"+role+"|") {
			flash.Push(c, fmt.Sprintf("You need to be logged in as %s", role))
			return c.Redirect("/")
		}
		return c.Next()
	}
}
