package main

import (
	"database/sql"
	"fmt"
	"html"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	tele "gopkg.in/telebot.v3"
)

type Config struct {
	BotToken string
	AdminIDs []int64
}

var db *sql.DB

func loadConfig() Config {
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("❌ خطای پیکربندی: مقدار BOT_TOKEN در فایل .env یافت نشد.")
	}

	adminStr := os.Getenv("ADMIN_ID")
	var adminIDs []int64
	for _, idStr := range strings.Split(adminStr, ",") {
		idStr = strings.TrimSpace(idStr)
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			adminIDs = append(adminIDs, id)
		}
	}

	return Config{
		BotToken: token,
		AdminIDs: adminIDs,
	}
}

func (c *Config) IsAdmin(userID int64) bool {
	for _, id := range c.AdminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func InitDB() {
	var err error
	db, err = sql.Open("sqlite3", "./wolf.db")
	if err != nil {
		log.Fatalf("❌ خطا در اتصال به دیتابیس: %v", err)
	}

	// تنظیم اتصال همزمان برای جلوگیری از قفل شدن SQLite
	db.SetMaxOpenConns(1)

	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		first_name TEXT,
		username TEXT,
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = db.Exec(query)
	if err != nil {
		log.Fatalf("❌ خطا در ساخت جدول دیتابیس: %v", err)
	}
}

func SaveUser(userID int64, firstName, username string) {
	if db == nil {
		return
	}
	query := `INSERT INTO users (id, first_name, username) VALUES (?, ?, ?) 
	          ON CONFLICT(id) DO UPDATE SET first_name=excluded.first_name, username=excluded.username`
	_, err := db.Exec(query, userID, firstName, username)
	if err != nil {
		log.Printf("⚠️ خطا در ذخیره کاربر: %v", err)
	}
}

func main() {
	cfg := loadConfig()

	InitDB()
	defer db.Close()

	pref := tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatalf("❌ خطا در راه‌اندازی ربات: %v", err)
	}

	userMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	adminMenu := &tele.ReplyMarkup{ResizeKeyboard: true}

	btnBuy := userMenu.Text("🛍️ خرید سلف")
	btnProfile := userMenu.Text("👤 حساب کاربری")
	btnWallet := userMenu.Text("👛 کیف پول 💳")
	btnSupport := userMenu.Text("🎧 پشتیبانی")
	btnGuide := userMenu.Text("📚 راهنما")
	btnAgency := userMenu.Text("👑 نمایندگی")
	btnAdminPanel := adminMenu.Text("⚙️ مدیریت")

	userMenu.Reply(
		userMenu.Row(btnBuy, btnProfile),
		userMenu.Row(btnWallet),
		userMenu.Row(btnSupport, btnGuide),
		userMenu.Row(btnAgency),
	)

	adminMenu.Reply(
		adminMenu.Row(btnBuy, btnProfile),
		adminMenu.Row(btnWallet),
		adminMenu.Row(btnSupport, btnGuide),
		adminMenu.Row(btnAgency),
		adminMenu.Row(btnAdminPanel),
	)

	getKeyboard := func(userID int64) *tele.ReplyMarkup {
		if cfg.IsAdmin(userID) {
			return adminMenu
		}
		return userMenu
	}

	bot.Handle("/start", func(c tele.Context) error {
		user := c.Sender()
		firstName := html.EscapeString(user.FirstName)
		if firstName == "" {
			firstName = "کاربر"
		}

		username := "ثبت نشده"
		if user.Username != "" {
			username = "@" + html.EscapeString(user.Username)
		}

		SaveUser(user.ID, user.FirstName, user.Username)

		welcomeTitle := "👑 <b>به ربات ولف سلف 🐺 خوش آمدید!</b>"
		if cfg.IsAdmin(user.ID) {
			welcomeTitle = "👑 <b>به ربات ولف سلف 🐺 خوش آمدید! (دسترسی مدیر)</b>"
		}

		text := fmt.Sprintf(
			"%s\n\n"+
				"💙 <b>یکی از گزینه‌های زیر را انتخاب کنید:</b>\n\n"+
				"👤 <b>نام:</b> %s\n"+
				"🆔 <b>آیدی عددی:</b> <code>%d</code>\n"+
				"🌐 <b>یوزرنیم:</b> %s",
			welcomeTitle, firstName, user.ID, username,
		)

		return c.Send(text, getKeyboard(user.ID), tele.ModeHTML)
	})

	bot.Handle(&btnBuy, func(c tele.Context) error {
		return c.Send("🛍️ <b>بخش خرید سلف</b>\n\nلطفاً خدمت مورد نظر خود را انتخاب کنید.", tele.ModeHTML)
	})

	bot.Handle(&btnProfile, func(c tele.Context) error {
		user := c.Sender()
		username := "ثبت نشده"
		if user.Username != "" {
			username = "@" + html.EscapeString(user.Username)
		}

		text := fmt.Sprintf(
			"👤 <b>اطلاعات حساب کاربری شما</b>\n\n"+
				"🔹 <b>نام:</b> %s\n"+
				"🔹 <b>آیدی عددی:</b> <code>%d</code>\n"+
				"🔹 <b>یوزرنیم:</b> %s",
			html.EscapeString(user.FirstName), user.ID, username,
		)
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnWallet, func(c tele.Context) error {
		return c.Send("👛 <b>بخش کیف پول</b>\n\nاز این بخش می‌توانید موجودی خود را مدیریت یا شارژ کنید.", tele.ModeHTML)
	})

	bot.Handle(&btnSupport, func(c tele.Context) error {
		return c.Send("🎧 <b>پشتیبانی</b>\n\nجهت ارتباط با پشتیبانی، پیام خود را ارسال کنید.", tele.ModeHTML)
	})

	bot.Handle(&btnGuide, func(c tele.Context) error {
		return c.Send("📚 <b>راهنمای استفاده</b>\n\nآموزش‌ها و راهنمای کامل استفاده از ربات.", tele.ModeHTML)
	})

	bot.Handle(&btnAgency, func(c tele.Context) error {
		return c.Send("👑 <b>بخش نمایندگی</b>\n\nاطلاعات و شرایط دریافت نمایندگی.", tele.ModeHTML)
	})

	bot.Handle(&btnAdminPanel, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send("❌ شما دسترسی به بخش مدیریت را ندارید.")
		}

		adminText := "⚙️ <b>پنل مدیریت ربات ولف سلف</b>\n\n" +
			"به بخش مدیریت خوش آمدید. از این بخش می‌توانید ربات را کنترل و نظارت کنید:\n\n" +
			"📊 <b>وضعیت سیستم:</b> فعال و آنلاین\n" +
			"⚡ <b>سرور:</b> پاسخ‌گویی با سرعت زیر چند میلی‌ثانیه"

		return c.Send(adminText, tele.ModeHTML)
	})

	log.Println("⚡ ربات ولف سلف آماده و روشن شد!")
	bot.Start()
}
