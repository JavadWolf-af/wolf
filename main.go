```go
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
	tele "gopkg.in/tucnak/telebot.v2"
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

// ============================================================
// CONFIG
// ============================================================

func loadConfig() Config {
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("خطای پیکربندی: مقدار BOT_TOKEN در فایل .env یافت نشد.")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")

	if dbUser == "" || dbName == "" {
		log.Fatal("خطای پیکربندی: اطلاعات دیتابیس یافت نشد.")
	}

	adminStr := os.Getenv("ADMIN_ID")
	var adminIDs []int64

	for _, idStr := range strings.Split(adminStr, ",") {
		idStr = strings.TrimSpace(idStr)

		if idStr == "" {
			continue
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err == nil {
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

	dsn := fmt.Sprintf(
		"%s:%s@tcp(127.0.0.1:3306)/%s?parseTime=true&charset=utf8mb4",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBName,
	)

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("خطا در اتصال به MySQL: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("خطا در برقراری ارتباط با دیتابیس: %v", err)
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
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	_, err = db.Exec(queryUsers)
	if err != nil {
		log.Fatalf("خطا در ساخت جدول کاربران: %v", err)
	}

	queryWallet := `
	CREATE TABLE IF NOT EXISTS wallets (
		user_id BIGINT PRIMARY KEY,
		balance INT DEFAULT 0,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	_, err = db.Exec(queryWallet)
	if err != nil {
		log.Fatalf("خطا در ساخت جدول کیف پول: %v", err)
	}

	log.Println("MySQL database connected successfully.")
}

// ============================================================
// USER
// ============================================================

func SaveUser(userID int64, firstName, username string) {
	if db == nil {
		return
	}

	query := `
	INSERT INTO users (id, first_name, username)
	VALUES (?, ?, ?)
	ON DUPLICATE KEY UPDATE
	first_name = VALUES(first_name),
	username = VALUES(username)
	`

	_, err := db.Exec(query, userID, firstName, username)
	if err != nil {
		log.Printf("خطا در ذخیره کاربر: %v", err)
	}

	_, _ = db.Exec(
		`INSERT IGNORE INTO wallets (user_id, balance) VALUES (?, 0)`,
		userID,
	)
}

func GetUserBalance(userID int64) int {
	var balance int

	err := db.QueryRow(
		"SELECT balance FROM wallets WHERE user_id = ?",
		userID,
	).Scan(&balance)

	if err != nil {
		return 0
	}

	return balance
}

func AddUserBalance(userID int64, amount int) {
	_, err := db.Exec(
		"UPDATE wallets SET balance = balance + ? WHERE user_id = ?",
		amount,
		userID,
	)

	if err != nil {
		log.Printf("خطا در افزایش موجودی کاربر %d: %v", userID, err)
	}
}

func IsUserBlocked(userID int64) bool {
	var blocked bool

	err := db.QueryRow(
		"SELECT is_blocked FROM users WHERE id = ?",
		userID,
	).Scan(&blocked)

	if err != nil {
		return false
	}

	return blocked
}

// ============================================================
// MONEY FORMAT
// ============================================================

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
		Token: cfg.BotToken,
		Poller: &tele.LongPoller{
			Timeout: 10 * time.Second,
		},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatalf("خطا در راه‌اندازی ربات: %v", err)
	}

	log.Println("Telegram bot connected.")

	// ========================================================
	// RAW WALLET KEYBOARD
	// ========================================================

	sendRawWalletKeyboard := func(
		c tele.Context,
		text string,
		isEdit bool,
	) error {

		keyboard := map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{
					{
						"text":          "+ 25,000",
						"callback_data": "\fwallet_change|25000",
						"style":         "primary",
					},
					{
						"text":          "+ 50,000",
						"callback_data": "\fwallet_change|50000",
						"style":         "primary",
					},
					{
						"text":          "+ 100,000",
						"callback_data": "\fwallet_change|100000",
						"style":         "primary",
					},
				},
				{
					{
						"text":          "- 1,000",
						"callback_data": "\fwallet_change|-1000",
						"style":         "danger",
					},
					{
						"text":          "+ 1,000",
						"callback_data": "\fwallet_change|1000",
						"style":         "primary",
					},
				},
				{
					{
						"text":          "- 5,000",
						"callback_data": "\fwallet_change|-5000",
						"style":         "danger",
					},
					{
						"text":          "+ 5,000",
						"callback_data": "\fwallet_change|5000",
						"style":         "primary",
					},
				},
				{
					{
						"text":          "- 10,000",
						"callback_data": "\fwallet_change|-10000",
						"style":         "danger",
					},
					{
						"text":          "+ 10,000",
						"callback_data": "\fwallet_change|10000",
						"style":         "primary",
					},
				},
				{
					{
						"text":          "✅ تایید و ساخت فاکتور",
						"callback_data": "\fwallet_confirm|",
						"style":         "success",
					},
				},
				{
					{
						"text":          "🔙 بازگشت",
						"callback_data": "\fwallet_back_main|",
					},
				},
			},
		}

		payload := map[string]interface{}{
			"chat_id":      c.Sender().ID,
			"text":         text,
			"parse_mode":   "HTML",
			"reply_markup": keyboard,
		}

		// ----------------------------------------------------
		// EDIT EXISTING MESSAGE
		// ----------------------------------------------------

		if isEdit && c.Message() != nil {

			payload["chat_id"] = c.Message().Chat.ID
			payload["message_id"] = c.Message().ID

			_, err := bot.Raw(
				"editMessageText",
				payload,
			)

			if err != nil {
				log.Printf(
					"RAW editMessageText wallet error: %v",
					err,
				)
			}

			return err
		}

		// ----------------------------------------------------
		// SEND NEW MESSAGE
		// ----------------------------------------------------

		_, err := bot.Raw(
			"sendMessage",
			payload,
		)

		if err != nil {
			log.Printf(
				"RAW sendMessage wallet error: %v",
				err,
			)
		}

		return err
	}

	// ========================================================
	// RAW INVOICE
	// ========================================================

	sendRawInvoice := func(
		c tele.Context,
		text string,
	) error {

		keyboard := map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{
					{
						"text":          "🔙 بازگشت به کیف پول",
						"callback_data": "\fwallet_back_to_wallet|",
					},
				},
			},
		}

		if c.Message() == nil {
			return fmt.Errorf("callback message is nil")
		}

		payload := map[string]interface{}{
			"chat_id":      c.Message().Chat.ID,
			"message_id":   c.Message().ID,
			"text":         text,
			"parse_mode":   "HTML",
			"reply_markup": keyboard,
		}

		_, err := bot.Raw(
			"editMessageText",
			payload,
		)

		if err != nil {
			log.Printf(
				"RAW editMessageText invoice error: %v",
				err,
			)
		}

		return err
	}

	// ========================================================
	// RAW ADMIN PANEL
	// ========================================================

	sendRawAdminPanel := func(
		adminID int64,
		text string,
		userID int64,
		amount int,
	) {

		keyboard := map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{
					{
						"text": "❌ رد فیش",
						"callback_data": fmt.Sprintf(
							"\fadmin_reject|%d",
							userID,
						),
						"style": "danger",
					},
					{
						"text": "✅ تایید فیش",
						"callback_data": fmt.Sprintf(
							"\fadmin_approve|%d_%d",
							userID,
							amount,
						),
						"style": "success",
					},
				},
				{
					{
						"text": "🚫 مسدود",
						"callback_data": fmt.Sprintf(
							"\fadmin_block|%d",
							userID,
						),
						"style": "danger",
					},
					{
						"text": "🔓 رفع مسدود",
						"callback_data": fmt.Sprintf(
							"\fadmin_unblock|%d",
							userID,
						),
					},
				},
				{
					{
						"text": "💬 پیام به کاربر",
						"callback_data": fmt.Sprintf(
							"\fadmin_msg|%d",
							userID,
						),
						"style": "primary",
					},
					{
						"text": "💰 افزایش موجودی دستی",
						"callback_data": fmt.Sprintf(
							"\fadmin_manual|%d",
							userID,
						),
						"style": "primary",
					},
				},
			},
		}

		payload := map[string]interface{}{
			"chat_id":      adminID,
			"text":         text,
			"parse_mode":   "HTML",
			"reply_markup": keyboard,
		}

		_, err := bot.Raw(
			"sendMessage",
			payload,
		)

		if err != nil {
			log.Printf(
				"RAW admin panel error for admin %d: %v",
				adminID,
				err,
			)
		}
	}

	// ========================================================
	// USER / ADMIN MENUS
	// ========================================================

	userMenu := &tele.ReplyMarkup{
		ResizeKeyboard: true,
	}

	adminMenu := &tele.ReplyMarkup{
		ResizeKeyboard: true,
	}

	btnBuy := userMenu.Text("🛍️ خرید سلف")
	btnProfile := userMenu.Text("👤 حساب کاربری")
	btnWallet := userMenu.Text("👛 کیف پول 💳")
	btnSupport := userMenu.Text("🎧 پشتیبانی")
	btnGuide := userMenu.Text("📚 راهنما")
	btnAdminPanel := adminMenu.Text("⚙️ مدیریت")

	userMenu.Reply(
		userMenu.Row(
			btnBuy,
			btnProfile,
		),
		userMenu.Row(
			btnWallet,
		),
		userMenu.Row(
			btnSupport,
			btnGuide,
		),
	)

	adminMenu.Reply(
		adminMenu.Row(
			btnBuy,
			btnProfile,
		),
		adminMenu.Row(
			btnWallet,
		),
		adminMenu.Row(
			btnSupport,
			btnGuide,
		),
		adminMenu.Row(
			btnAdminPanel,
		),
	)

	// ========================================================
	// PROFILE MENU
	// ========================================================

	profileMenu := &tele.ReplyMarkup{
		ResizeKeyboard: true,
	}

	btnTurnOnSelf := profileMenu.Text("🟢 روشن کردن سلف")
	btnTurnOffSelf := profileMenu.Text("🔴 خاموش کردن سلف")
	btnExitSelf := profileMenu.Text("🛑 خروج سلف")
	btnBack := profileMenu.Text("🔙 بازگشت")

	profileMenu.Reply(
		profileMenu.Row(
			btnTurnOnSelf,
			btnTurnOffSelf,
		),
		profileMenu.Row(
			btnExitSelf,
		),
		profileMenu.Row(
			btnBack,
		),
	)

	getKeyboard := func(userID int64) *tele.ReplyMarkup {
		if cfg.IsAdmin(userID) {
			return adminMenu
		}

		return userMenu
	}

	// ========================================================
	// START
	// ========================================================

	bot.Handle("/start", func(c tele.Context) error {

		user := c.Sender()

		if IsUserBlocked(user.ID) {
			return c.Send(
				"❌ حساب کاربری شما مسدود شده است.",
			)
		}

		firstName := html.EscapeString(
			user.FirstName,
		)

		if firstName == "" {
			firstName = "کاربر"
		}

		username := "ثبت نشده"

		if user.Username != "" {
			username =
				"@" +
					html.EscapeString(
						user.Username,
					)
		}

		SaveUser(
			user.ID,
			user.FirstName,
			user.Username,
		)

		welcomeTitle :=
			"👑 <b>به ربات ولف سلف 🐺 خوش آمدید!</b>"

		if cfg.IsAdmin(user.ID) {
			welcomeTitle =
				"👑 <b>به ربات ولف سلف 🐺 خوش آمدید! (دسترسی مدیر)</b>"
		}

		text := fmt.Sprintf(
			"%s\n\n"+
				"💙 <b>یکی از گزینه‌های زیر را انتخاب کنید:</b>\n\n"+
				"👤 <b>نام:</b> %s\n"+
				"🆔 <b>آیدی عددی:</b> <code>%d</code>\n"+
				"🌐 <b>یوزرنیم:</b> %s",
			welcomeTitle,
			firstName,
			user.ID,
			username,
		)

		return c.Send(
			text,
			getKeyboard(user.ID),
			tele.ModeHTML,
		)
	})

	// ========================================================
	// PROFILE
	// ========================================================

	bot.Handle(&btnProfile, func(c tele.Context) error {

		user := c.Sender()

		if IsUserBlocked(user.ID) {
			return c.Send(
				"❌ حساب کاربری شما مسدود شده است.",
			)
		}

		var joinedAt time.Time

		err := db.QueryRow(
			"SELECT joined_at FROM users WHERE id = ?",
			user.ID,
		).Scan(&joinedAt)

		if err != nil {
			joinedAt = time.Now()
		}

		loc, err := time.LoadLocation(
			"Asia/Tehran",
		)

		if err != nil {
			loc = time.FixedZone(
				"Tehran",
				3*60*60+30*60,
			)
		}

		now := time.Now().In(loc)
		joinedAtLocal := joinedAt.In(loc)

		tNow := gpc.New(now)

		todayJalali :=
			tNow.Format("yyyy/MM/dd")

		timeNow :=
			tNow.Format("HH:mm:ss")

		tJoined :=
			gpc.New(joinedAtLocal)

		joinedJalali :=
			tJoined.Format("yyyy/MM/dd")

		daysActive :=
			int(
				now.Sub(joinedAtLocal).Hours() / 24,
			)

		if daysActive < 1 {
			daysActive = 1
		}

		balance :=
			GetUserBalance(user.ID)

		text := fmt.Sprintf(
			"💙 تاریخ امروز: %s\n\n"+
				"⏰ ساعت: %s\n\n"+
				"🔒 اطلاعات حساب کاربری\n\n"+
				"⭐ آیدی عددی: <code>%d</code>\n"+
				"📅 تاریخ عضویت در ربات: %s\n"+
				"👀 فعالیت در ربات: %d روز\n"+
				"💰 موجودی: %d تومان\n"+
				"🔥 وضعیت سلف: ❌ غیرفعال (سلف نخریدی)",
			todayJalali,
			timeNow,
			user.ID,
			joinedJalali,
			daysActive,
			balance,
		)

		return c.Send(
			text,
			profileMenu,
			tele.ModeHTML,
		)
	})

	// ========================================================
	// BACK
	// ========================================================

	bot.Handle(&btnBack, func(c tele.Context) error {

		return c.Send(
			"🔙 به منوی اصلی بازگشتید.",
			getKeyboard(c.Sender().ID),
		)
	})

	// ========================================================
	// WALLET
	// ========================================================

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

		if IsUserBlocked(c.Sender().ID) {
			return c.Send(
				"❌ حساب کاربری شما مسدود شده است.",
			)
		}

		userID := c.Sender().ID

		userWalletTemp[userID] = 0

		err := sendRawWalletKeyboard(
			c,
			formatWalletText(0),
			false,
		)

		if err != nil {
			log.Printf(
				"Wallet opening error for user %d: %v",
				userID,
				err,
			)

			return c.Send(
				"❌ خطایی در باز کردن کیف پول رخ داد. لطفاً دوباره تلاش کنید.",
			)
		}

		return nil
	})

	// ========================================================
	// WALLET CHANGE
	// ========================================================

	bot.Handle(
		&tele.Btn{Unique: "wallet_change"},
		func(c tele.Context) error {

			userID := c.Sender().ID

			val, err := strconv.Atoi(
				c.Data(),
			)

			if err != nil {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ مبلغ نامعتبر است.",
					},
				)
			}

			current :=
				userWalletTemp[userID]

			current += val

			if current < 0 {
				current = 0
			}

			userWalletTemp[userID] =
				current

			err = sendRawWalletKeyboard(
				c,
				formatWalletText(current),
				true,
			)

			if err != nil {

				log.Printf(
					"Wallet update error for user %d: %v",
					userID,
					err,
				)

				return c.Respond(
					&tele.CallbackResponse{
						Text: fmt.Sprintf(
							"مبلغ فعلی: %s تومان",
							formatMoney(current),
						),
					},
				)
			}

			return c.Respond()
		},
	)

	// ========================================================
	// WALLET CONFIRM
	// ========================================================

	bot.Handle(
		&tele.Btn{Unique: "wallet_confirm"},
		func(c tele.Context) error {

			userID :=
				c.Sender().ID

			amount :=
				userWalletTemp[userID]

			if amount <= 0 {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ لطفاً ابتدا مبلغی را انتخاب کنید.",
					},
				)
			}

			userPendingInvoice[userID] =
				amount

			keys :=
				float64(amount) / 3333.0

			text := fmt.Sprintf(
				"🧾 <b>فاکتور شارژ کیف پول</b>\n\n"+
					"💰 <b>مبلغ قابل پرداخت:</b> <code>%s تومان</code>\n"+
					"🔑 <b>تعداد کلید دریافتی:</b> <code>%.2f کلید</code>\n"+
					"(نرخ هر کلید: ۳,۳۳۳ تومان)\n\n"+
					"💳 لطفاً مبلغ فوق را به کارت زیر واریز کرده و سپس <b>تصویر رسید (فیش) واریزی</b> را همینجا برای ربات ارسال کنید:\n\n"+
					"<code>6037-9971-XXXX-XXXX</code>\n"+
					"به نام: <b>جواد ولف</b>",
				formatMoney(amount),
				keys,
			)

			return sendRawInvoice(
				c,
				text,
			)
		},
	)

	// ========================================================
	// BACK TO MAIN
	// ========================================================

	bot.Handle(
		&tele.Btn{Unique: "wallet_back_main"},
		func(c tele.Context) error {

			userID :=
				c.Sender().ID

			userWalletTemp[userID] = 0

			delete(
				userPendingInvoice,
				userID,
			)

			_ = c.Delete()

			return c.Send(
				"🔙 به منوی اصلی بازگشتید.",
				getKeyboard(userID),
			)
		},
	)

	// ========================================================
	// BACK TO WALLET
	// ========================================================

	bot.Handle(
		&tele.Btn{Unique: "wallet_back_to_wallet"},
		func(c tele.Context) error {

			userID :=
				c.Sender().ID

			delete(
				userPendingInvoice,
				userID,
			)

			amount :=
				userWalletTemp[userID]

			err := sendRawWalletKeyboard(
				c,
				formatWalletText(amount),
				true,
			)

			if err != nil {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ خطا در بازگشت به کیف پول.",
					},
				)
			}

			return c.Respond()
		},
	)

	// ========================================================
	// RECEIVE RECEIPT
	// ========================================================

	bot.Handle(
		tele.OnPhoto,
		func(c tele.Context) error {

			user :=
				c.Sender()

			if IsUserBlocked(user.ID) {
				return c.Send(
					"❌ حساب کاربری شما مسدود شده است.",
				)
			}

			amount, exists :=
				userPendingInvoice[user.ID]

			if !exists || amount <= 0 {
				return c.Send(
					"📸 تصویر شما دریافت شد.",
				)
			}

			var dbJoinedAt time.Time
			var phone string
			var selfStatus string
			var purchasesCount int

			err := db.QueryRow(
				"SELECT joined_at, phone, self_status, purchases_count FROM users WHERE id = ?",
				user.ID,
			).Scan(
				&dbJoinedAt,
				&phone,
				&selfStatus,
				&purchasesCount,
			)

			if err != nil {
				dbJoinedAt = time.Now()
				phone = "ثبت نشده"
				selfStatus = "خرید نداشته"
				purchasesCount = 0
			}

			loc, err := time.LoadLocation(
				"Asia/Tehran",
			)

			if err != nil {
				loc = time.FixedZone(
					"Tehran",
					3*60*60+30*60,
				)
			}

			now :=
				time.Now().In(loc)

			tNow :=
				gpc.New(now)

			tJoined :=
				gpc.New(dbJoinedAt.In(loc))

			keysCount :=
				float64(amount) / 3333.0

			usernameStr :=
				"ثبت نشده"

			if user.Username != "" {
				usernameStr =
					"@" +
						html.EscapeString(
							user.Username,
						)
			}

			tableText := fmt.Sprintf(
				"📋 <b>اطلاعات فیش واریزی و کاربر</b>\n"+
					"━━━━━━━━━━━━━━━━━━━\n"+
					"👤 <b>نام:</b> %s\n"+
					"🆔 <b>آیدی عددی:</b> <code>%d</code>\n"+
					"🌐 <b>یوزرنیم:</b> %s\n"+
					"📅 <b>تاریخ عضویت:</b> %s\n"+
					"⏰ <b>ساعت ثبت فیش:</b> %s (تاریخ: %s)\n"+
					"🔥 <b>وضعیت سلف:</b> %s\n"+
					"🔑 <b>تعداد کلید:</b> <code>%.2f کلید</code> (~%s تومان)\n"+
					"🛍️ <b>تعداد خریدها:</b> %d\n"+
					"📞 <b>شماره تماس:</b> %s\n"+
					"━━━━━━━━━━━━━━━━━━━\n"+
					"📌 <b>وضعیت:</b> در انتظار بررسی...",
				html.EscapeString(user.FirstName),
				user.ID,
				usernameStr,
				tJoined.Format("yyyy/MM/dd"),
				tNow.Format("HH:mm:ss"),
				tNow.Format("yyyy/MM/dd"),
				html.EscapeString(selfStatus),
				keysCount,
				formatMoney(amount),
				purchasesCount,
				html.EscapeString(phone),
			)

			// ------------------------------------------------
			// SEND RECEIPT TO ADMINS
			// ------------------------------------------------

			for _, adminID :=
				range cfg.AdminIDs {

				_, err :=
					bot.Send(
						&tele.User{
							ID: adminID,
						},
						c.Message().Photo,
					)

				if err != nil {
					log.Printf(
						"خطا در ارسال فیش به ادمین %d: %v",
						adminID,
						err,
					)

					continue
				}

				sendRawAdminPanel(
					adminID,
					tableText,
					user.ID,
					amount,
				)
			}

			delete(
				userPendingInvoice,
				user.ID,
			)

			userWalletTemp[user.ID] = 0

			return c.Send(
				"✅ <b>فیش واریزی شما با موفقیت برای ادمین ارسال شد.</b>\n\n"+
					"پس از بررسی و تایید، موجودی کیف پول شما به‌روزرسانی خواهد شد.",
				tele.ModeHTML,
				getKeyboard(user.ID),
			)
		},
	)

	// ========================================================
	// ADMIN APPROVE
	// ========================================================

	bot.Handle(
		&tele.Btn{Unique: "admin_approve"},
		func(c tele.Context) error {

			if !cfg.IsAdmin(
				c.Sender().ID,
			) {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ شما دسترسی ندارید.",
					},
				)
			}

			parts :=
				strings.Split(
					c.Data(),
					"_",
				)

			if len(parts) != 2 {
				return c.Respond()
			}

			targetUserID, err :=
				strconv.ParseInt(
					parts[0],
					10,
					64,
				)

			if err != nil {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ آیدی کاربر نامعتبر است.",
					},
				)
			}

			amount, err :=
				strconv.Atoi(
					parts[1],
				)

			if err != nil || amount <= 0 {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ مبلغ نامعتبر است.",
					},
				)
			}

			AddUserBalance(
				targetUserID,
				amount,
			)

			_, _ = db.Exec(
				"UPDATE users SET purchases_count = purchases_count + 1 WHERE id = ?",
				targetUserID,
			)

			_, _ =
				bot.Send(
					&tele.User{
						ID: targetUserID,
					},
					fmt.Sprintf(
						"🎉 <b>فیش واریزی شما تایید شد!</b>\n\n"+
							"مبلغ <code>%s تومان</code> به کیف پول شما اضافه گردید. 💳",
						formatMoney(amount),
					),
					tele.ModeHTML,
				)

			if c.Message() != nil {

				updatedTable :=
					c.Message().Text +
						"\n\n"+
						"✅ <b>وضعیت: فیش تایید شد و موجودی کاربر شارژ گردید.</b>"

				_ = c.Edit(
					updatedTable,
					tele.ModeHTML,
					&tele.ReplyMarkup{},
				)
			}

			return c.Respond(
				&tele.CallbackResponse{
					Text: "✅ فیش تایید شد.",
				},
			)
		},
	)

	// ========================================================
	// ADMIN REJECT
	// ========================================================

	bot.Handle(
		&tele.Btn{Unique: "admin_reject"},
		func(c tele.Context) error {

			if !cfg.IsAdmin(
				c.Sender().ID,
			) {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ شما دسترسی ندارید.",
					},
				)
			}

			targetUserID, err :=
				strconv.ParseInt(
					c.Data(),
					10,
					64,
				)

			if err != nil {
				return c.Respond()
			}

			_, _ =
				bot.Send(
					&tele.User{
						ID: targetUserID,
					},
					"❌ <b>فیش واریزی شما توسط ادمین رد شد.</b>\n\n"+
						"لطفاً در صورت وجود مشکل با پشتیبانی ارتباط برقرار کنید.",
					tele.ModeHTML,
				)

			if c.Message() != nil {

				updatedTable :=
					c.Message().Text +
						"\n\n"+
						"❌ <b>وضعیت: فیش واریزی رد شد.</b>"

				_ = c.Edit(
					updatedTable,
					tele.ModeHTML,
					&tele.ReplyMarkup{},
				)
			}

			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ فیش رد شد.",
				},
			)
		},
	)

	// ========================================================
	// ADMIN BLOCK
	// ========================================================

	bot.Handle(
		&tele.Btn{Unique: "admin_block"},
		func(c tele.Context) error {

			if !cfg.IsAdmin(
				c.Sender().ID,
			) {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ شما دسترسی ندارید.",
					},
				)
			}

			targetUserID, err :=
				strconv.ParseInt(
					c.Data(),
					10,
					64,
				)

			if err != nil {
				return c.Respond()
			}

			adminStates[c.Sender().ID] =
				AdminAction{
					Action:   "block_reason",
					TargetID: targetUserID,
				}

			return c.Send(
				"🚫 <b>لطفاً دلیل مسدودی را ارسال کنید تا به همراه پیام مسدودی به صورت بولد برای کاربر ارسال شود:</b>",
				tele.ModeHTML,
			)
		},
	)

	// ========================================================
	// ADMIN UNBLOCK
	// ========================================================

	bot.Handle(
		&tele.Btn{Unique: "admin_unblock"},
		func(c tele.Context) error {

			if !cfg.IsAdmin(
				c.Sender().ID,
			) {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ شما دسترسی ندارید.",
					},
				)
			}

			targetUserID, err :=
				strconv.ParseInt(
					c.Data(),
					10,
					64,
				)

			if err != nil {
				return c.Respond()
			}

			_, _ =
				db.Exec(
					"UPDATE users SET is_blocked = FALSE WHERE id = ?",
					targetUserID,
				)

			_, _ =
				bot.Send(
					&tele.User{
						ID: targetUserID,
					},
					"🔓 <b>حساب کاربری شما رفع مسدودی شد.</b>",
					tele.ModeHTML,
				)

			if c.Message() != nil {

				updatedTable :=
					c.Message().Text +
						"\n\n"+
						"🔓 <b>وضعیت: کاربر رفع مسدودی گردید.</b>"

				_ = c.Edit(
					updatedTable,
					tele.ModeHTML,
					&tele.ReplyMarkup{},
				)
			}

			return c.Respond(
				&tele.CallbackResponse{
					Text: "🔓 کاربر رفع مسدود شد.",
				},
			)
		},
	)

	// ========================================================
	// ADMIN MESSAGE
	// ========================================================

	bot.Handle(
		&tele.Btn{Unique: "admin_msg"},
		func(c tele.Context) error {

			if !cfg.IsAdmin(
				c.Sender().ID,
			) {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ شما دسترسی ندارید.",
					},
				)
			}

			targetUserID, err :=
				strconv.ParseInt(
					c.Data(),
					10,
					64,
				)

			if err != nil {
				return c.Respond()
			}

			adminStates[c.Sender().ID] =
				AdminAction{
					Action:   "msg",
					TargetID: targetUserID,
				}

			return c.Send(
				"💬 <b>لطفاً متن پیام خود برای کاربر را ارسال کنید:</b>",
				tele.ModeHTML,
			)
		},
	)

	// ========================================================
	// ADMIN MANUAL BALANCE
	// ========================================================

	bot.Handle(
		&tele.Btn{Unique: "admin_manual"},
		func(c tele.Context) error {

			if !cfg.IsAdmin(
				c.Sender().ID,
			) {
				return c.Respond(
					&tele.CallbackResponse{
						Text: "❌ شما دسترسی ندارید.",
					},
				)
			}

			targetUserID, err :=
				strconv.ParseInt(
					c.Data(),
					10,
					64,
				)

			if err != nil {
				return c.Respond()
			}

			adminStates[c.Sender().ID] =
				AdminAction{
					Action:   "manual_add",
					TargetID: targetUserID,
				}

			return c.Send(
				"💰 <b>لطفاً مبلغ مورد نظر برای افزایش دستی موجودی را (فقط عدد به تومان) ارسال کنید:</b>",
				tele.ModeHTML,
			)
		},
	)

	// ========================================================
	// ADMIN TEXT ACTIONS
	// ========================================================

	bot.Handle(
		tele.OnText,
		func(c tele.Context) error {

			adminID :=
				c.Sender().ID

			if !cfg.IsAdmin(adminID) {
				return nil
			}

			state, exists :=
				adminStates[adminID]

			if !exists {
				return nil
			}

			text :=
				c.Text()

			switch state.Action {

			case "msg":

				_, err :=
					bot.Send(
						&tele.User{
							ID: state.TargetID,
						},
						fmt.Sprintf(
							"💬 <b>پیام از طرف مدیریت:</b>\n\n%s",
							html.EscapeString(text),
						),
						tele.ModeHTML,
					)

				if err == nil {
					_ = c.Send(
						"✅ پیام با موفقیت به کاربر ارسال شد.",
					)
				} else {
					_ = c.Send(
						"❌ خطا در ارسال پیام به کاربر.",
					)
				}

				delete(
					adminStates,
					adminID,
				)

			case "manual_add":

				amount, err :=
					strconv.Atoi(
						strings.TrimSpace(text),
					)

				if err != nil || amount <= 0 {
					_ = c.Send(
						"❌ مبلغ نامعتبر است. لطفاً فقط یک عدد صحیح وارد کنید.",
					)

					return nil
				}

				AddUserBalance(
					state.TargetID,
					amount,
				)

				_, _ =
					bot.Send(
						&tele.User{
							ID: state.TargetID,
						},
						fmt.Sprintf(
							"💰 <b>موجودی کیف پول شما به صورت دستی شارژ شد:</b>\n\n"+
								"مبلغ: <code>%s تومان</code>",
							formatMoney(amount),
						),
						tele.ModeHTML,
					)

				_ = c.Send(
					fmt.Sprintf(
						"✅ مبلغ %s تومان با موفقیت به کیف پول کاربر اضافه شد.",
						formatMoney(amount),
					),
				)

				delete(
					adminStates,
					adminID,
				)

			case "block_reason":

				_, _ =
					db.Exec(
						"UPDATE users SET is_blocked = TRUE WHERE id = ?",
						state.TargetID,
					)

				_, err :=
					bot.Send(
						&tele.User{
							ID: state.TargetID,
						},
						fmt.Sprintf(
							"❌ <b>حساب کاربری شما مسدود شد.</b>\n\n"+
								"<b>دلیل مسدودی: %s</b>",
							html.EscapeString(text),
						),
						tele.ModeHTML,
					)

				if err == nil {
					_ = c.Send(
						"✅ کاربر مسدود شد و دلیل به صورت بولد برایش ارسال گردید.",
					)
				} else {
					_ = c.Send(
						"❌ خطا در ارسال پیام به کاربر.",
					)
				}

				delete(
					adminStates,
					adminID,
				)
			}

			return nil
		},
	)

	// ========================================================
	// SELF
	// ========================================================

	bot.Handle(
		&btnTurnOnSelf,
		func(c tele.Context) error {

			return c.Send(
				"⏳ این بخش به زودی پس از اتصال سرورهای سلف فعال خواهد شد.",
			)
		},
	)

	bot.Handle(
		&btnTurnOffSelf,
		func(c tele.Context) error {

			return c.Send(
				"⏳ این بخش به زودی پس از اتصال سرورهای سلف فعال خواهد شد.",
			)
		},
	)

	bot.Handle(
		&btnExitSelf,
		func(c tele.Context) error {

			return c.Send(
				"⏳ این بخش به زودی پس از اتصال سرورهای سلف فعال خواهد شد.",
			)
		},
	)

	// ========================================================
	// BUY
	// ========================================================

	bot.Handle(
		&btnBuy,
		func(c tele.Context) error {

			return c.Send(
				"🛍️ <b>بخش خرید سلف</b>\n\n"+
					"لطفاً خدمت مورد نظر خود را انتخاب کنید.",
				tele.ModeHTML,
			)
		},
	)

	// ========================================================
	// SUPPORT
	// ========================================================

	bot.Handle(
		&btnSupport,
		func(c tele.Context) error {

			return c.Send(
				"🎧 <b>پشتیبانی</b>\n\n"+
					"جهت ارتباط با پشتیبانی، پیام خود را ارسال کنید.",
				tele.ModeHTML,
			)
		},
	)

	// ========================================================
	// GUIDE
	// ========================================================

	bot.Handle(
		&btnGuide,
		func(c tele.Context) error {

			return c.Send(
				"📚 <b>راهنمای استفاده</b>\n\n"+
					"آموزش‌ها و راهنمای کامل استفاده از ربات.",
				tele.ModeHTML,
			)
		},
	)

	// ========================================================
	// ADMIN PANEL
	// ========================================================

	bot.Handle(
		&btnAdminPanel,
		func(c tele.Context) error {

			if !cfg.IsAdmin(
				c.Sender().ID,
			) {
				return c.Send(
					"❌ شما دسترسی به بخش مدیریت را ندارید.",
				)
			}

			return c.Send(
				"⚙️ <b>پنل مدیریت ربات ولف سلف</b>\n\n"+
					"وضعیت سیستم: فعال و متصل به MySQL",
				tele.ModeHTML,
			)
		},
	)

	// ========================================================
	// START BOT
	// ========================================================

	log.Println(
		"⚡ ربات ولف سلف با دیتابیس MySQL آماده و روشن شد!",
	)

	bot.Start()
}
```
