package helpers

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2/log"
	"strings"
)

type ShelfModelInterface interface {
	Set(key string, value string)
	Get(key string) string
}

type ShelfModel struct {
	DB *sql.DB
}

func (s *ShelfModel) Set(key string, value string) {
	SetShelf(s.DB, key, value)
}

func (s *ShelfModel) Get(key string) string {
	return GetShelf(s.DB, key)
}

func SetShelf(db *sql.DB, key string, value string) {
	log.Infof("setting in shelf: %s", key)
	query := `
		INSERT INTO shelf (name, value)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE
			value = VALUES(value)
		`
	result, err := db.Exec(query, key, value)
	if err != nil {
		log.Errorf("failed to insert/update key", key)
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Errorf("failed to check the affected rows: %v", err)
		return
	}
	if rowsAffected == 0 {
		log.Errorf("%v", sql.ErrNoRows)
		return
	}
	log.Infof("updated key-value pair: (%s, %s)", key, value)
}

func GetShelf(db *sql.DB, key string) string {
	log.Infof("retrieving from shelf: %s", key)
	query := "SELECT id, name, value FROM shelf WHERE name = ?"
	rows, err := db.Query(query, key)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id    int
			name  string
			value string
		)
		err = rows.Scan(&id, &name, &value)
		if err != nil {
			return ""
		}
		log.Infof("shelf value: %s", value)
		return value
	}
	return ""
}

func InitShelf(db *sql.DB, appName string) {
	RunMigration(strings.ReplaceAll(`
	-- Select database
USE <appName>;

-- Create table
CREATE TABLE IF NOT EXISTS shelf (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    value VARCHAR(100) NOT NULL
);
	`, "<appName>", appName), db)
}
