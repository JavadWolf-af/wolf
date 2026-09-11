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
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
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

	querySettings := `
	CREATE TABLE IF NOT EXISTS settings (
		setting_key VARCHAR(50) PRIMARY KEY,
		setting_value TEXT
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`
	_, _ = db.Exec(querySettings)

	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_number', '6037-9971-XXXX-XXXX')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_name', 'جواد ولف')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_bank', 'بانک ملی')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('support_text', '🎧 <b>بخش پشتیبانی</b>\n\nجهت حل مشکلات و پاسخ به سوالات خود، با ما در ارتباط باشید:')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('support_id', '@JavadWolf')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('key_price', '3333')`)
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

// ============================================================
// UTILS
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

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func getKeyPrice() int {
	priceStr := GetSetting("key_price")
	price, err := strconv.Atoi(priceStr)
	if err != nil || price <= 0 {
		return 3333
	}
	return price
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
	adminPanelMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	accountConfigMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	supportConfigMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	profileMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	confirmSelfMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	walletReplyMenu := &tele.ReplyMarkup{ResizeKeyboard: true}

	btnBuy := userMenu.Text("🛍️ خرید سلف")
	btnProfile := userMenu.Text("👤 حساب کاربری")
	btnWallet := userMenu.Text("👛 کیف پول 💳")
	btnSupport := userMenu.Text("🎧 پشتیبانی")
	btnGuide := userMenu.Text("📚 راهنما")
	btnAdminPanel := adminMenu.Text("⚙️ مدیریت")
	btnBack := adminPanelMenu.Text("🔙 بازگشت") 

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

	btnConfigAccount := adminPanelMenu.Text("🛠 تنظیم حساب بانکی")
	btnConfigSupport := adminPanelMenu.Text("📞 تنظیم پشتیبانی")
	btnConfigKeyPrice := adminPanelMenu.Text("🔑 تنظیم نرخ کلید")

	adminPanelMenu.Reply(
		adminPanelMenu.Row(btnConfigAccount, btnConfigSupport),
		adminPanelMenu.Row(btnConfigKeyPrice),
		adminPanelMenu.Row(btnBack),
	)

	btnConfigCardNum := accountConfigMenu.Text("💳 شماره کارت")
	btnConfigCardName := accountConfigMenu.Text("👤 نام صاحب حساب")
	btnConfigCardBank := accountConfigMenu.Text("🏦 نام بانک")
	btnBackToAdminAcc := accountConfigMenu.Text("🔙 بازگشت به مدیریت")

	accountConfigMenu.Reply(
		accountConfigMenu.Row(btnConfigCardNum, btnConfigCardName),
		accountConfigMenu.Row(btnConfigCardBank),
		accountConfigMenu.Row(btnBackToAdminAcc),
	)

	btnConfigSupportText := supportConfigMenu.Text("📝 تنظیم متن پشتیبانی")
	btnConfigSupportID := supportConfigMenu.Text("🆔 تنظیم آیدی پشتیبانی")
	btnBackToAdminSup := supportConfigMenu.Text("🔙 بازگشت به مدیریت")

	supportConfigMenu.Reply(
		supportConfigMenu.Row(btnConfigSupportText, btnConfigSupportID),
		supportConfigMenu.Row(btnBackToAdminSup),
	)

	btnTurnOnSelf := profileMenu.Text("🟢 روشن کردن سلف")
	btnTurnOffSelf := profileMenu.Text("🔴 خاموش کردن سلف")
	btnExitSelf := profileMenu.Text("🛑 خروج سلف")

	profileMenu.Reply(
		profileMenu.Row(btnTurnOnSelf, btnTurnOffSelf),
		profileMenu.Row(btnExitSelf),
		profileMenu.Row(btnBack),
	)

	btnConfirmSelfAction := confirmSelfMenu.Text("🟢 تایید و فعالسازی")
	confirmSelfMenu.Reply(
		confirmSelfMenu.Row(btnConfirmSelfAction),
		confirmSelfMenu.Row(btnBack),
	)

	btnWalletConfirm := walletReplyMenu.Text("✅ تایید و ساخت فاکتور")
	walletReplyMenu.Reply(
		walletReplyMenu.Row(btnWalletConfirm),
		walletReplyMenu.Row(btnBack),
	)

	getKeyboard := func(userID int64) *tele.ReplyMarkup {
		if cfg.IsAdmin(userID) {
			return adminMenu
		}
		return userMenu
	}

	// =========================
	// DASHBOARD GENERATOR
	// =========================
	getAdminDashboard := func() string {
		var totalUsers, activeUsers, blockedUsers int
		_ = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
		_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE is_blocked = TRUE").Scan(&blockedUsers)
		_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE self_status = 'روشن'").Scan(&activeUsers)
		inactiveUsers := totalUsers - activeUsers
		adminCount := len(cfg.AdminIDs)

		var cpuUsage float64
		c, err := cpu.Percent(0, false)
		if err == nil && len(c) > 0 {
			cpuUsage = c[0]
		}

		var ramUsed, ramTotal uint64
		var ramPercent float64
		v, err := mem.VirtualMemory()
		if err == nil {
			ramUsed = v.Used
			ramTotal = v.Total
			ramPercent = v.UsedPercent
		}

		var swapUsed, swapTotal uint64
		var swapPercent float64
		s, err := mem.SwapMemory()
		if err == nil {
			swapUsed = s.Used
			swapTotal = s.Total
			swapPercent = s.UsedPercent
		}

		var diskUsed, diskTotal uint64
		var diskPercent float64
		d, err := disk.Usage("/")
		if err == nil {
			diskUsed = d.Used
			diskTotal = d.Total
			diskPercent = d.UsedPercent
		}

		var totalUp, totalDown uint64
		nv, err := net.IOCounters(false)
		if err == nil && len(nv) > 0 {
			totalUp = nv[0].BytesSent
			totalDown = nv[0].BytesRecv
		}

		currentKeyPrice := getKeyPrice()

		return fmt.Sprintf(`👑 <b>مدیریت کل سیستم به دست شماست!</b>

🖥 <b>مشخصات سرور به شرح زیر است:</b>
⚙️ <b>CPU :</b> <code>%.1f%%</code>
🧮 <b>RAM :</b> <code>%s / %s (%.1f%%)</code>
🔄 <b>Swap :</b> <code>%s / %s (%.1f%%)</code>
💾 <b>Storage :</b> <code>%s / %s (%.1f%%)</code>
🌐 <b>Traffic :</b> 🔺 Up: <code>%s</code> | 🔻 Down: <code>%s</code>

👥 <b>مشخصات سلف به شرح زیر است:</b>
🔹 <b>تعداد کل کاربران :</b> <code>%d نفر</code>
🟢 <b>کاربران فعال :</b> <code>%d نفر</code>
🔴 <b>کاربران غیر فعال :</b> <code>%d نفر</code>
🚫 <b>کاربران مسدود شده :</b> <code>%d نفر</code>
👨‍💻 <b>تعداد ادمین :</b> <code>%d نفر</code>

🔑 <b>قیمت فعلی کلید :</b> <code>%s تومان</code>

✨ <i>بخش مورد نظر خود را از منوی زیر انتخاب کنید:</i>`,
			cpuUsage,
			formatBytes(ramUsed), formatBytes(ramTotal), ramPercent,
			formatBytes(swapUsed), formatBytes(swapTotal), swapPercent,
			formatBytes(diskUsed), formatBytes(diskTotal), diskPercent,
			formatBytes(totalUp), formatBytes(totalDown),
			totalUsers, activeUsers, inactiveUsers, blockedUsers, adminCount,
			formatMoney(currentKeyPrice),
		)
	}

	// =========================
	// START & PROFILE
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
		userID := c.Sender().ID
		delete(adminStates, userID)
		delete(userPendingInvoice, userID)
		userWalletTemp[userID] = 0
		return c.Send("🔙 <b>به منوی اصلی بازگشتید.</b>", getKeyboard(userID), tele.ModeHTML)
	})

	backToAdminHandler := func(c tele.Context) error {
		delete(adminStates, c.Sender().ID)
		return c.Send(getAdminDashboard(), adminPanelMenu, tele.ModeHTML)
	}
	bot.Handle(&btnBackToAdminAcc, backToAdminHandler)
	bot.Handle(&btnBackToAdminSup, backToAdminHandler)

	bot.Handle(&btnProfile, func(c tele.Context) error {
		user := c.Sender()
		if IsUserBlocked(user.ID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		var joinedAt time.Time
		var selfStatus string
		_ = db.QueryRow("SELECT joined_at, self_status FROM users WHERE id = ?", user.ID).Scan(&joinedAt, &selfStatus)

		loc, _ := time.LoadLocation("Asia/Tehran")
		now := time.Now().In(loc)
		tNow := gpc.New(now)
		tJoined := gpc.New(joinedAt.In(loc))

		daysActive := int(now.Sub(joinedAt.In(loc)).Hours() / 24)
		if daysActive < 1 {
			daysActive = 1
		}
		
		statusIcon := "❌"
		if selfStatus == "روشن" {
			statusIcon = "✅"
		} else if selfStatus == "خاموش" {
			statusIcon = "⏸️"
		}

		text := fmt.Sprintf("💙 تاریخ امروز: %s\n\n⏰ ساعت: %s\n\n🔒 اطلاعات حساب کاربری\n\n⭐ آیدی عددی: <code>%d</code>\n📅 تاریخ عضویت در ربات: %s\n👀 فعالیت در ربات: %d روز\n💰 موجودی: %d تومان\n🔥 وضعیت سلف: %s %s", tNow.Format("yyyy/MM/dd"), tNow.Format("HH:mm:ss"), user.ID, tJoined.Format("yyyy/MM/dd"), daysActive, GetUserBalance(user.ID), statusIcon, selfStatus)
		return c.Send(text, profileMenu, tele.ModeHTML)
	})

	// =========================
	// WALLET SYSTEM
	// =========================
	getWalletInlineKeyboard := func() *tele.ReplyMarkup {
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

		menu.Inline(
			menu.Row(btnP25k, btnP50k, btnP100k),
			menu.Row(btnM1k, btnP1k),
			menu.Row(btnM5k, btnP5k),
			menu.Row(btnM10k, btnP10k),
		)
		return menu
	}

	formatWalletText := func(amountToAdd int, currentKeys int) string {
		price := getKeyPrice()
		return fmt.Sprintf("👛 <b>شارژ کیف پول (کارت به کارت)</b>\n\n"+
			"🌿 <b>جهت افزایش موجودی با استفاده از دکمه‌های زیر مبلغ مورد نظر را انتخاب کنید:</b>\n\n"+
			"💰 <b>مبلغ مورد نظر جهت افزایش موجودی:</b> <code>%s تومان</code>\n"+
			"🔑 <b>کلیدهای موجود :</b> <code>%d</code>\n\n"+
			"⚠️ <i>حداقل برای فعالسازی سلف شما 30 کلید نیاز دارید</i>\n"+
			"🏷 <i>قیمت هر کلید : %s تومان</i>", formatMoney(amountToAdd), currentKeys, formatMoney(price))
	}

	bot.Handle(&btnWallet, func(c tele.Context) error {
		if IsUserBlocked(c.Sender().ID) { return c.Send("❌ حساب کاربری شما مسدود شده است.") }
		userID := c.Sender().ID
		userWalletTemp[userID] = 0
		
		price := getKeyPrice()
		currentBalance := GetUserBalance(userID)
		currentKeys := currentBalance / price
		
		_ = c.Send("🔰 <b>به بخش شارژ کیف پول خوش آمدید!</b>\nلطفاً مبلغ را از پیام زیر تنظیم کرده و سپس دکمه تایید پایین صفحه را بزنید.", walletReplyMenu, tele.ModeHTML)
		
		return c.Send(formatWalletText(0, currentKeys), getWalletInlineKeyboard(), tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "wallet_change"}, func(c tele.Context) error {
		userID := c.Sender().ID
		val, err := strconv.Atoi(c.Data())
		if err != nil { return c.Respond(&tele.CallbackResponse{Text: "❌ خطا در پردازش مبلغ."}) }

		current := userWalletTemp[userID] + val
		if current < 0 { current = 0 }
		userWalletTemp[userID] = current

		price := getKeyPrice()
		currentBalance := GetUserBalance(userID)
		currentKeys := currentBalance / price

		_ = c.Edit(formatWalletText(current, currentKeys), getWalletInlineKeyboard(), tele.ModeHTML)
		return c.Respond()
	})

	bot.Handle(&btnWalletConfirm, func(c tele.Context) error {
		userID := c.Sender().ID
		amount := userWalletTemp[userID]

		if amount <= 0 {
			return c.Send("❌ <b>لطفاً ابتدا مبلغی را با استفاده از دکمه‌های شیشه‌ای انتخاب کنید.</b>", tele.ModeHTML)
		}

		userPendingInvoice[userID] = amount
		price := getKeyPrice()
		keys := float64(amount) / float64(price)

		cNum := GetSetting("card_number")
		cName := GetSetting("card_name")
		cBank := GetSetting("card_bank")

		text := fmt.Sprintf(
			"🧾 <b>فاکتور شارژ کیف پول</b>\n\n💰 <b>مبلغ قابل پرداخت:</b> <code>%s تومان</code>\n🔑 <b>تعداد کلید دریافتی:</b> <code>%.2f کلید</code>\n(نرخ هر کلید: %s تومان)\n\n💳 لطفاً مبلغ فوق را به کارت زیر واریز کرده و سپس <b>تصویر رسید (فیش) واریزی</b> را همینجا برای ربات ارسال کنید:\n\n🏦 <b>%s</b>\n💳 <code>%s</code>\n👤 به نام: <b>%s</b>",
			formatMoney(amount), keys, formatMoney(price), cBank, cNum, cName,
		)

		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		user := c.Sender()
		if IsUserBlocked(user.ID) { return c.Send("❌ حساب کاربری شما مسدود شده است.") }

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

		price := getKeyPrice()
		captionText := fmt.Sprintf(
			"🔔 <b>درخواست شارژ (کارت به کارت)</b>\n\n👤 %s (%s)\n🆔 <code>%d</code>\n💰 <b>مبلغ:</b> <code>%s تومان</code>\n🔑 <b>تعداد کلید:</b> <code>%.2f کلید</code>\n📅 <b>عضویت:</b> %s\n🔥 <b>وضعیت سلف:</b> %s",
			html.EscapeString(user.FirstName), usernameStr, user.ID, formatMoney(amount), float64(amount)/float64(price), tJoined.Format("yyyy/MM/dd"), html.EscapeString(selfStatus),
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
	// BUY & ACTIVATE SELF
	// =========================
	handleSelfActivation := func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) { return c.Send("❌ حساب کاربری شما مسدود شده است.") }

		price := getKeyPrice()
		balance := GetUserBalance(userID)
		keys := balance / price

		if keys < 30 {
			text := fmt.Sprintf(
				"❌ <b>سلام شما کلید لازم برای شروع ندارید !</b>\n\n" +
				"⏳ <i>سلف روزانه بیلینگ میشه : هر روز یک کلید از حسابت کم میشه !</i>\n\n" +
				"🔑 تعداد کلید های موجود شما <b>%d</b> عدد هست!\n\n" +
				"⚠️ <b>برای فعالسازی حداقل باید 30 کلید داشته باشید ..</b>\n\n" +
				"🛒 <i>لطفا از بخش کیف پول کلید خریداری نمایید.</i>", keys,
			)
			return c.Send(text, tele.ModeHTML)
		}

		text := fmt.Sprintf(
			"🎉 <b>سلام شما کلید لازم برای شروع را دارید !</b>\n\n" +
			"⏳ <i>سلف روزانه بیلینگ میشه : هر روز یک کلید از حسابت کم میشه !</i>\n\n" +
			"🔑 تعداد کلید های موجود شما <b>%d</b> عدد هست!\n\n" +
			"✅ <b>برای فعالسازی سلف و شروع کسر کلید روی دکمه زیر کلیک کنید.</b>", keys,
		)

		return c.Send(text, confirmSelfMenu, tele.ModeHTML)
	}

	bot.Handle(&btnBuy, handleSelfActivation)
	bot.Handle(&btnTurnOnSelf, handleSelfActivation)

	bot.Handle(&btnConfirmSelfAction, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) { return c.Send("❌ حساب کاربری شما مسدود شده است.") }

		price := getKeyPrice()
		balance := GetUserBalance(userID)
		keys := balance / price

		if keys < 30 {
			return c.Send("❌ <b>شما کلید کافی برای فعالسازی ندارید!</b>", getKeyboard(userID), tele.ModeHTML)
		}
		
		_, _ = db.Exec("UPDATE users SET self_status = 'روشن' WHERE id = ?", userID)
		
		return c.Send("✅ <b>سلف شما با موفقیت فعال شد!</b> 🐺\n\nاز این پس روزانه ۱ کلید از حساب شما کسر خواهد شد.", getKeyboard(userID), tele.ModeHTML)
	})

	bot.Handle(&btnTurnOffSelf, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) { return c.Send("❌ حساب کاربری شما مسدود شده است.") }

		_, _ = db.Exec("UPDATE users SET self_status = 'خاموش' WHERE id = ?", userID)
		return c.Send("🔴 <b>سلف شما با موفقیت خاموش شد.</b>\nکسر کلید روزانه متوقف گردید.", tele.ModeHTML)
	})

	bot.Handle(&btnExitSelf, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) { return c.Send("❌ حساب کاربری شما مسدود شده است.") }

		_, _ = db.Exec("UPDATE users SET self_status = 'خروج' WHERE id = ?", userID)
		return c.Send("🛑 <b>شما با موفقیت از سیستم سلف خارج شدید.</b>", tele.ModeHTML)
	})

	// =========================
	// ADMIN PANEL MENUS
	// =========================
	bot.Handle(&btnAdminPanel, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return c.Send("❌ شما دسترسی به بخش مدیریت را ندارید.") }
		return c.Send(getAdminDashboard(), adminPanelMenu, tele.ModeHTML)
	})

	bot.Handle(&btnConfigAccount, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return c.Send("❌ شما دسترسی ندارید.") }
		text := "💳 <b>بخش تنظیمات اطلاعات بانکی</b>\n\nلطفاً برای مشاهده و تغییر اطلاعات، از دکمه‌های زیر استفاده کنید:"
		return c.Send(text, accountConfigMenu, tele.ModeHTML)
	})

	bot.Handle(&btnConfigCardNum, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		current := GetSetting("card_number")
		text := fmt.Sprintf("💳 <b>تنظیم شماره کارت</b>\n\n🔹 مقدار فعلی: <code>%s</code>\n\n✏️ <i>لطفاً شماره کارت جدید را ارسال کنید:</i>", current)
		adminStates[c.Sender().ID] = AdminAction{Action: "set_card_num"}
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnConfigCardName, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		current := GetSetting("card_name")
		text := fmt.Sprintf("👤 <b>تنظیم نام صاحب حساب</b>\n\n🔹 مقدار فعلی: <b>%s</b>\n\n✏️ <i>لطفاً نام جدید دارنده حساب را ارسال کنید:</i>", current)
		adminStates[c.Sender().ID] = AdminAction{Action: "set_card_name"}
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnConfigCardBank, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		current := GetSetting("card_bank")
		text := fmt.Sprintf("🏦 <b>تنظیم نام بانک</b>\n\n🔹 مقدار فعلی: <b>%s</b>\n\n✏️ <i>لطفاً نام بانک جدید را ارسال کنید:</i>", current)
		adminStates[c.Sender().ID] = AdminAction{Action: "set_card_bank"}
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnConfigSupport, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return c.Send("❌ شما دسترسی ندارید.") }
		text := "📞 <b>بخش تنظیمات پشتیبانی</b>\n\nاز طریق منوی زیر می‌توانید متن و آیدی پشتیبانی که به کاربران نمایش داده می‌شود را تغییر دهید:"
		return c.Send(text, supportConfigMenu, tele.ModeHTML)
	})

	bot.Handle(&btnConfigSupportText, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		current := GetSetting("support_text")
		text := fmt.Sprintf("📝 <b>تنظیم متن پشتیبانی</b>\n\n🔹 مقدار فعلی:\n<i>%s</i>\n\n✏️ <i>لطفاً متن جدید پشتیبانی را ارسال کنید:</i>", current)
		adminStates[c.Sender().ID] = AdminAction{Action: "set_support_text"}
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnConfigSupportID, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		current := GetSetting("support_id")
		text := fmt.Sprintf("🆔 <b>تنظیم آیدی پشتیبانی</b>\n\n🔹 مقدار فعلی: <b>%s</b>\n\n✏️ <i>لطفاً آیدی جدید پشتیبانی (مثال: @YourID) را ارسال کنید:</i>", current)
		adminStates[c.Sender().ID] = AdminAction{Action: "set_support_id"}
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnConfigKeyPrice, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		currentPrice := getKeyPrice()
		text := fmt.Sprintf("🔑 <b>تنظیم نرخ کلید</b>\n\n🔹 قیمت فعلی: <code>%s تومان</code>\n\n✏️ <i>لطفاً مبلغ جدید را (فقط عدد به تومان) ارسال کنید:</i>", formatMoney(currentPrice))
		adminStates[c.Sender().ID] = AdminAction{Action: "set_key_price"}
		return c.Send(text, tele.ModeHTML)
	})

	// =========================
	// ADMIN INLINE ACTIONS
	// =========================
	bot.Handle(&tele.Btn{Unique: "admin_approve"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."}) }
		parts := strings.Split(c.Data(), "_")
		if len(parts) != 2 { return c.Respond() }
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
		if !cfg.IsAdmin(c.Sender().ID) { return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."}) }
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
		if !cfg.IsAdmin(c.Sender().ID) { return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."}) }
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
		if !cfg.IsAdmin(c.Sender().ID) { return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."}) }
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
		if !cfg.IsAdmin(c.Sender().ID) { return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."}) }
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
		if !cfg.IsAdmin(c.Sender().ID) { return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."}) }
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
		if !cfg.IsAdmin(c.Sender().ID) { return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."}) }
		
		if c.Message() != nil {
			updatedCaption := c.Message().Caption + "\n\n❌ <b>وضعیت: پنل دستی بسته شد.</b>"
			_, _ = bot.EditCaption(c.Message(), updatedCaption, tele.ModeHTML, &tele.ReplyMarkup{})
		}
		return c.Respond(&tele.CallbackResponse{Text: "✅ پنل بسته شد."})
	})

	// =========================
	// TEXT INPUT HANDLER
	// =========================
	bot.Handle(tele.OnText, func(c tele.Context) error {
		adminID := c.Sender().ID
		if !cfg.IsAdmin(adminID) { return nil }

		state, exists := adminStates[adminID]
		if !exists { return nil }
		text := strings.TrimSpace(c.Text())

		switch state.Action {
		case "set_card_num":
			SetSetting("card_number", text)
			_ = c.Send("✅ <b>شماره کارت با موفقیت به‌روزرسانی شد.</b>", tele.ModeHTML)
			delete(adminStates, adminID)

		case "set_card_name":
			SetSetting("card_name", text)
			_ = c.Send("✅ <b>نام صاحب حساب با موفقیت به‌روزرسانی شد.</b>", tele.ModeHTML)
			delete(adminStates, adminID)

		case "set_card_bank":
			SetSetting("card_bank", text)
			_ = c.Send("✅ <b>نام بانک با موفقیت به‌روزرسانی شد.</b>", tele.ModeHTML)
			delete(adminStates, adminID)

		case "set_support_text":
			SetSetting("support_text", text)
			_ = c.Send("✅ <b>متن پشتیبانی با موفقیت به‌روزرسانی شد.</b>", tele.ModeHTML)
			delete(adminStates, adminID)

		case "set_support_id":
			SetSetting("support_id", text)
			_ = c.Send("✅ <b>آیدی پشتیبانی با موفقیت به‌روزرسانی شد.</b>", tele.ModeHTML)
			delete(adminStates, adminID)

		case "set_key_price":
			price, err := strconv.Atoi(text)
			if err != nil || price <= 0 {
				_ = c.Send("❌ <b>مبلغ نامعتبر است.</b>\nلطفاً فقط یک عدد صحیح (بدون کاما، حرف یا ریال) وارد کنید.", tele.ModeHTML)
				return nil
			}
			SetSetting("key_price", strconv.Itoa(price))
			_ = c.Send(fmt.Sprintf("✅ <b>نرخ کلید با موفقیت به %s تومان تغییر یافت.</b>\n\nاز این پس تمامی محاسبات ربات بر اساس نرخ جدید انجام خواهد شد.", formatMoney(price)), tele.ModeHTML)
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
			amount, err := strconv.Atoi(text)
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

	bot.Handle(&btnSupport, func(c tele.Context) error {
		if IsUserBlocked(c.Sender().ID) { return c.Send("❌ حساب کاربری شما مسدود شده است.") }
		sText := GetSetting("support_text")
		sID := GetSetting("support_id")
		text := fmt.Sprintf("%s\n\n🆔 <b>آیدی ارتباط:</b> %s", sText, sID)
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnGuide, func(c tele.Context) error {
		return c.Send("📚 <b>راهنمای استفاده</b>\n\nآموزش‌ها و راهنمای کامل استفاده از ربات.", tele.ModeHTML)
	})

	log.Println("⚡ ربات ولف سلف با دیتابیس MySQL آماده و روشن شد!")
	bot.Start()
}
