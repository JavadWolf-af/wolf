package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v3"
)

type Config struct {
	BotToken string
	AdminIDs []int64
}

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

func main() {
	cfg := loadConfig()

	// راه‌اندازی دیتابیس
	InitDB()

	pref := tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatalf("❌ خطا در راه‌اندازی ربات: %v", err)
	}

	// کیبوردها
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

	// دستور /start
	bot.Handle("/start", func(c tele.Context) error {
		user := c.Sender()
		firstName := user.FirstName
		if firstName == "" {
			firstName = "کاربر"
		}

		username := "ثبت نشده"
		if user.Username != "" {
			username = "@" + user.Username
		}

		// ذخیره کاربر در دیتابیس
		SaveUser(user.ID, firstName, username)

		welcomeTitle := "👑 *به ربات ولف سلف 🐺 خوش آمدید!*"
		if cfg.IsAdmin(user.ID) {
			welcomeTitle = "👑 *به ربات ولف سلف 🐺 خوش آمدید! (دسترسی مدیر)*"
		}

		text := fmt.Sprintf(
			"%s\n\n"+
				"💙 *یکی از گزینه‌های زیر را انتخاب کنید:*\n\n"+
				"👤 *نام:* %s\n"+
				"🆔 *آیدی عددی:* `%d`\n"+
				"🌐 *یوزرنیم:* %s",
			welcomeTitle, firstName, user.ID, username,
		)

		return c.Send(text, getKeyboard(user.ID), tele.ModeMarkdown)
	})

	// هندلرهای دکمه‌ها
	bot.Handle(&btnBuy, func(c tele.Context) error {
		return c.Send("🛍️ *بخش خرید سلف*\n\nلطفاً خدمت مورد نظر خود را انتخاب کنید.", tele.ModeMarkdown)
	})

	bot.Handle(&btnProfile, func(c tele.Context) error {
		user := c.Sender()
		username := "ثبت نشده"
		if user.Username != "" {
			username = "@" + user.Username
		}

		text := fmt.Sprintf(
			"👤 *اطلاعات حساب کاربری شما*\n\n"+
				"🔹 *نام:* %s\n"+
				"🔹 *آیدی عددی:* `%d`\n"+
				"🔹 *یوزرنیم:* %s",
			user.FirstName, user.ID, username,
		)
		return c.Send(text, tele.ModeMarkdown)
	})

	bot.Handle(&btnWallet, func(c tele.Context) error {
		return c.Send("👛 *بخش کیف پول*\n\nاز این بخش می‌توانید موجودی خود را مدیریت یا شارژ کنید.", tele.ModeMarkdown)
	})

	bot.Handle(&btnSupport, func(c tele.Context) error {
		return c.Send("🎧 *پشتیبانی*\n\nجهت ارتباط با پشتیبانی، پیام خود را ارسال کنید.", tele.ModeMarkdown)
	})

	bot.Handle(&btnGuide, func(c tele.Context) error {
		return c.Send("📚 *راهنمای استفاده*\n\nآموزش‌ها و راهنمای کامل استفاده از ربات.", tele.ModeMarkdown)
	})

	bot.Handle(&btnAgency, func(c tele.Context) error {
		return c.Send("👑 *بخش نمایندگی*\n\nاطلاعات و شرایط دریافت نمایندگی.", tele.ModeMarkdown)
	})

	bot.Handle(&btnAdminPanel, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send("❌ شما دسترسی به بخش مدیریت را ندارید.")
		}

		adminText := "⚙️ *پنل مدیریت ربات ولف سلف*\n\n" +
			"به بخش مدیریت خوش آمدید. از این بخش می‌توانید ربات را کنترل و نظارت کنید:\n\n" +
			"📊 *وضعیت سیستم:* فعال و آنلاین\n" +
			"⚡ *سرور:* پاسخ‌گویی با سرعت زیر چند میلی‌ثانیه"

		return c.Send(adminText, tele.ModeMarkdown)
	})

	log.Println("⚡ ربات ولف سلف آماده و روشن شد!")
	bot.Start()
}
