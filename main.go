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

var userWalletTemp = make(map[int64]int)
var userPendingInvoice = make(map[int64]int)

type AdminAction struct {
	Action   string
	TargetID int64
}

var adminStates = make(map[int64]AdminAction)

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

// ============================================================
// DATABASE
// ============================================================
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

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)

	queryUsers := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY,
		first_name VARCHAR(255),
		username VARCHAR(255),
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		phone VARCHAR(50) DEFAULT 'ثبت نشده',
		is_blocked BOOLEAN DEFAULT FALSE,
		self_status VARCHAR(50) DEFAULT 'خرید نداشته',
		purchases_count INT DEFAULT 0
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`
	_, _ = db.Exec(queryUsers)

	queryWallet := `
	CREATE TABLE IF NOT EXISTS wallets (
		user_id BIGINT PRIMARY KEY,
		balance INT DEFAULT 0,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`
	_, _ = db.Exec(queryWallet)

	// جدول تنظیمات بات (مثل شماره کارت)
	querySettings := `
	CREATE TABLE IF NOT EXISTS settings (
		setting_key VARCHAR(50) PRIMARY KEY,
		setting_value TEXT
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`
	_, _ = db.Exec(querySettings)

	// مقادیر پیش‌فرض در صورت خالی بودن دیتابیس
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_number', '6037-9971-XXXX-XXXX')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_name', 'جواد ولف')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_bank', 'بانک ملی')`)
}

// توابع مدیریت تنظیمات
func GetSetting(key string) string {
	var val string
	err := db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = ?", key).Scan(&val)
	if err != nil {
		return ""
	}
	return val
}

func SetSetting(key, val string) {
	_, _ = db.Exec("UPDATE settings SET setting_value = ? WHERE setting_key = ?", val, key)
}

func SaveUser(userID int64, firstName, username string) {
	if db == nil {
		return
	}
	query := `INSERT INTO users (id, first_name, username) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE first_name=?, username=?`
	_, _ = db.Exec(query, userID, firstName, username, firstName, username)
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

func AddUserBalance(userID int64, amount int) {
	_, _ = db.Exec(`UPDATE wallets SET balance = balance + ? WHERE user_id = ?`, amount, userID)
}

func IsUserBlocked(userID int64) bool {
	var blocked bool
	err := db.QueryRow("SELECT is_blocked FROM users WHERE id = ?", userID).Scan(&blocked)
	if err != nil {
		return false
	}
	return blocked
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

// ============================================================
// MAIN
// ============================================================
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

	// =========================
	// KEYBOARDS
	// =========================
	userMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	adminMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	adminPanelMenu := &tele.ReplyMarkup{ResizeKeyboard: true} // کیبورد اختصاصی پنل مدیریت
	profileMenu := &tele.ReplyMarkup{ResizeKeyboard: true}

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

	// دکمه‌های پنل مدیریت (Reply Keyboard)
	btnConfigCard := adminPanelMenu.Text("💳 تنظیم شماره کارت")
	btnBack := adminPanelMenu.Text("🔙 بازگشت") // دکمه بازگشت مشترک

	adminPanelMenu.Reply(
		adminPanelMenu.Row(btnConfigCard),
		adminPanelMenu.Row(btnBack),
	)

	// دکمه‌های حساب کاربری
	btnTurnOnSelf := profileMenu.Text("🟢 روشن کردن سلف")
	btnTurnOffSelf := profileMenu.Text("🔴 خاموش کردن سلف")
	btnExitSelf := profileMenu.Text("🛑 خروج سلف")

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

	// =========================
	// HANDLERS
	// =========================
	bot.Handle("/start", func(c tele.Context) error {
		user := c.Sender()
		if IsUserBlocked(user.ID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

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

		text := fmt.Sprintf("%s\n\n💙 <b>یکی از گزینه‌های زیر را انتخاب کنید:</b>\n\n👤 <b>نام:</b> %s\n🆔 <b>آیدی عددی:</b> <code>%d</code>\n🌐 <b>یوزرنیم:</b> %s", welcomeTitle, firstName, user.ID, username)
		return c.Send(text, getKeyboard(user.ID), tele.ModeHTML)
	})

	bot.Handle(&btnBack, func(c tele.Context) error {
		// اگر ادمین در حال تنظیم کارت بود و پشیمان شد، وضعیتش پاک شود
		delete(adminStates, c.Sender().ID)
		return c.Send("🔙 به منوی اصلی بازگشتید.", getKeyboard(c.Sender().ID))
	})

	bot.Handle(&btnProfile, func(c tele.Context) error {
		user := c.Sender()
		if IsUserBlocked(user.ID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		var joinedAt time.Time
		_ = db.QueryRow("SELECT joined_at FROM users WHERE id = ?", user.ID).Scan(&joinedAt)

		loc, _ := time.LoadLocation("Asia/Tehran")
		now := time.Now().In(loc)
		tNow := gpc.New(now)
		tJoined := gpc.New(joinedAt.In(loc))

		daysActive := int(now.Sub(joinedAt.In(loc)).Hours() / 24)
		if daysActive < 1 {
			daysActive = 1
		}

		text := fmt.Sprintf("💙 تاریخ امروز: %s\n\n⏰ ساعت: %s\n\n🔒 اطلاعات حساب کاربری\n\n⭐ آیدی عددی: <code>%d</code>\n📅 تاریخ عضویت در ربات: %s\n👀 فعالیت در ربات: %d روز\n💰 موجودی: %d تومان\n🔥 وضعیت سلف: ❌ غیرفعال (سلف نخریدی)", tNow.Format("yyyy/MM/dd"), tNow.Format("HH:mm:ss"), user.ID, tJoined.Format("yyyy/MM/dd"), daysActive, GetUserBalance(user.ID))
		return c.Send(text, profileMenu, tele.ModeHTML)
	})

	getWalletKeyboard := func() *tele.ReplyMarkup {
		menu := &tele.ReplyMarkup{}

		btnP25k := menu.Data("➕ 25,000", "wallet_change", "25000")
		btnP50k := menu.Data("➕ 50,000", "wallet_change", "50000")
		btnP100k := menu.Data("➕ 100,000", "wallet_change", "100000")

		btnM1k := menu.Data("➖ 1,000", "wallet_change", "-1000")
		btnP1k := menu.Data("➕ 1,000", "wallet_change", "1000")

		btnM5k := menu.Data("➖ 5,000", "wallet_change", "-5000")
		btnP5k := menu.Data("➕ 5,000", "wallet_change", "5000")

		btnM10k := menu.Data("➖ 10,000", "wallet_change", "-10000")
		btnP10k := menu.Data("➕ 10,000", "wallet_change", "10000")

		btnConfirm := menu.Data("✅ تایید و ساخت فاکتور", "wallet_confirm")
		btnWalletBack := menu.Data("🔙 بازگشت", "wallet_back_main")

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
		return fmt.Sprintf("👛 <b>شارژ کیف پول (کارت به کارت)</b>\n\n🌿 <b>جهت افزایش موجودی با استفاده از دکمه‌های زیر مبلغ مورد نظر را انتخاب کنید:</b> 🫴\n\n••• <b>مبلغ مورد نظر جهت افزایش موجودی:</b> <b>~></b> |\n✨ <code>%s تومان</code> | ⭐️⭐️⭐️⭐️⭐️", formatMoney(amount))
	}

	bot.Handle(&btnWallet, func(c tele.Context) error {
		if IsUserBlocked(c.Sender().ID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}
		userID := c.Sender().ID
		userWalletTemp[userID] = 0
		return c.Send(formatWalletText(0), getWalletKeyboard(), tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "wallet_change"}, func(c tele.Context) error {
		userID := c.Sender().ID
		val, err := strconv.Atoi(c.Data())
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ خطا در پردازش مبلغ."})
		}

		current := userWalletTemp[userID] + val
		if current < 0 {
			current = 0
		}
		userWalletTemp[userID] = current

		_ = c.Edit(formatWalletText(current), getWalletKeyboard(), tele.ModeHTML)
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "wallet_confirm"}, func(c tele.Context) error {
		userID := c.Sender().ID
		amount := userWalletTemp[userID]

		if amount <= 0 {
			return c.Respond(&tele.CallbackResponse{Text: "❌ لطفاً ابتدا مبلغی را انتخاب کنید."})
		}

		userPendingInvoice[userID] = amount
		keys := float64(amount) / 3333.0

		// فراخوانی اطلاعات کارت از دیتابیس
		cNum := GetSetting("card_number")
		cName := GetSetting("card_name")
		cBank := GetSetting("card_bank")

		text := fmt.Sprintf(
			"🧾 <b>فاکتور شارژ کیف پول</b>\n\n💰 <b>مبلغ قابل پرداخت:</b> <code>%s تومان</code>\n🔑 <b>تعداد کلید دریافتی:</b> <code>%.2f کلید</code>\n(نرخ هر کلید: ۳,۳۳۳ تومان)\n\n💳 لطفاً مبلغ فوق را به کارت زیر واریز کرده و سپس <b>تصویر رسید (فیش) واریزی</b> را همینجا برای ربات ارسال کنید:\n\n🏦 <b>%s</b>\n💳 <code>%s</code>\n👤 به نام: <b>%s</b>",
			formatMoney(amount), keys, cBank, cNum, cName,
		)

		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(menu.Data("🔙 بازگشت به کیف پول", "wallet_back_to_wallet")))
		return c.Edit(text, menu, tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "wallet_back_main"}, func(c tele.Context) error {
		userID := c.Sender().ID
		userWalletTemp[userID] = 0
		delete(userPendingInvoice, userID)
		_ = c.Delete()
		return c.Send("🔙 به منوی اصلی بازگشتید.", getKeyboard(userID))
	})

	bot.Handle(&tele.Btn{Unique: "wallet_back_to_wallet"}, func(c tele.Context) error {
		userID := c.Sender().ID
		delete(userPendingInvoice, userID)
		amount := userWalletTemp[userID]
		_ = c.Edit(formatWalletText(amount), getWalletKeyboard(), tele.ModeHTML)
		return c.Respond()
	})

	bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		user := c.Sender()
		if IsUserBlocked(user.ID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		amount, exists := userPendingInvoice[user.ID]
		if !exists || amount <= 0 {
			return c.Send("📸 تصویر شما دریافت شد.")
		}

		var dbJoinedAt time.Time
		var phone, selfStatus string
		var purchasesCount int
		_ = db.QueryRow("SELECT joined_at, phone, self_status, purchases_count FROM users WHERE id = ?", user.ID).Scan(&dbJoinedAt, &phone, &selfStatus, &purchasesCount)

		loc, _ := time.LoadLocation("Asia/Tehran")
		tJoined := gpc.New(dbJoinedAt.In(loc))

		usernameStr := "ثبت نشده"
		if user.Username != "" {
			usernameStr = "@" + html.EscapeString(user.Username)
		}

		captionText := fmt.Sprintf(
			"🔔 <b>درخواست شارژ (کارت به کارت)</b>\n\n👤 %s (%s)\n🆔 <code>%d</code>\n💰 <b>مبلغ:</b> <code>%s تومان</code>\n🔑 <b>تعداد کلید:</b> <code>%.2f کلید</code>\n📅 <b>عضویت:</b> %s\n🔥 <b>وضعیت سلف:</b> %s",
			html.EscapeString(user.FirstName), usernameStr, user.ID, formatMoney(amount), float64(amount)/3333.0, tJoined.Format("yyyy/MM/dd"), html.EscapeString(selfStatus),
		)

		menu := &tele.ReplyMarkup{}
		btnApprove := menu.Data("✅ تایید", "admin_approve", fmt.Sprintf("%d_%d", user.ID, amount))
		btnReject := menu.Data("❌ رد", "admin_reject", strconv.FormatInt(user.ID, 10))
		btnBlock := menu.Data("🚫 مسدود", "admin_block", strconv.FormatInt(user.ID, 10))
		btnUnblock := menu.Data("🔓 رفع مسدود", "admin_unblock", strconv.FormatInt(user.ID, 10))
		btnMessage := menu.Data("💬 پیام", "admin_msg", strconv.FormatInt(user.ID, 10))
		btnManual := menu.Data("💰 شارژ دستی", "admin_manual", strconv.FormatInt(user.ID, 10))
		btnClose := menu.Data("❌ بستن پنل", "admin_close")

		menu.Inline(
			menu.Row(btnApprove, btnReject),
			menu.Row(btnBlock, btnUnblock),
			menu.Row(btnMessage, btnManual),
			menu.Row(btnClose),
		)

		photo := c.Message().Photo
		photo.Caption = captionText

		for _, adminID := range cfg.AdminIDs {
			_, _ = bot.Send(&tele.User{ID: adminID}, photo, menu, tele.ModeHTML)
		}

		delete(userPendingInvoice, user.ID)
		userWalletTemp[user.ID] = 0

		return c.Send("✅ <b>فیش واریزی شما با موفقیت برای ادمین ارسال شد.</b>\n\nپس از بررسی و تایید، موجودی کیف پول شما به‌روزرسانی خواهد شد.", tele.ModeHTML, getKeyboard(user.ID))
	})

	// =========================
	// ADMIN PANEL ACTIONS
	// =========================
	bot.Handle(&btnAdminPanel, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send("❌ شما دسترسی به بخش مدیریت را ندارید.")
		}
		return c.Send("⚙️ <b>به پنل مدیریت ربات خوش آمدید.</b>\nلطفاً یکی از بخش‌های زیر را انتخاب کنید:", adminPanelMenu)
	})

	bot.Handle(&btnConfigCard, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send("❌ شما دسترسی ندارید.")
		}

		cNum := GetSetting("card_number")
		cName := GetSetting("card_name")
		cBank := GetSetting("card_bank")

		text := fmt.Sprintf(
			"💳 <b>اطلاعات فعلی کارت:</b>\n\n🏦 بانک: %s\n💳 شماره: <code>%s</code>\n👤 نام: %s\n\n"+
				"✏️ برای تغییر این اطلاعات، لطفاً <b>شماره کارت</b>، <b>نام دارنده</b> و <b>نام بانک</b> را در <b>۳ خط مجزا</b> ارسال کنید.\n\n"+
				"مثال:\n<code>6037991122334455\nجواد ولف\nبانک ملی</code>",
			cBank, cNum, cName,
		)

		adminStates[c.Sender().ID] = AdminAction{Action: "set_card"}
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "admin_approve"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}
		parts := strings.Split(c.Data(), "_")
		if len(parts) != 2 {
			return c.Respond()
		}
		targetUserID, _ := strconv.ParseInt(parts[0], 10, 64)
		amount, _ := strconv.Atoi(parts[1])

		AddUserBalance(targetUserID, amount)
		_, _ = db.Exec("UPDATE users SET purchases_count = purchases_count + 1 WHERE id = ?", targetUserID)

		_, _ = bot.Send(&tele.User{ID: targetUserID}, fmt.Sprintf("🎉 <b>فیش واریزی شما تایید شد!</b>\n\nمبلغ <code>%s تومان</code> به کیف پول شما اضافه گردید. 💳", formatMoney(amount)), tele.ModeHTML)

		if c.Message() != nil {
			updatedCaption := c.Message().Caption + "\n\n✅ <b>وضعیت: فیش تایید شد و موجودی کاربر شارژ گردید.</b>"
			_, _ = bot.EditCaption(c.Message(), updatedCaption, tele.ModeHTML, &tele.ReplyMarkup{})
			_ = c.Reply(fmt.Sprintf("✅ <b>شارژ با موفقیت انجام شد!</b>\nمبلغ <code>%s تومان</code> به کیف پول کاربر اضافه گردید.", formatMoney(amount)), tele.ModeHTML)
		} else {
			_ = c.Send(fmt.Sprintf("✅ <b>شارژ با موفقیت انجام شد!</b>\nمبلغ <code>%s تومان</code> به کیف پول کاربر اضافه گردید.", formatMoney(amount)), tele.ModeHTML)
		}
		
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "admin_reject"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}
		targetUserID, _ := strconv.ParseInt(c.Data(), 10, 64)
		_, _ = bot.Send(&tele.User{ID: targetUserID}, "❌ <b>فیش واریزی شما توسط ادمین رد شد.</b>\n\nلطفاً در صورت وجود مشکل با پشتیبانی ارتباط برقرار کنید.", tele.ModeHTML)

		if c.Message() != nil {
			updatedCaption := c.Message().Caption + "\n\n❌ <b>وضعیت: فیش واریزی رد شد.</b>"
			_, _ = bot.EditCaption(c.Message(), updatedCaption, tele.ModeHTML, &tele.ReplyMarkup{})
			_ = c.Reply("❌ <b>فیش رد شد و به کاربر اطلاع داده شد.</b>", tele.ModeHTML)
		} else {
			_ = c.Send("❌ <b>فیش رد شد و به کاربر اطلاع داده شد.</b>", tele.ModeHTML)
		}

		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "admin_block"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}
		targetUserID, _ := strconv.ParseInt(c.Data(), 10, 64)
		adminStates[c.Sender().ID] = AdminAction{Action: "block_reason", TargetID: targetUserID}
		
		if c.Message() != nil {
			_ = c.Reply("🚫 <b>لطفاً دلیل مسدودی را ارسال کنید تا به همراه پیام مسدودی به صورت بولد برای کاربر ارسال شود:</b>", tele.ModeHTML)
		} else {
			_ = c.Send("🚫 <b>لطفاً دلیل مسدودی را ارسال کنید تا به همراه پیام مسدودی به صورت بولد برای کاربر ارسال شود:</b>", tele.ModeHTML)
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "admin_unblock"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}
		targetUserID, _ := strconv.ParseInt(c.Data(), 10, 64)
		_, _ = db.Exec("UPDATE users SET is_blocked = FALSE WHERE id = ?", targetUserID)
		_, _ = bot.Send(&tele.User{ID: targetUserID}, "🔓 <b>حساب کاربری شما رفع مسدودی شد.</b>", tele.ModeHTML)

		if c.Message() != nil {
			updatedCaption := c.Message().Caption + "\n\n🔓 <b>وضعیت: کاربر رفع مسدودی گردید.</b>"
			_, _ = bot.EditCaption(c.Message(), updatedCaption, tele.ModeHTML, &tele.ReplyMarkup{})
			_ = c.Reply("🔓 <b>کاربر با موفقیت رفع مسدود شد.</b>", tele.ModeHTML)
		} else {
			_ = c.Send("🔓 <b>کاربر با موفقیت رفع مسدود شد.</b>", tele.ModeHTML)
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "admin_msg"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}
		targetUserID, _ := strconv.ParseInt(c.Data(), 10, 64)
		adminStates[c.Sender().ID] = AdminAction{Action: "msg", TargetID: targetUserID}
		
		if c.Message() != nil {
			_ = c.Reply("💬 <b>لطفاً متن پیام خود برای کاربر را ارسال کنید:</b>", tele.ModeHTML)
		} else {
			_ = c.Send("💬 <b>لطفاً متن پیام خود برای کاربر را ارسال کنید:</b>", tele.ModeHTML)
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "admin_manual"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}
		targetUserID, _ := strconv.ParseInt(c.Data(), 10, 64)
		adminStates[c.Sender().ID] = AdminAction{Action: "manual_add", TargetID: targetUserID}
		
		if c.Message() != nil {
			_ = c.Reply("💰 <b>لطفاً مبلغ مورد نظر برای افزایش دستی موجودی را (فقط عدد به تومان) ارسال کنید:</b>", tele.ModeHTML)
		} else {
			_ = c.Send("💰 <b>لطفاً مبلغ مورد نظر برای افزایش دستی موجودی را (فقط عدد به تومان) ارسال کنید:</b>", tele.ModeHTML)
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "admin_close"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}
		
		if c.Message() != nil {
			updatedCaption := c.Message().Caption + "\n\n❌ <b>وضعیت: پنل دستی بسته شد.</b>"
			_, _ = bot.EditCaption(c.Message(), updatedCaption, tele.ModeHTML, &tele.ReplyMarkup{})
		}
		
		return c.Respond(&tele.CallbackResponse{Text: "✅ پنل بسته شد."})
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		adminID := c.Sender().ID
		if !cfg.IsAdmin(adminID) {
			return nil
		}

		state, exists := adminStates[adminID]
		if !exists {
			return nil
		}
		text := c.Text()

		switch state.Action {
		case "set_card":
			lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
			if len(lines) < 3 {
				_ = c.Send("❌ فرمت وارد شده اشتباه است.\nلطفاً حتماً اطلاعات را در ۳ خط مجزا (شماره کارت، نام دارنده، نام بانک) ارسال کنید.")
				return nil
			}
			
			SetSetting("card_number", strings.TrimSpace(lines[0]))
			SetSetting("card_name", strings.TrimSpace(lines[1]))
			SetSetting("card_bank", strings.TrimSpace(lines[2]))

			_ = c.Send("✅ <b>اطلاعات کارت با موفقیت ثبت و به‌روزرسانی شد!</b>\nدر بخش کیف پول کارت جدید به کاربران نمایش داده خواهد شد.", tele.ModeHTML)
			delete(adminStates, adminID)

		case "msg":
			_, err := bot.Send(&tele.User{ID: state.TargetID}, fmt.Sprintf("💬 <b>پیام از طرف مدیریت:</b>\n\n%s", html.EscapeString(text)), tele.ModeHTML)
			if err == nil {
				_ = c.Send("✅ پیام با موفقیت به کاربر ارسال شد.")
			} else {
				_ = c.Send("❌ خطا در ارسال پیام به کاربر.")
			}
			delete(adminStates, adminID)

		case "manual_add":
			amount, err := strconv.Atoi(strings.TrimSpace(text))
			if err != nil || amount <= 0 {
				_ = c.Send("❌ مبلغ نامعتبر است. لطفاً فقط یک عدد صحیح وارد کنید.")
				return nil
			}
			AddUserBalance(state.TargetID, amount)
			_, _ = bot.Send(&tele.User{ID: state.TargetID}, fmt.Sprintf("💰 <b>موجودی کیف پول شما به صورت دستی شارژ شد:</b>\n\nمبلغ: <code>%s تومان</code>", formatMoney(amount)), tele.ModeHTML)
			_ = c.Send(fmt.Sprintf("✅ مبلغ %s تومان با موفقیت به کیف پول کاربر اضافه شد.", formatMoney(amount)))
			delete(adminStates, adminID)

		case "block_reason":
			_, _ = db.Exec("UPDATE users SET is_blocked = TRUE WHERE id = ?", state.TargetID)
			_, err := bot.Send(&tele.User{ID: state.TargetID}, fmt.Sprintf("❌ <b>حساب کاربری شما مسدود شد.</b>\n\n<b>دلیل مسدودی: %s</b>", html.EscapeString(text)), tele.ModeHTML)
			if err == nil {
				_ = c.Send("✅ کاربر مسدود شد و دلیل به صورت بولد برایش ارسال گردید.")
			} else {
				_ = c.Send("❌ خطا در ارسال پیام به کاربر.")
			}
			delete(adminStates, adminID)
		}
		return nil
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

	log.Println("⚡ ربات ولف سلف با دیتابیس MySQL آماده و روشن شد!")
	bot.Start()
}
