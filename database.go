package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() {
	var err error
	DB, err = sql.Open("sqlite3", "./wolf.db")
	if err != nil {
		log.Fatalf("❌ خطا در اتصال به دیتابیس: %v", err)
	}

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		telegram_id INTEGER UNIQUE,
		username TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = DB.Exec(createTableQuery)
	if err != nil {
		log.Fatalf("❌ خطا در ساخت جدول دیتابیس: %v", err)
	}

	log.Println("✅ دیتابیس با موفقیت راه‌اندازی شد.")
}
