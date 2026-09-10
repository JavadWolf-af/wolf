package main

import (
	"database/sql"
	"fmt"
	"html"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
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

// ============================================================
// RECEIPT MESSAGE MANAGEMENT
// ============================================================

type ReceiptMessage struct {
	ChatID    int64
	MessageID int
}

var receiptMessages = make(map[string][]ReceiptMessage)
var receiptStatus = make(map[string]string)

var receiptMutex sync.Mutex
var stateMutex sync.Mutex

// ============================================================
// CONFIG
// ============================================================

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

	if len(adminIDs) == 0 {
		log.Fatal("❌ خطای پیکربندی: هیچ ADMIN_ID معتبری پیدا نشد.")
	}

	return Config{
		BotToken: token,
		AdminIDs: adminIDs,
		DBUser:   dbUser,
		DBPass:   dbPass,
		DBName:   dbName,
	}
}

// ============================================================
// ADMIN CHECK
// ============================================================

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
		"%s:%s@tcp(127.0.0.1:3306)/%s?parseTime=true",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBName,
	)

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
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	_, err = db.Exec(queryUsers)

	if err != nil {
		log.Fatalf("❌ خطا در ساخت جدول کاربران: %v", err)
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
		log.Fatalf("❌ خطا در ساخت جدول کیف پول: %v", err)
	}
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
	first_name=?,
	username=?
	`

	_, err := db.Exec(
		query,
		userID,
		firstName,
		username,
		firstName,
		username,
	)

	if err != nil {
		log.Printf("⚠️ خطا در ذخیره کاربر: %v", err)
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
	_, _ = db.Exec(
		`UPDATE wallets SET balance = balance + ? WHERE user_id = ?`,
		amount,
		userID,
	)
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
// RECEIPT HELPERS
// ============================================================

func saveReceiptMessage(receiptID string, msg *tele.Message) {
	if msg == nil || msg.Chat == nil {
		return
	}

	receiptMutex.Lock()
	defer receiptMutex.Unlock()

	receiptMessages[receiptID] = append(
		receiptMessages[receiptID],
		ReceiptMessage{
			ChatID:    msg.Chat.ID,
			MessageID: msg.ID,
		},
	)
}

func getReceiptMessages(receiptID string) []ReceiptMessage {
	receiptMutex.Lock()
	defer receiptMutex.Unlock()

	items := receiptMessages[receiptID]

	result := make([]ReceiptMessage, len(items))
	copy(result, items)

	return result
}

func setReceiptStatus(receiptID, status string) {
	receiptMutex.Lock()
	defer receiptMutex.Unlock()

	receiptStatus[receiptID] = status
}

func getReceiptStatus(receiptID string) string {
	receiptMutex.Lock()
	defer receiptMutex.Unlock()

	return receiptStatus[receiptID]
}

func deleteReceipt(receiptID string) {
	receiptMutex.Lock()
	defer receiptMutex.Unlock()

	delete(receiptMessages, receiptID)
	delete(receiptStatus, receiptID)
}

// ============================================================
// EDIT ALL RECEIPT MESSAGES
// ============================================================

func editAllReceiptMessages(
	bot *tele.Bot,
	receiptID string,
	finalCaption string,
) {
	messages := getReceiptMessages(receiptID)

	for _, item := range messages {

		msg := &tele.Message{
			ID: item.MessageID,
			Chat: &tele.Chat{
				ID: item.ChatID,
			},
		}

		// تغییر کپشن
		_, err := bot.EditCaption(
			msg,
			finalCaption,
			tele.ModeHTML,
		)

		if err != nil {
			log.Printf(
				"⚠️ خطا در تغییر کپشن فیش ChatID=%d MessageID=%d: %v",
				item.ChatID,
				item.MessageID,
				err,
			)
		}

		// حذف کامل Inline Keyboard
		_, err = bot.EditReplyMarkup(
			msg,
			nil,
		)

		if err != nil {
			log.Printf(
				"⚠️ خطا در حذف دکمه‌های فیش ChatID=%d MessageID=%d: %v",
				item.ChatID,
				item.MessageID,
				err,
			)
		} else {
			log.Printf(
				"✅ دکمه‌های فیش ChatID=%d MessageID=%d حذف شدند.",
				item.ChatID,
				item.MessageID,
			)
		}
	}
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
		log.Fatalf("❌ خطا در راه‌اندازی ربات: %v", err)
	}

	// ============================================================
	// USER MENU
	// ============================================================

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

	getKeyboard := func(userID int64) *tele.ReplyMarkup {
		if cfg.IsAdmin(userID) {
			return adminMenu
		}

		return userMenu
	}

	// ============================================================
	// PROFILE MENU
	// ============================================================

	profileMenu := &tele.ReplyMarkup{
		ResizeKeyboard: true,
	}

	btnTurnOnSelf := profileMenu.Text("🟢 روشن کردن سلف")
	btnTurnOffSelf := profileMenu.Text("🔴 خاموش کردن سلف")
	btnExitSelf := profileMenu.Text("🛑 خروج سلف")
	btnBack := profileMenu.Text("🔙 بازگشت")

	profileMenu.Reply(
		profileMenu.Row(btnTurnOnSelf, btnTurnOffSelf),
		profileMenu.Row(btnExitSelf),
		profileMenu.Row(btnBack),
	)

	// ============================================================
	// START
	// ============================================================

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

		SaveUser(
			user.ID,
			user.FirstName,
			user.Username,
		)

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

	// ============================================================
	// PROFILE
	// ============================================================

	bot.Handle(&btnProfile, func(c tele.Context) error {

		user := c.Sender()

		if IsUserBlocked(user.ID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		var joinedAt time.Time

		err := db.QueryRow(
			"SELECT joined_at FROM users WHERE id = ?",
			user.ID,
		).Scan(&joinedAt)

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

		daysActive := int(
			now.Sub(joinedAtLocal).Hours() / 24,
		)

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

	// ============================================================
	// BACK
	// ============================================================

	bot.Handle(&btnBack, func(c tele.Context) error {
		return c.Send(
			"🔙 به منوی اصلی بازگشتید.",
			getKeyboard(c.Sender().ID),
		)
	})

	// ============================================================
	// WALLET KEYBOARD
	// ============================================================

	getWalletKeyboard := func() *tele.ReplyMarkup {

		menu := &tele.ReplyMarkup{}

		btnP25k := menu.Data(
			"+ 25,000",
			"wallet_change",
			"25000",
		)

		btnP50k := menu.Data(
			"+ 50,000",
			"wallet_change",
			"50000",
		)

		btnP100k := menu.Data(
			"+ 100,000",
			"wallet_change",
			"100000",
		)

		btnM1k := menu.Data(
			"- 1,000",
			"wallet_change",
			"-1000",
		)

		btnP1k := menu.Data(
			"+ 1,000",
			"wallet_change",
			"1000",
		)

		btnM5k := menu.Data(
			"- 5,000",
			"wallet_change",
			"-5000",
		)

		btnP5k := menu.Data(
			"+ 5,000",
			"wallet_change",
			"5000",
		)

		btnM10k := menu.Data(
			"- 10,000",
			"wallet_change",
			"-10000",
		)

		btnP10k := menu.Data(
			"+ 10,000",
			"wallet_change",
			"10000",
		)

		btnConfirm := menu.Data(
			"✅ تایید و ساخت فاکتور",
			"wallet_confirm",
		)

		btnWalletBack := menu.Data(
			"🔙 بازگشت",
			"wallet_back_main",
		)

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

	// ============================================================
	// WALLET
	// ============================================================

	bot.Handle(&btnWallet, func(c tele.Context) error {

		if IsUserBlocked(c.Sender().ID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		userID := c.Sender().ID

		userWalletTemp[userID] = 0

		return c.Send(
			formatWalletText(0),
			getWalletKeyboard(),
			tele.ModeHTML,
		)
	})

	// ============================================================
	// WALLET CHANGE
	// ============================================================

	bot.Handle(&tele.Btn{Unique: "wallet_change"}, func(c tele.Context) error {

		userID := c.Sender().ID

		val, _ := strconv.Atoi(c.Data())

		current := userWalletTemp[userID]

		current += val

		if current < 0 {
			current = 0
		}

		userWalletTemp[userID] = current

		err := c.Edit(
			formatWalletText(current),
			getWalletKeyboard(),
			tele.ModeHTML,
		)

		if err != nil {
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
	})

	// ============================================================
	// CREATE INVOICE
	// ============================================================

	bot.Handle(&tele.Btn{Unique: "wallet_confirm"}, func(c tele.Context) error {

		userID := c.Sender().ID

		amount := userWalletTemp[userID]

		if amount <= 0 {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ لطفاً ابتدا مبلغی را انتخاب کنید.",
				},
			)
		}

		userPendingInvoice[userID] = amount

		keys := float64(amount) / 3333.0

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

		invoiceMenu := &tele.ReplyMarkup{}

		btnInvoiceBack := invoiceMenu.Data(
			"🔙 بازگشت به کیف پول",
			"wallet_back_to_wallet",
		)

		invoiceMenu.Inline(
			invoiceMenu.Row(btnInvoiceBack),
		)

		return c.Edit(
			text,
			invoiceMenu,
			tele.ModeHTML,
		)
	})

	// ============================================================
	// WALLET BACK MAIN
	// ============================================================

	bot.Handle(&tele.Btn{Unique: "wallet_back_main"}, func(c tele.Context) error {

		userID := c.Sender().ID

		userWalletTemp[userID] = 0

		delete(userPendingInvoice, userID)

		_ = c.Delete()

		return c.Send(
			"🔙 به منوی اصلی بازگشتید.",
			getKeyboard(userID),
		)
	})

	// ============================================================
	// WALLET BACK
	// ============================================================

	bot.Handle(&tele.Btn{Unique: "wallet_back_to_wallet"}, func(c tele.Context) error {

		userID := c.Sender().ID

		delete(userPendingInvoice, userID)

		amount := userWalletTemp[userID]

		return c.Edit(
			formatWalletText(amount),
			getWalletKeyboard(),
			tele.ModeHTML,
		)
	})

	// ============================================================
	// RECEIVE RECEIPT
	// ============================================================

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

		loc, _ := time.LoadLocation("Asia/Tehran")

		now := time.Now().In(loc)

		tNow := gpc.New(now)

		dateNowJalali := tNow.Format("yyyy/MM/dd")
		timeNowStr := tNow.Format("HH:mm:ss")

		tJoined := gpc.New(
			dbJoinedAt.In(loc),
		)

		joinedJalali := tJoined.Format("yyyy/MM/dd")

		keysCount := float64(amount) / 3333.0

		usernameStr := "ثبت نشده"

		if user.Username != "" {
			usernameStr = "@" + html.EscapeString(user.Username)
		}

		// ========================================================
		// RECEIPT ID
		// ========================================================

		receiptID := fmt.Sprintf(
			"%d_%d_%d",
			user.ID,
			amount,
			time.Now().UnixNano(),
		)

		// ========================================================
		// ADMIN INLINE KEYBOARD
		// ========================================================

		adminKeyboard := &tele.ReplyMarkup{}

		btnApprove := adminKeyboard.Data(
			"✅ تایید فیش",
			"admin_approve",
			fmt.Sprintf(
				"%d_%d_%s",
				user.ID,
				amount,
				receiptID,
			),
		)

		btnReject := adminKeyboard.Data(
			"❌ رد فیش",
			"admin_reject",
			fmt.Sprintf(
				"%d_%s",
				user.ID,
				receiptID,
			),
		)

		btnBlock := adminKeyboard.Data(
			"🚫 مسدود",
			"admin_block",
			fmt.Sprintf(
				"%d_%s",
				user.ID,
				receiptID,
			),
		)

		btnUnblock := adminKeyboard.Data(
			"🔓 رفع مسدود",
			"admin_unblock",
			fmt.Sprintf(
				"%d_%s",
				user.ID,
				receiptID,
			),
		)

		btnMessage := adminKeyboard.Data(
			"💬 پیام به کاربر",
			"admin_msg",
			fmt.Sprintf(
				"%d",
				user.ID,
			),
		)

		btnManualAdd := adminKeyboard.Data(
			"💰 افزایش موجودی دستی",
			"admin_manual",
			fmt.Sprintf(
				"%d",
				user.ID,
			),
		)

		adminKeyboard.Inline(
			adminKeyboard.Row(btnReject, btnApprove),
			adminKeyboard.Row(btnBlock, btnUnblock),
			adminKeyboard.Row(btnMessage, btnManualAdd),
		)

		caption := fmt.Sprintf(
			"🔔 <b>فیش واریزی جدید!</b>\n\n"+
				"👤 <b>نام:</b> %s\n"+
				"🆔 <b>آیدی عددی:</b> <code>%d</code>\n"+
				"🌐 <b>یوزرنیم:</b> %s\n"+
				"📅 <b>تاریخ عضویت کاربر:</b> %s\n"+
				"⏰ <b>ساعت:</b> %s (تاریخ امروز: %s)\n"+
				"🔥 <b>وضعیت سلف:</b> %s\n"+
				"🔑 <b>تعداد کلید:</b> <code>%.2f کلید</code> (~%s تومان)\n"+
				"🛍️ <b>تعداد خریدها:</b> %d\n"+
				"📞 <b>شماره تماس:</b> %s",
			html.EscapeString(user.FirstName),
			user.ID,
			usernameStr,
			joinedJalali,
			timeNowStr,
			dateNowJalali,
			selfStatus,
			keysCount,
			formatMoney(amount),
			purchasesCount,
			phone,
		)

		setReceiptStatus(
			receiptID,
			"pending",
		)

		// ========================================================
		// SEND RECEIPT TO ALL ADMINS
		// ========================================================

		for _, adminID := range cfg.AdminIDs {

			sentMsg, err := bot.Send(
				&tele.User{ID: adminID},
				c.Message().Photo,
				caption,
				adminKeyboard,
				tele.ModeHTML,
			)

			if err != nil {
				log.Printf(
					"⚠️ خطا در ارسال فیش به ادمین %d: %v",
					adminID,
					err,
				)

				continue
			}

			// ذخیره MessageID و ChatID
			saveReceiptMessage(
				receiptID,
				sentMsg,
			)

			log.Printf(
				"✅ فیش %s برای ادمین %d ارسال شد. MessageID=%d",
				receiptID,
				adminID,
				sentMsg.ID,
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
	})

	// ============================================================
	// APPROVE RECEIPT
	// ============================================================

	bot.Handle(&tele.Btn{Unique: "admin_approve"}, func(c tele.Context) error {

		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ شما دسترسی ندارید.",
				},
			)
		}

		parts := strings.Split(
			c.Data(),
			"_",
		)

		// userID + amount + receiptID
		if len(parts) < 5 {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ خطا در پردازش اطلاعات فیش.",
				},
			)
		}

		targetUserID, err := strconv.ParseInt(
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

		amount, err := strconv.Atoi(parts[1])

		if err != nil || amount <= 0 {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ مبلغ نامعتبر است.",
				},
			)
		}

		receiptID := strings.Join(
			parts[2:],
			"_",
		)

		// ========================================================
		// PREVENT DOUBLE APPROVAL
		// ========================================================

		currentStatus := getReceiptStatus(receiptID)

		if currentStatus != "pending" {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "⚠️ این فیش قبلاً بررسی شده است.",
					ShowAlert: true,
				},
			)
		}

		setReceiptStatus(
			receiptID,
			"approved",
		)

		// ========================================================
		// ADD BALANCE
		// ========================================================

		AddUserBalance(
			targetUserID,
			amount,
		)

		// ========================================================
		// INCREASE PURCHASE COUNT
		// ========================================================

		_, err = db.Exec(
			"UPDATE users SET purchases_count = purchases_count + 1 WHERE id = ?",
			targetUserID,
		)

		if err != nil {
			log.Printf(
				"⚠️ خطا در افزایش تعداد خرید کاربر %d: %v",
				targetUserID,
				err,
			)
		}

		// ========================================================
		// MESSAGE TO USER
		// ========================================================

		_, err = bot.Send(
			&tele.User{ID: targetUserID},
			fmt.Sprintf(
				"🎉 <b>فیش واریزی شما تایید شد!</b>\n\n"+
					"مبلغ <code>%s تومان</code> به کیف پول شما اضافه گردید. 💳",
				formatMoney(amount),
			),
			tele.ModeHTML,
		)

		if err != nil {
			log.Printf(
				"⚠️ خطا در ارسال پیام تایید به کاربر %d: %v",
				targetUserID,
				err,
			)
		}

		// ========================================================
		// FINAL CAPTION
		// ========================================================

		oldCaption := c.Message().Caption

		finalCaption := oldCaption +
			"\n\n✅ <b>فیش تایید شد و موجودی کاربر شارژ گردید.</b>"

		// ========================================================
		// EDIT ALL ADMIN RECEIPTS
		// ========================================================

		editAllReceiptMessages(
			bot,
			receiptID,
			finalCaption,
		)

		// اطمینان از حذف دکمه همین پیام
		_, _ = bot.EditReplyMarkup(
			c.Message(),
			nil,
		)

		log.Printf(
			"✅ فیش %s توسط ادمین %d تایید شد.",
			receiptID,
			c.Sender().ID,
		)

		return c.Respond(
			&tele.CallbackResponse{
				Text: "✅ فیش تایید شد و دکمه‌ها حذف شدند.",
			},
		)
	})

	// ============================================================
	// REJECT RECEIPT
	// ============================================================

	bot.Handle(&tele.Btn{Unique: "admin_reject"}, func(c tele.Context) error {

		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ شما دسترسی ندارید.",
				},
			)
		}

		parts := strings.Split(
			c.Data(),
			"_",
		)

		if len(parts) < 4 {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ خطا در پردازش اطلاعات فیش.",
				},
			)
		}

		targetUserID, err := strconv.ParseInt(
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

		receiptID := strings.Join(
			parts[1:],
			"_",
		)

		// ========================================================
		// PREVENT DOUBLE ACTION
		// ========================================================

		currentStatus := getReceiptStatus(receiptID)

		if currentStatus != "pending" {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "⚠️ این فیش قبلاً بررسی شده است.",
					ShowAlert: true,
				},
			)
		}

		setReceiptStatus(
			receiptID,
			"rejected",
		)

		// ========================================================
		// MESSAGE TO USER
		// ========================================================

		_, err = bot.Send(
			&tele.User{ID: targetUserID},
			"❌ <b>فیش واریزی شما توسط ادمین رد شد.</b>\n\n"+
				"لطفاً در صورت وجود مشکل با پشتیبانی ارتباط برقرار کنید.",
			tele.ModeHTML,
		)

		if err != nil {
			log.Printf(
				"⚠️ خطا در ارسال پیام رد فیش به کاربر %d: %v",
				targetUserID,
				err,
			)
		}

		// ========================================================
		// FINAL CAPTION
		// ========================================================

		oldCaption := c.Message().Caption

		finalCaption := oldCaption +
			"\n\n❌ <b>فیش واریزی رد شد.</b>"

		// ========================================================
		// EDIT ALL ADMIN RECEIPTS
		// ========================================================

		editAllReceiptMessages(
			bot,
			receiptID,
			finalCaption,
		)

		// اطمینان از حذف دکمه همین پیام
		_, _ = bot.EditReplyMarkup(
			c.Message(),
			nil,
		)

		log.Printf(
			"❌ فیش %s توسط ادمین %d رد شد.",
			receiptID,
			c.Sender().ID,
		)

		return c.Respond(
			&tele.CallbackResponse{
				Text: "❌ فیش رد شد و دکمه‌ها حذف شدند.",
			},
		)
	})

	// ============================================================
	// BLOCK USER
	// ============================================================

	bot.Handle(&tele.Btn{Unique: "admin_block"}, func(c tele.Context) error {

		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ شما دسترسی ندارید.",
				},
			)
		}

		parts := strings.Split(
			c.Data(),
			"_",
		)

		if len(parts) < 4 {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ خطا در پردازش اطلاعات.",
				},
			)
		}

		targetUserID, err := strconv.ParseInt(
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

		receiptID := strings.Join(
			parts[1:],
			"_",
		)

		stateMutex.Lock()

		adminStates[c.Sender().ID] = AdminAction{
			Action:   "block_reason",
			TargetID: targetUserID,
		}

		stateMutex.Unlock()

		// حذف دکمه‌ها از همین پیام
		_, _ = bot.EditReplyMarkup(
			c.Message(),
			nil,
		)

		return c.Send(
			fmt.Sprintf(
				"🚫 <b>لطفاً دلیل مسدودی کاربر %d را ارسال کنید:</b>",
				targetUserID,
			),
			tele.ModeHTML,
		)
	})

	// ============================================================
	// UNBLOCK USER
	// ============================================================

	bot.Handle(&tele.Btn{Unique: "admin_unblock"}, func(c tele.Context) error {

		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ شما دسترسی ندارید.",
				},
			)
		}

		parts := strings.Split(
			c.Data(),
			"_",
		)

		if len(parts) < 4 {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ خطا در پردازش اطلاعات.",
				},
			)
		}

		targetUserID, err := strconv.ParseInt(
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

		receiptID := strings.Join(
			parts[1:],
			"_",
		)

		_, _ = db.Exec(
			"UPDATE users SET is_blocked = FALSE WHERE id = ?",
			targetUserID,
		)

		_, _ = bot.Send(
			&tele.User{ID: targetUserID},
			"🔓 <b>حساب کاربری شما رفع مسدودی شد.</b>",
			tele.ModeHTML,
		)

		finalCaption := c.Message().Caption +
			"\n\n🔓 <b>کاربر رفع مسدودی گردید.</b>"

		editAllReceiptMessages(
			bot,
			receiptID,
			finalCaption,
		)

		_, _ = bot.EditReplyMarkup(
			c.Message(),
			nil,
		)

		return c.Respond(
			&tele.CallbackResponse{
				Text: "🔓 کاربر رفع مسدودی شد.",
			},
		)
	})

	// ============================================================
	// MESSAGE USER
	// ============================================================

	bot.Handle(&tele.Btn{Unique: "admin_msg"}, func(c tele.Context) error {

		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ شما دسترسی ندارید.",
				},
			)
		}

		targetUserID, err := strconv.ParseInt(
			c.Data(),
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

		stateMutex.Lock()

		adminStates[c.Sender().ID] = AdminAction{
			Action:   "msg",
			TargetID: targetUserID,
		}

		stateMutex.Unlock()

		return c.Send(
			"💬 <b>لطفاً متن پیام خود برای کاربر را ارسال کنید:</b>",
			tele.ModeHTML,
		)
	})

	// ============================================================
	// MANUAL ADD
	// ============================================================

	bot.Handle(&tele.Btn{Unique: "admin_manual"}, func(c tele.Context) error {

		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(
				&tele.CallbackResponse{
					Text: "❌ شما دسترسی ندارید.",
				},
			)
		}

		targetUserID, err := strconv.ParseInt(
			c.Data(),
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

		stateMutex.Lock()

		adminStates[c.Sender().ID] = AdminAction{
			Action:   "manual_add",
			TargetID: targetUserID,
		}

		stateMutex.Unlock()

		return c.Send(
			"💰 <b>لطفاً مبلغ مورد نظر برای افزایش دستی موجودی را (فقط عدد به تومان) ارسال کنید:</b>",
			tele.ModeHTML,
		)
	})

	// ============================================================
	// ADMIN TEXT STATES
	// ============================================================

	bot.Handle(tele.OnText, func(c tele.Context) error {

		adminID := c.Sender().ID

		if !cfg.IsAdmin(adminID) {
			return nil
		}

		stateMutex.Lock()

		state, exists := adminStates[adminID]

		stateMutex.Unlock()

		if !exists {
			return nil
		}

		text := c.Text()

		switch state.Action {

		case "msg":

			_, err := bot.Send(
				&tele.User{ID: state.TargetID},
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

			stateMutex.Lock()
			delete(
				adminStates,
				adminID,
			)
			stateMutex.Unlock()

		case "manual_add":

			amount, err := strconv.Atoi(
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

			_, _ = bot.Send(
				&tele.User{ID: state.TargetID},
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

			stateMutex.Lock()
			delete(
				adminStates,
				adminID,
			)
			stateMutex.Unlock()

		case "block_reason":

			_, _ = db.Exec(
				"UPDATE users SET is_blocked = TRUE WHERE id = ?",
				state.TargetID,
			)

			_, err := bot.Send(
				&tele.User{ID: state.TargetID},
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

			stateMutex.Lock()
			delete(
				adminStates,
				adminID,
			)
			stateMutex.Unlock()
		}

		return nil
	})

	// ============================================================
	// SELF
	// ============================================================

	bot.Handle(&btnTurnOnSelf, func(c tele.Context) error {

		return c.Send(
			"⏳ این بخش به زودی پس از اتصال سرورهای سلف فعال خواهد شد.",
		)
	})

	bot.Handle(&btnTurnOffSelf, func(c tele.Context) error {

		return c.Send(
			"⏳ این بخش به زودی پس از اتصال سرورهای سلف فعال خواهد شد.",
		)
	})

	bot.Handle(&btnExitSelf, func(c tele.Context) error {

		return c.Send(
			"⏳ این بخش به زودی پس از اتصال سرورهای سلف فعال خواهد شد.",
		)
	})

	// ============================================================
	// BUY
	// ============================================================

	bot.Handle(&btnBuy, func(c tele.Context) error {

		return c.Send(
			"🛍️ <b>بخش خرید سلف</b>\n\n"+
				"لطفاً خدمت مورد نظر خود را انتخاب کنید.",
			tele.ModeHTML,
		)
	})

	// ============================================================
	// SUPPORT
	// ============================================================

	bot.Handle(&btnSupport, func(c tele.Context) error {

		return c.Send(
			"🎧 <b>پشتیبانی</b>\n\n"+
				"جهت ارتباط با پشتیبانی، پیام خود را ارسال کنید.",
			tele.ModeHTML,
		)
	})

	// ============================================================
	// GUIDE
	// ============================================================

	bot.Handle(&btnGuide, func(c tele.Context) error {

		return c.Send(
			"📚 <b>راهنمای استفاده</b>\n\n"+
				"آموزش‌ها و راهنمای کامل استفاده از ربات.",
			tele.ModeHTML,
		)
	})

	// ============================================================
	// ADMIN PANEL
	// ============================================================

	bot.Handle(&btnAdminPanel, func(c tele.Context) error {

		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send(
				"❌ شما دسترسی به بخش مدیریت را ندارید.",
			)
		}

		adminText :=
			"⚙️ <b>پنل مدیریت ربات ولف سلف</b>\n\n" +
				"وضعیت سیستم: فعال و متصل به MySQL"

		return c.Send(
			adminText,
			tele.ModeHTML,
		)
	})

	// ============================================================
	// START BOT
	// ============================================================

	log.Println(
		"⚡ ربات ولف سلف با دیتابیس MySQL آماده و روشن شد!",
	)

	bot.Start()
}

// ============================================================
// FORMAT MONEY
// ============================================================

func formatMoney(n int) string {

	s := fmt.Sprintf(
		"%d",
		n,
	)

	var parts []string

	for len(s) > 3 {

		parts = append(
			[]string{s[len(s)-3:]},
			parts...,
		)

		s = s[:len(s)-3]
	}

	parts = append(
		[]string{s},
		parts...,
	)

	return strings.Join(
		parts,
		",",
	)
}
