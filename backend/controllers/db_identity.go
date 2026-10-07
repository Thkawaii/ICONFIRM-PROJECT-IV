package controllers

import (
	"log"

	"gorm.io/gorm"
)

func tableNameOf(db *gorm.DB, model interface{}) string {
	if db == nil || model == nil {
		return ""
	}
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		log.Println("resolve table name:", err)
		return ""
	}
	return stmt.Schema.Table
}

func ResetIdentityIfEmpty(db *gorm.DB, model interface{}) {
	table := tableNameOf(db, model)
	if table == "" {
		return
	}

	var remaining int64
	if err := db.Table(table).Count(&remaining).Error; err != nil {
		log.Println("reset identity: count", table, ":", err)
		return
	}
	if remaining > 0 {
		return
	}

	switch db.Dialector.Name() {
	case "postgres":

		if err := db.Exec(
			`SELECT setval(pg_get_serial_sequence(?, 'id'), 1, false)`, table,
		).Error; err != nil {
			log.Println("reset identity:", table, ":", err)
		}
	case "sqlite", "sqlite3":

		db.Exec(`DELETE FROM sqlite_sequence WHERE name = ?`, table)
	}
}

func SyncIdentityToMax(db *gorm.DB, model interface{}) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return
	}
	table := tableNameOf(db, model)
	if table == "" {
		return
	}
	if err := db.Exec(
		`SELECT setval(pg_get_serial_sequence(?, 'id'),
		               COALESCE((SELECT MAX(id) FROM "`+table+`"), 1),
		               (SELECT MAX(id) FROM "`+table+`") IS NOT NULL)`,
		table,
	).Error; err != nil {
		log.Println("sync identity:", table, ":", err)
	}
}
