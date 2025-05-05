package database

import (
	"database/sql"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(dbPath, schemaPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	if _, err := db.Exec(string(schema)); err != nil {
		return err
	}

	DB = db

	return nil
}

func LogViolation(kid, prompt, violation string) {
	DB.Exec("INSERT INTO violation_attempts(kid_username,prompt,violation) VALUES(?,?,?)",
		kid, prompt, violation)
}

// LogEvent writes a generic audit event and returns any error it encounters.
func LogEvent(eventType, username string) error {
	_, err := DB.Exec(
		"INSERT INTO audit_events(event_type, username) VALUES(?,?)",
		eventType, username,
	)
	return err
}
