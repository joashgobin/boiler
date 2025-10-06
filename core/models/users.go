package models

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
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
	Authenticate(email, password string) (User, error)
	EmailAuthenticate(email string) (User, error)
	Exists(email string) (bool, error)
	AssignRole(email, role string) error
	RemoveRole(email, role string) error
	ParseFromCSV(path string) error
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

func (m *UserModel) ParseFromCSV(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		return err
	}
	for _, record := range records {
		if len(record) >= 3 {
			m.Insert(record[0], record[1], "")
			for _, role := range strings.Split(record[2], ";") {
				m.AssignRole(record[1], role)
			}
		}
	}
	return nil
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

func (m *UserModel) AssignRole(email, role string) error {
	selectStmt := `SELECT id, roles FROM users WHERE email = ?`

	var roles string
	var id int

	err := m.DB.QueryRow(selectStmt, email).Scan(&id, &roles)

	if err != nil {
		return err
	}

	newRoles := roles
	if !strings.Contains(roles, "|"+role+"|") {
		newRoles += "|" + role + "|"

		log.Infof("setting new roles to: %s", newRoles)

		updateStmt := `UPDATE users
		SET roles = ?
		WHERE email = ?
		`
		result, err := m.DB.Exec(updateStmt, newRoles, email)
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

func (m *UserModel) RemoveRole(email, role string) error {
	selectStmt := `SELECT id, roles FROM users WHERE email = ?`

	var roles string
	var id int

	err := m.DB.QueryRow(selectStmt, id).Scan(&id, &roles)

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
		WHERE email = ?
		`
		result, err := m.DB.Exec(updateStmt, newRoles, email)
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

func (m *UserModel) EmailAuthenticate(email string) (User, error) {
	var user User
	stmt := "SELECT id, name, roles FROM users WHERE email = ?"
	err := m.DB.QueryRow(stmt, email).Scan(&user.ID, &user.Name, &user.Roles)
	if err != nil {
		return User{}, err
	}
	user.Email = email
	log.Info("user sign-in: ", user.Email)
	return user, nil
}

func (m *UserModel) Authenticate(email, password string) (User, error) {
	var user User
	stmt := "SELECT id, name, roles, hashed_password FROM users WHERE email = ?"
	err := m.DB.QueryRow(stmt, email).Scan(&user.ID, &user.Name, &user.Roles, &user.HashedPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user, ErrInvalidCredentials
		} else {
			return user, err
		}
	}
	err = bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return user, ErrInvalidCredentials
		} else {
			return user, err
		}
	}
	return user, nil
}

func (m *UserModel) Exists(email string) (bool, error) {
	var exists bool
	stmt := "SELECT EXISTS(SELECT true FROM users WHERE email = ?)"
	err := m.DB.QueryRow(stmt, email).Scan(&exists)
	return exists, err
}

func RequireRoleMiddleware(store *session.Store, flash helpers.FlashInterface, role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sess, err := store.Get(c)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		user, ok := sess.Get("user").(User)

		// redirect if user value is not set in session
		if !ok {
			flash.Push(c, "You need to be logged in")
			return c.Redirect("/login")
		}

		// redirect if user roles are not defined in session
		roles := user.Roles
		if roles == "" {
			flash.Push(c, "You need to be logged in")
			return c.Redirect("/login")
		}

		// redirect if user session does not specify the required role
		if !strings.Contains(roles, "|"+role+"|") {
			flash.Push(c, fmt.Sprintf("You need to be logged in as %s", role))
			return c.Redirect("/")
		}
		return c.Next()
	}
}
