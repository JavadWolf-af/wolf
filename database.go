package main

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// کش‌های حافظه برای جلوگیری از فشار به دیتابیس در تراکنش‌های همزمان بالا (سرعت بیشتر ربات)
var (
	friendsCacheMu sync.RWMutex
	friendsCache   = make(map[int64]map[int64]bool)

	enemiesCacheMu sync.RWMutex
	enemiesCache   = make(map[int64]map[int64]bool)

	autoReactsCacheMu sync.RWMutex
	autoReactsCache   = make(map[int64]map[int64]string)

	fontSettingsMu sync.RWMutex
	fontSettings   = make(map[int64]struct {
		Enabled bool
		Mode    string
	})
)

func InitDB(cfg Config) {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s?parseTime=true&charset=utf8mb4", cfg.DBUser, cfg.DBPass, cfg.DBName)
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ خطا در اتصال به MySQL: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("❌ خطا در برقراری ارتباط با دیتابیس: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY,
		first_name VARCHAR(255),
		username VARCHAR(255),
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		phone VARCHAR(50) DEFAULT 'ثبت نشده',
		is_blocked BOOLEAN DEFAULT FALSE,
		self_status VARCHAR(50) DEFAULT 'خرید نداشته',
		purchases_count INT DEFAULT 0,
		last_billed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		is_clock_enabled BOOLEAN DEFAULT FALSE,
		original_last_name VARCHAR(255) DEFAULT '',
		is_emoji_enabled BOOLEAN DEFAULT FALSE,
		original_first_name VARCHAR(255) DEFAULT '',
		is_timer_media_enabled BOOLEAN DEFAULT FALSE,
		is_bio_enabled BOOLEAN DEFAULT FALSE,
		bio_mode VARCHAR(20) DEFAULT 'random',
		custom_bio VARCHAR(255) DEFAULT '',
		original_bio VARCHAR(255) DEFAULT '',
		is_font_enabled BOOLEAN DEFAULT FALSE,
		font_mode VARCHAR(30) DEFAULT 'bold_italic'
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	columns := []string{
		"last_billed_at DATETIME DEFAULT CURRENT_TIMESTAMP",
		"is_clock_enabled BOOLEAN DEFAULT FALSE",
		"original_last_name VARCHAR(255) DEFAULT ''",
		"is_emoji_enabled BOOLEAN DEFAULT FALSE",
		"original_first_name VARCHAR(255) DEFAULT ''",
		"is_timer_media_enabled BOOLEAN DEFAULT FALSE",
		"is_bio_enabled BOOLEAN DEFAULT FALSE",
		"bio_mode VARCHAR(20) DEFAULT 'random'",
		"custom_bio VARCHAR(255) DEFAULT ''",
		"original_bio VARCHAR(255) DEFAULT ''",
		"is_font_enabled BOOLEAN DEFAULT FALSE",
		"font_mode VARCHAR(30) DEFAULT 'bold_italic'",
	}
	for _, col := range columns {
		_, _ = db.Exec(fmt.Sprintf("ALTER TABLE users ADD COLUMN %s", col))
	}

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_friends (
		owner_id BIGINT,
		friend_id BIGINT,
		friend_name VARCHAR(255) DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, friend_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_enemies (
		owner_id BIGINT,
		enemy_id BIGINT,
		enemy_name VARCHAR(255) DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, enemy_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_auto_reacts (
		owner_id BIGINT,
		target_id BIGINT,
		target_name VARCHAR(255) DEFAULT '',
		emoji VARCHAR(50) DEFAULT '❤️',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, target_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wallets (
		user_id BIGINT PRIMARY KEY,
		balance INT DEFAULT 0,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS settings (
		setting_key VARCHAR(50) PRIMARY KEY,
		setting_value TEXT
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS transactions (
		id INT AUTO_INCREMENT PRIMARY KEY,
		user_id BIGINT,
		amount INT,
		status VARCHAR(50) DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_number', '6037-9971-XXXX-XXXX')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_name', 'جواد ولف')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_bank', 'بانک ملی')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('support_text', '🎧 <b>بخش پشتیبانی</b>\n\nجهت حل مشکلات و پاسخ به سوالات خود، با ما در ارتباط باشید:')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('support_id', '@JavadWolf')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('key_price', '3333')`)

	loadAllFriendsToCache()
	loadAllEnemiesToCache()
	loadAllAutoReactsToCache()
	loadAllFontSettingsToCache()
}

func loadAllAutoReactsToCache() {
	if db == nil { return }
	rows, err := db.Query("SELECT owner_id, target_id, emoji FROM wolf_auto_reacts")
	if err != nil { return }
	defer rows.Close()

	autoReactsCacheMu.Lock()
	autoReactsCache = make(map[int64]map[int64]string)
	for rows.Next() {
		var oID, tID int64
		var emoji string
		if err := rows.Scan(&oID, &tID, &emoji); err == nil {
			if _, exists := autoReactsCache[oID]; !exists {
				autoReactsCache[oID] = make(map[int64]string)
			}
			autoReactsCache[oID][tID] = emoji
		}
	}
	autoReactsCacheMu.Unlock()
}

func getAutoReact(ownerID, targetID int64) (string, bool) {
	autoReactsCacheMu.RLock()
	defer autoReactsCacheMu.RUnlock()
	if userSet, exists := autoReactsCache[ownerID]; exists {
		emoji, found := userSet[targetID]
		return emoji, found
	}
	return "", false
}

func setAutoReactToCache(ownerID, targetID int64, emoji string) {
	autoReactsCacheMu.Lock()
	defer autoReactsCacheMu.Unlock()
	if _, exists := autoReactsCache[ownerID]; !exists {
		autoReactsCache[ownerID] = make(map[int64]string)
	}
	autoReactsCache[ownerID][targetID] = emoji
}

func removeAutoReactFromCache(ownerID, targetID int64) {
	autoReactsCacheMu.Lock()
	defer autoReactsCacheMu.Unlock()
	if userSet, exists := autoReactsCache[ownerID]; exists {
		delete(userSet, targetID)
	}
}

func clearAutoReactsCache(ownerID int64) {
	autoReactsCacheMu.Lock()
	defer autoReactsCacheMu.Unlock()
	delete(autoReactsCache, ownerID)
}

func loadAllFriendsToCache() {
	if db == nil { return }
	rows, err := db.Query("SELECT owner_id, friend_id FROM wolf_friends")
	if err != nil { return }
	defer rows.Close()

	friendsCacheMu.Lock()
	friendsCache = make(map[int64]map[int64]bool)
	for rows.Next() {
		var oID, fID int64
		if err := rows.Scan(&oID, &fID); err == nil {
			if _, exists := friendsCache[oID]; !exists {
				friendsCache[oID] = make(map[int64]bool)
			}
			friendsCache[oID][fID] = true
		}
	}
	friendsCacheMu.Unlock()
}

func isUserFriend(ownerID, targetID int64) bool {
	friendsCacheMu.RLock()
	defer friendsCacheMu.RUnlock()
	if userSet, exists := friendsCache[ownerID]; exists {
		return userSet[targetID]
	}
	return false
}

func addFriendToCache(ownerID, friendID int64) {
	friendsCacheMu.Lock()
	defer friendsCacheMu.Unlock()
	if _, exists := friendsCache[ownerID]; !exists {
		friendsCache[ownerID] = make(map[int64]bool)
	}
	friendsCache[ownerID][friendID] = true
}

func removeFriendFromCache(ownerID, friendID int64) {
	friendsCacheMu.Lock()
	defer friendsCacheMu.Unlock()
	if userSet, exists := friendsCache[ownerID]; exists {
		delete(userSet, friendID)
	}
}

func clearFriendsCache(ownerID int64) {
	friendsCacheMu.Lock()
	defer friendsCacheMu.Unlock()
	delete(friendsCache, ownerID)
}

func loadAllEnemiesToCache() {
	if db == nil { return }
	rows, err := db.Query("SELECT owner_id, enemy_id FROM wolf_enemies")
	if err != nil { return }
	defer rows.Close()

	enemiesCacheMu.Lock()
	enemiesCache = make(map[int64]map[int64]bool)
	for rows.Next() {
		var oID, eID int64
		if err := rows.Scan(&oID, &eID); err == nil {
			if _, exists := enemiesCache[oID]; !exists {
				enemiesCache[oID] = make(map[int64]bool)
			}
			enemiesCache[oID][eID] = true
		}
	}
	enemiesCacheMu.Unlock()
}

func isUserEnemy(ownerID, targetID int64) bool {
	enemiesCacheMu.RLock()
	defer enemiesCacheMu.RUnlock()
	if userSet, exists := enemiesCache[ownerID]; exists {
		return userSet[targetID]
	}
	return false
}

func addEnemyToCache(ownerID, enemyID int64) {
	enemiesCacheMu.Lock()
	defer enemiesCacheMu.Unlock()
	if _, exists := enemiesCache[ownerID]; !exists {
		enemiesCache[ownerID] = make(map[int64]bool)
	}
	enemiesCache[ownerID][enemyID] = true
}

func removeEnemyFromCache(ownerID, enemyID int64) {
	enemiesCacheMu.Lock()
	defer enemiesCacheMu.Unlock()
	if userSet, exists := enemiesCache[ownerID]; exists {
		delete(userSet, enemyID)
	}
}

func clearEnemiesCache(ownerID int64) {
	enemiesCacheMu.Lock()
	defer enemiesCacheMu.Unlock()
	delete(enemiesCache, ownerID)
}

func loadAllFontSettingsToCache() {
	if db == nil { return }
	rows, err := db.Query("SELECT id, is_font_enabled, font_mode FROM users")
	if err != nil { return }
	defer rows.Close()

	fontSettingsMu.Lock()
	fontSettings = make(map[int64]struct {
		Enabled bool
		Mode    string
	})
	for rows.Next() {
		var uid int64
		var en bool
		var mode string
		if err := rows.Scan(&uid, &en, &mode); err == nil {
			if mode == "" {
				mode = "bold_italic"
			}
			fontSettings[uid] = struct {
				Enabled bool
				Mode    string
			}{Enabled: en, Mode: mode}
		}
	}
	fontSettingsMu.Unlock()
}

func updateFontCache(uid int64, en bool, mode string) {
	fontSettingsMu.Lock()
	defer fontSettingsMu.Unlock()
	fontSettings[uid] = struct {
		Enabled bool
		Mode    string
	}{Enabled: en, Mode: mode}
}

func getFontSetting(uid int64) (bool, string) {
	fontSettingsMu.RLock()
	defer fontSettingsMu.RUnlock()
	if s, ok := fontSettings[uid]; ok {
		return s.Enabled, s.Mode
	}
	return false, "bold_italic"
}

func GetSetting(key string) string {
	var val string
	err := db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = ?", key).Scan(&val)
	if err != nil {
		return ""
	}
	return val
}

func SetSetting(key, val string) {
	_, _ = db.Exec("INSERT INTO settings (setting_key, setting_value) VALUES (?, ?) ON DUPLICATE KEY UPDATE setting_value = ?", key, val, val)
}

func SaveUser(userID int64, firstName, username string) {
	if db == nil {
		return
	}
	_, _ = db.Exec(`INSERT INTO users (id, first_name, username) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE first_name=?, username=?`, userID, firstName, username, firstName, username)
	_, _ = db.Exec(`INSERT IGNORE INTO wallets (user_id, balance) VALUES (?, 0)`, userID)
}

func GetUserBalance(userID int64) int {
	var balance int
	err := db.QueryRow("SELECT balance FROM wallets WHERE user_id = ?", userID).Scan(&balance)
	if err != nil {
		return 0
	}
	return balance
}

func IsUserBlocked(userID int64) bool {
	var blocked bool
	err := db.QueryRow("SELECT is_blocked FROM users WHERE id = ?", userID).Scan(&blocked)
	if err != nil {
		return false
	}
	return blocked
}

func GetUserSelfStatus(userID int64) string {
	var status string
	err := db.QueryRow("SELECT self_status FROM users WHERE id = ?", userID).Scan(&status)
	if err != nil {
		return "خرید نداشته"
	}
	return status
}

func SafeAddUserBalance(userID int64, amount int) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("❌ DB Begin Error: %v", err)
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT IGNORE INTO users (id, first_name, username) VALUES (?, 'کاربر', 'ثبت_نشده')`, userID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO wallets (user_id, balance) VALUES (?, ?) ON DUPLICATE KEY UPDATE balance = balance + ?`, userID, amount, amount)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE users SET purchases_count = purchases_count + 1 WHERE id = ?`, userID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
