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

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	gpc "github.com/yaa110/go-persian-calendar"
	tele "gopkg.in/telebot.v3"
)

type Config struct {
	BotToken string
	AdminIDs []int64
	DBUser   string
	DBPass   string
	DBName   string
}

var db *sql.DB

func loadConfig() Config {
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("❌ خطای پیکربندی: مقدار BOT_TOKEN در فایل .env یافت نشد.")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")
	if dbUser == "" || dbName == "" {
		log.Fatal("❌ خطای پیکربندی: اطلاعات دیتابیس یافت نشد.")
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
		DBUser:   dbUser,
		DBPass:   dbPass,
		DBName:   dbName,
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

func InitDB(cfg Config) {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s?parseTime=true", cfg.DBUser, cfg.DBPass, cfg.DBName)

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("❌ خطا در اتصال به MySQL: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("❌ خطا در برقراری ارتباط با دیتابیس: %v", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)

	query := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY,
		first_name VARCHAR(255),
		username VARCHAR(255),
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`

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
	          ON DUPLICATE KEY UPDATE first_name=?, username=?`
	_, err := db.Exec(query, userID, firstName, username, firstName, username)
	if err != nil {
		log.Printf("⚠️ خطا در ذخیره کاربر: %v", err)
	}
}

func main() {
	cfg := loadConfig()

	InitDB(cfg)
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
	btnAdminPanel := adminMenu.Text("⚙️ مدیریت")

	userMenu.Reply(
		userMenu.Row(btnBuy, btnProfile),
		userMenu.Row(btnWallet),
		userMenu.Row(btnSupport, btnGuide),
	)

	adminMenu.Reply(
		adminMenu.Row(btnBuy, btnProfile),
		adminMenu.Row(btnWallet),
		adminMenu.Row(btnSupport, btnGuide),
		adminMenu.Row(btnAdminPanel),
	)

	profileMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	btnTurnOnSelf := profileMenu.Text("🟢 روشن کردن سلف")
	btnTurnOffSelf := profileMenu.Text("🔴 خاموش کردن سلف")
	btnExitSelf := profileMenu.Text("🛑 خروج سلف")
	btnBack := profileMenu.Text("🔙 بازگشت")

	profileMenu.Reply(
		profileMenu.Row(btnTurnOnSelf, btnTurnOffSelf),
		profileMenu.Row(btnExitSelf),
		profileMenu.Row(btnBack),
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

	bot.Handle(&btnProfile, func(c tele.Context) error {
		user := c.Sender()

		var joinedAt time.Time
		err := db.QueryRow("SELECT joined_at FROM users WHERE id = ?", user.ID).Scan(&joinedAt)
		if err != nil {
			joinedAt = time.Now()
		}

		loc, _ := time.LoadLocation("Asia/Tehran")
		now := time.Now().In(loc)
		joinedAtLocal := joinedAt.In(loc)

		tNow := gpc.New(now)
		todayJalali := tNow.Format("yyyy/MM/dd")
		timeNow := tNow.Format("HH:mm:ss")

		tJoined := gpc.New(joinedAtLocal)
		joinedJalali := tJoined.Format("yyyy/MM/dd")

		daysActive := int(now.Sub(joinedAtLocal).Hours() / 24)
		if daysActive < 1 {
			daysActive = 1
		}

		balance := "0"

		text := fmt.Sprintf(
			"💙 تاریخ امروز: %s\n\n"+
				"⏰ ساعت: %s\n\n"+
				"🔒 اطلاعات حساب کاربری\n\n"+
				"⭐ آیدی عددی: <code>%d</code>\n"+
				"📅 تاریخ عضویت در ربات: %s\n"+
				"👀 فعالیت در ربات: %d روز\n"+
				"💰 موجودی: %s\n"+
				"🔥 وضعیت سلف: ❌ غیرفعال (سلف نخریدی)",
			todayJalali, timeNow, user.ID, joinedJalali, daysActive, balance,
		)

		return c.Send(text, profileMenu, tele.ModeHTML)
	})

	bot.Handle(&btnBack, func(c tele.Context) error {
		return c.Send("🔙 به منوی اصلی بازگشتید.", getKeyboard(c.Sender().ID))
	})

	bot.Handle(&btnTurnOnSelf, func(c tele.Context) error {
		return c.Send("⏳ این بخش به زودی پس از اتصال سرورهای سلف فعال خواهد شد.")
	})

	bot.Handle(&btnTurnOffSelf, func(c tele.Context) error {
		return c.Send("⏳ این بخش به زودی پس از اتصال سرورهای سلف فعال خواهد شد.")
	})

	bot.Handle(&btnExitSelf, func(c tele.Context) error {
		return c.Send("⏳ این بخش به زودی پس از اتصال سرورهای سلف فعال خواهد شد.")
	})

	bot.Handle(&btnBuy, func(c tele.Context) error {
		return c.Send("🛍️ <b>بخش خرید سلف</b>\n\nلطفاً خدمت مورد نظر خود را انتخاب کنید.", tele.ModeHTML)
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

	bot.Handle(&btnAdminPanel, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send("❌ شما دسترسی به بخش مدیریت را ندارید.")
		}
		adminText := "⚙️ <b>پنل مدیریت ربات ولف سلف</b>\n\nوضعیت سیستم: فعال و متصل به MySQL"
		return c.Send(adminText, tele.ModeHTML)
	})

	log.Println("⚡ ربات ولف سلف با دیتابیس MySQL آماده و روشن شد!")
	bot.Start()
}
