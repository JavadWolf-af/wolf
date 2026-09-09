package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// راه‌اندازی دیتابیس و ساخت جداول در صورت عدم وجود
func InitDB() {
	var err error
	DB, err = sql.Open("sqlite", "./wolf.db")
	if err != nil {
		log.Fatalf("❌ خطا در اتصال به دیتابیس: %v", err)
	}

	// کوئری‌های ساخت جداول (بدون آسیب به داده‌های قدیمی)
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			user_id INTEGER PRIMARY KEY,
			first_name TEXT,
			username TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS wallets (
			user_id INTEGER PRIMARY KEY,
			balance REAL DEFAULT 0.0,
			FOREIGN KEY(user_id) REFERENCES users(user_id)
		);`,
	}

	for _, q := range queries {
		_, err := DB.Exec(q)
		if err != nil {
			log.Fatalf("❌ خطا در ایجاد جدول دیتابیس: %v", err)
		}
	}

	log.Println("💾 دیتابیس SQLite با موفقیت متصل و همگام‌سازی شد.")
}

// ذخیره یا به‌روزرسانی مشخصات کاربر
func SaveUser(userID int64, firstName, username string) {
	query := `INSERT INTO users (user_id, first_name, username) 
			  VALUES (?, ?, ?) 
			  ON CONFLICT(user_id) DO UPDATE SET 
			  first_name=excluded.first_name, 
			  username=excluded.username;`
	_, err := DB.Exec(query, userID, firstName, username)
	if err != nil {
		log.Printf("⚠️ خطا در ذخیره کاربر: %v", err)
	}
}
