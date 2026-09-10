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

// حافظه موقت برای نگهداری مبلغ در حال انتخاب هر کاربر
var userWalletTemp = make(map[int64]int)

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

	queryUsers := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY,
		first_name VARCHAR(255),
		username VARCHAR(255),
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`

	_, err = db.Exec(queryUsers)
	if err != nil {
		log.Fatalf("❌ خطا در ساخت جدول کاربران: %v", err)
	}

	queryWallet := `
	CREATE TABLE IF NOT EXISTS wallets (
		user_id BIGINT PRIMARY KEY,
		balance INT DEFAULT 0,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`

	_, err = db.Exec(queryWallet)
	if err != nil {
		log.Fatalf("❌ خطا در ساخت جدول کیف پول: %v", err)
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
		adminMenu.Row(adminMenu.Text("⚙️ مدیریت")),
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

		balance := GetUserBalance(user.ID)

		text := fmt.Sprintf(
			"💙 تاریخ امروز: %s\n\n"+
				"⏰ ساعت: %s\n\n"+
				"🔒 اطلاعات حساب کاربری\n\n"+
				"⭐ آیدی عددی: <code>%d</code>\n"+
				"📅 تاریخ عضویت در ربات: %s\n"+
				"👀 فعالیت در ربات: %d روز\n"+
				"💰 موجودی: %d تومان\n"+
				"🔥 وضعیت سلف: ❌ غیرفعال (سلف نخریدی)",
			todayJalali, timeNow, user.ID, joinedJalali, daysActive, balance,
		)

		return c.Send(text, profileMenu, tele.ModeHTML)
	})

	bot.Handle(&btnBack, func(c tele.Context) error {
		return c.Send("🔙 به منوی اصلی بازگشتید.", getKeyboard(c.Sender().ID))
	})

	// --- بخش کیف پول پیشرفته با قابلیت جمع و تفریق زنده ---
	getWalletKeyboard := func() *tele.ReplyMarkup {
		menu := &tele.ReplyMarkup{}

		btnP25k := menu.Data("+ 25,000", "wallet_change", "25000")
		btnP50k := menu.Data("+ 50,000", "wallet_change", "50000")
		btnP100k := menu.Data("+ 100,000", "wallet_change", "100000")

		btnM1k := menu.Data("- 1,000", "wallet_change", "-1000")
		btnP1k := menu.Data("+ 1,000", "wallet_change", "1000")

		btnM5k := menu.Data("- 5,000", "wallet_change", "-5000")
		btnP5k := menu.Data("+ 5,000", "wallet_change", "5000")

		btnM10k := menu.Data("- 10,000", "wallet_change", "-10000")
		btnP10k := menu.Data("+ 10,000", "wallet_change", "10000")

		btnConfirm := menu.Data("✅ تایید و ساخت فاکتور", "wallet_confirm")
		btnWalletBack := menu.Data("🔙 بازگشت", "wallet_back")

		menu.Inline(
			menu.Row(btnP25k, btnP50k, btnP100k),
			menu.Row(btnM1k, btnP1k),
			menu.Row(btnM5k, btnP5k),
			menu.Row(btnM10k, btnP10k),
			menu.Row(btnConfirm),
			menu.Row(btnWalletBack),
		)
		return menu
	}

	formatWalletText := func(amount int) string {
		return fmt.Sprintf(
			"👛 <b>شارژ کیف پول (کارت به کارت)</b>\n\n"+
				"🌿 <b>جهت افزایش موجودی با استفاده از دکمه‌های زیر مبلغ مورد نظر را انتخاب کنید:</b> 🫴\n\n"+
				"••• <b>مبلغ مورد نظر جهت افزایش موجودی:</b> <b>~></b> |\n"+
				"✨ <code>%s تومان</code> | ⭐️⭐️⭐️⭐️⭐️",
			formatMoney(amount),
		)
	}

	bot.Handle(&btnWallet, func(c tele.Context) error {
		userID := c.Sender().ID
		userWalletTemp[userID] = 0 // ریست کردن مبلغ هنگام ورود به کیف پول
		return c.Send(formatWalletText(0), getWalletKeyboard(), tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "wallet_change"}, func(c tele.Context) error {
		userID := c.Sender().ID
		val, _ := strconv.Atoi(c.Data())

		// جمع یا کم کردن از مقدار قبلی کاربر
		current := userWalletTemp[userID]
		current += val
		if current < 0 {
			current = 0
		}
		userWalletTemp[userID] = current

		// آپدیت متن پیام با مبلغ جدید
		err := c.Edit(formatWalletText(current), getWalletKeyboard(), tele.ModeHTML)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: fmt.Sprintf("مبلغ فعلی: %s تومان", formatMoney(current))})
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "wallet_confirm"}, func(c tele.Context) error {
		userID := c.Sender().ID
		amount := userWalletTemp[userID]

		if amount <= 0 {
			return c.Respond(&tele.CallbackResponse{Text: "❌ لطفاً ابتدا مبلغی را انتخاب کنید."})
		}

		// محاسبه تعداد کلید (هر کلید ۳,۳۳۳ تومان)
		// کلید = مبلغ / 3333
		keys := float64(amount) / 3333.0

		text := fmt.Sprintf(
			"🧾 <b>فاکتور شارژ کیف پول</b>\n\n"+
				"💰 <b>مبلغ قابل پرداخت:</b> <code>%s تومان</code>\n"+
				"🔑 <b>تعداد کلید دریافتی:</b> <code>%.2f کلید</code>\n"+
				"(نرخ هر کلید: ۳,۳۳۳ تومان)\n\n"+
				"💳 لطفاً مبلغ فوق را به کارت زیر واریز کرده و فیش واریزی را ارسال کنید:\n\n"+
				"<code>6037-9971-XXXX-XXXX</code>\n"+
				"به نام: <b>جواد ولف</b>",
			formatMoney(amount), keys,
		)

		return c.Edit(text, &tele.ReplyMarkup{
			Inline: [][]tele.InlineButton{
				{{"🔙 بازگشت به کیف پول", "wallet_back"}},
			},
		}, tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "wallet_back"}, func(c tele.Context) error {
		userID := c.Sender().ID
		userWalletTemp[userID] = 0
		return c.Edit(formatWalletText(0), getWalletKeyboard(), tele.ModeHTML)
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

func formatMoney(n int) string {
	s := fmt.Sprintf("%d", n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}
