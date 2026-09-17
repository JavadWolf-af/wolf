package main

import (
	"fmt"
	"html"
	"log"
	"strconv"
	"time"

	gpc "github.com/yaa110/go-persian-calendar"
	tele "gopkg.in/telebot.v3"
)

var (
	walletReplyMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	waitingReceiptMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	confirmSelfMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}

	btnWalletConfirm     = walletReplyMenu.Text("✅ تایید و ساخت فاکتور")
	btnCancelReceipt     = waitingReceiptMenu.Text("🔙 لغو و بازگشت به منوی اصلی")
	btnConfirmSelfAction = confirmSelfMenu.Text("🟢 تایید و فعالسازی")
)

func initWalletMarkups() {
	walletReplyMenu.Reply(
		walletReplyMenu.Row(btnWalletConfirm),
		walletReplyMenu.Row(btnBack),
	)

	waitingReceiptMenu.Reply(
		waitingReceiptMenu.Row(btnCancelReceipt),
	)

	confirmSelfMenu.Reply(
		confirmSelfMenu.Row(btnConfirmSelfAction),
		confirmSelfMenu.Row(btnBack),
	)
}

func getWalletInlineKeyboard() *tele.ReplyMarkup {
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

func formatWalletText(amountToAdd int, currentKeys int) string {
	price := getKeyPrice()
	return fmt.Sprintf("👛 <b>شارژ کیف پول (کارت به کارت)</b>\n\n"+
		"🌿 <b>جهت افزایش موجودی با استفاده از دکمه‌های زیر مبلغ مورد نظر را انتخاب کنید:</b>\n\n"+
		"💰 <b>مبلغ مورد نظر جهت افزایش موجودی:</b> <code>%s تومان</code>\n"+
		"🔑 <b>کلیدهای موجود :</b> <code>%d</code>\n\n"+
		"⚠️ <i>حداقل برای فعالسازی سلف شما 30 کلید نیاز دارید</i>\n"+
		"🏷 <i>قیمت هر کلید : %s تومان</i>", formatMoney(amountToAdd), currentKeys, formatMoney(price))
}

func RegisterWalletHandlers(bot *tele.Bot) {
	initWalletMarkups()

	// منوی کیف پول
	bot.Handle(&btnWallet, func(c tele.Context) error {
		if IsUserBlocked(c.Sender().ID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}
		userID := c.Sender().ID

		stateMu.Lock()
		userWalletTemp[userID] = 0
		stateMu.Unlock()

		price := getKeyPrice()
		currentBalance := GetUserBalance(userID)
		currentKeys := currentBalance / price

		_ = c.Send("🔰 <b>به بخش شارژ کیف پول خوش آمدید!</b>\nلطفاً مبلغ را از پیام زیر تنظیم کرده و سپس دکمه تایید پایین صفحه را بزنید.", walletReplyMenu, tele.ModeHTML)
		return c.Send(formatWalletText(0, currentKeys), getWalletInlineKeyboard(), tele.ModeHTML)
	})

	// تغییرات دکمه‌های شیشه‌ای مبلغ
	bot.Handle(&tele.Btn{Unique: "wallet_change"}, func(c tele.Context) error {
		userID := c.Sender().ID
		val, err := strconv.Atoi(c.Data())
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ خطا در پردازش مبلغ."})
		}

		stateMu.Lock()
		current := userWalletTemp[userID] + val
		if current < 0 {
			current = 0
		}
		userWalletTemp[userID] = current
		stateMu.Unlock()

		price := getKeyPrice()
		currentBalance := GetUserBalance(userID)
		currentKeys := currentBalance / price

		_ = c.Edit(formatWalletText(current, currentKeys), getWalletInlineKeyboard(), tele.ModeHTML)
		return c.Respond()
	})

	// صدور فاکتور
	bot.Handle(&btnWalletConfirm, func(c tele.Context) error {
		userID := c.Sender().ID

		stateMu.RLock()
		amount := userWalletTemp[userID]
		stateMu.RUnlock()

		if amount <= 0 {
			return c.Send("❌ <b>لطفاً ابتدا مبلغی را با استفاده از دکمه‌های شیشه‌ای انتخاب کنید.</b>", tele.ModeHTML)
		}

		price := getKeyPrice()
		keys := float64(amount) / float64(price)

		cNum := GetSetting("card_number")
		cName := GetSetting("card_name")
		cBank := GetSetting("card_bank")

		text := fmt.Sprintf(
			"🧾 <b>فاکتور شارژ کیف پول صادر شد</b>\n\n"+
				"💰 <b>مبلغ قابل پرداخت:</b> <code>%s تومان</code>\n"+
				"🔑 <b>تعداد کلید دریافتی:</b> <code>%.2f کلید</code>\n"+
				"🏷 (نرخ هر کلید: %s تومان)\n\n"+
				"💳 لطفاً مبلغ فوق را به کارت زیر واریز نمایید:\n\n"+
				"🏦 <b>%s</b>\n"+
				"💳 <code>%s</code>\n"+
				"👤 به نام: <b>%s</b>\n\n"+
				"📸 <b>سپس تصویر رسید (فیش) واریزی را همینجا ارسال نمایید:</b>",
			formatMoney(amount), keys, formatMoney(price), cBank, cNum, cName,
		)

		return c.Send(text, waitingReceiptMenu, tele.ModeHTML)
	})

	// لغو پرداخت
	bot.Handle(&btnCancelReceipt, func(c tele.Context) error {
		userID := c.Sender().ID
		stateMu.Lock()
		userWalletTemp[userID] = 0
		stateMu.Unlock()
		return c.Send("❌ <b>فرآیند پرداخت لغو گردید.</b>", getMainKeyboard(userID), tele.ModeHTML)
	})

	// دریافت تصویر فیش
	bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		user := c.Sender()
		if IsUserBlocked(user.ID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		stateMu.RLock()
		amount := userWalletTemp[user.ID]
		stateMu.RUnlock()

		if amount <= 0 {
			return c.Send("📸 تصویر شما دریافت شد.")
		}

		res, err := db.Exec(`INSERT INTO transactions (user_id, amount, status) VALUES (?, ?, 'pending')`, user.ID, amount)
		if err != nil {
			log.Printf("❌ خطا در ثبت تراکنش: %v", err)
			return c.Send("❌ خطایی در پردازش اطلاعات رخ داد. لطفاً مجدداً تلاش کنید.")
		}
		txID, _ := res.LastInsertId()

		var dbJoinedAt time.Time
		var phone, selfStatus string
		var purchasesCount int

		err = db.QueryRow("SELECT joined_at, phone, self_status, purchases_count FROM users WHERE id = ?", user.ID).Scan(&dbJoinedAt, &phone, &selfStatus, &purchasesCount)
		if err != nil || dbJoinedAt.IsZero() {
			dbJoinedAt = time.Now()
		}

		loc := getTehranLocation()
		tJoined := gpc.New(dbJoinedAt.In(loc))
		tJoinedStr := toPersianDigits(tJoined.Format("yyyy/MM/dd"))

		usernameStr := "ثبت نشده"
		if user.Username != "" {
			usernameStr = "@" + html.EscapeString(user.Username)
		}

		price := getKeyPrice()
		captionText := fmt.Sprintf(
			"🔔 <b>درخواست شارژ (کارت به کارت)</b>\n\n🆔 <b>شناسه فاکتور:</b> <code>#%d</code>\n👤 %s (%s)\n🆔 <code>%d</code>\n💰 <b>مبلغ:</b> <code>%s تومان</code>\n🔑 <b>تعداد کلید:</b> <code>%.2f کلید</code>\n📅 <b>عضویت:</b> %s\n🔥 <b>وضعیت سلف:</b> %s",
			txID, html.EscapeString(user.FirstName), usernameStr, user.ID, formatMoney(amount), float64(amount)/float64(price), tJoinedStr, html.EscapeString(selfStatus),
		)

		menu := &tele.ReplyMarkup{}
		btnApprove := menu.Data("✅ تایید", "admin_approve", strconv.FormatInt(txID, 10))
		btnReject := menu.Data("❌ رد", "admin_reject", strconv.FormatInt(txID, 10))
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

		stateMu.Lock()
		userWalletTemp[user.ID] = 0
		stateMu.Unlock()

		return c.Send("✅ <b>فیش واریزی شما با موفقیت برای ادمین ارسال شد.</b>\n\nپس از بررسی و تایید، موجودی کیف پول شما به‌روزرسانی خواهد شد.", tele.ModeHTML, getMainKeyboard(user.ID))
	})

	// تایید فیش توسط ادمین
	bot.Handle(&tele.Btn{Unique: "admin_approve"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}

		txID, err := strconv.ParseInt(c.Data(), 10, 64)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ داده نامعتبر است."})
		}

		tx, err := db.Begin()
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ خطای سرور دیتابیس!", ShowAlert: true})
		}
		defer tx.Rollback()

		var currentStatus string
		var targetUserID int64
		var amount int

		err = tx.QueryRow("SELECT status, user_id, amount FROM transactions WHERE id = ? FOR UPDATE", txID).Scan(&currentStatus, &targetUserID, &amount)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ فاکتور یافت نشد!", ShowAlert: true})
		}

		if currentStatus != "pending" {
			return c.Respond(&tele.CallbackResponse{Text: "⚠️ این فیش قبلاً تعیین تکلیف شده است!", ShowAlert: true})
		}

		_, err = tx.Exec("UPDATE transactions SET status = 'approved' WHERE id = ?", txID)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ خطا در ثبت وضعیت فاکتور!", ShowAlert: true})
		}

		_, err = tx.Exec(`INSERT IGNORE INTO users (id, first_name, username) VALUES (?, 'کاربر', 'ثبت_نشده')`, targetUserID)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ خطا در بروزرسانی کاربر!", ShowAlert: true})
		}

		_, err = tx.Exec(`INSERT INTO wallets (user_id, balance) VALUES (?, ?) ON DUPLICATE KEY UPDATE balance = balance + ?`, targetUserID, amount, amount)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ خطا در بروزرسانی کیف پول!", ShowAlert: true})
		}

		_, err = tx.Exec(`UPDATE users SET purchases_count = purchases_count + 1 WHERE id = ?`, targetUserID)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ خطا در بروزرسانی خریدها!", ShowAlert: true})
		}

		if err = tx.Commit(); err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ خطا در نهایی‌سازی تراکنش!", ShowAlert: true})
		}

		_, _ = bot.Send(&tele.User{ID: targetUserID}, fmt.Sprintf("🎉 <b>فیش واریزی شما تایید شد!</b>\n\nمبلغ <code>%s تومان</code> به کیف پول شما اضافه گردید. 💳", formatMoney(amount)), tele.ModeHTML)

		if c.Message() != nil {
			updatedCaption := c.Message().Caption + "\n\n✅ <b>وضعیت: فیش تایید شد و موجودی کاربر شارژ گردید.</b>"
			_, _ = bot.EditCaption(c.Message(), updatedCaption, tele.ModeHTML, &tele.ReplyMarkup{})
			_ = c.Reply(fmt.Sprintf("✅ <b>شارژ با موفقیت انجام شد!</b>\nمبلغ <code>%s تومان</code> به کیف پول کاربر اضافه گردید.", formatMoney(amount)), tele.ModeHTML)
		}
		return c.Respond(&tele.CallbackResponse{Text: "✅ فیش تایید شد."})
	})

	// رد فیش توسط ادمین
	bot.Handle(&tele.Btn{Unique: "admin_reject"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}

		txID, err := strconv.ParseInt(c.Data(), 10, 64)
		if err != nil {
			return c.Respond()
		}

		tx, err := db.Begin()
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "❌ خطا در سرور دیتابیس!", ShowAlert: true})
		}
		defer tx.Rollback()

		var currentStatus string
		var targetUserID int64
		err = tx.QueryRow("SELECT status, user_id FROM transactions WHERE id = ? FOR UPDATE", txID).Scan(&currentStatus, &targetUserID)
		if err != nil || currentStatus != "pending" {
			return c.Respond(&tele.CallbackResponse{Text: "⚠️ این فیش قبلاً تعیین تکلیف شده است!", ShowAlert: true})
		}

		_, _ = tx.Exec("UPDATE transactions SET status = 'rejected' WHERE id = ?", txID)
		_ = tx.Commit()

		_, _ = bot.Send(&tele.User{ID: targetUserID}, "❌ <b>فیش واریزی شما توسط ادمین رد شد.</b>\n\nلطفاً در صورت وجود مشکل با پشتیبانی ارتباط برقرار کنید.", tele.ModeHTML)

		if c.Message() != nil {
			updatedCaption := c.Message().Caption + "\n\n❌ <b>وضعیت: فیش واریزی رد شد.</b>"
			_, _ = bot.EditCaption(c.Message(), updatedCaption, tele.ModeHTML, &tele.ReplyMarkup{})
			_ = c.Reply("❌ <b>فیش رد شد و به کاربر اطلاع داده شد.</b>", tele.ModeHTML)
		}
		return c.Respond(&tele.CallbackResponse{Text: "❌ فیش رد شد."})
	})

	// دکمه خرید سلف
	bot.Handle(&btnBuy, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		selfStatus := GetUserSelfStatus(userID)
		price := getKeyPrice()
		balance := GetUserBalance(userID)
		keys := balance / price

		if selfStatus == "روشن" || selfStatus == "خاموش" {
			loc := getTehranLocation()
			now := time.Now().In(loc)
			tNow := gpc.New(now)
			tNowStr := toPersianDigits(tNow.Format("yyyy/MM/dd"))
			tTimeStr := toPersianDigits(tNow.Format("HH:mm:ss"))

			text := fmt.Sprintf(
				"🎉 <b>سلف شما فعال هست!</b> 🐺\n\n"+
					"🔑 <b>تعداد کلیدهای شما:</b> <code>%d</code> عدد\n"+
					"📅 <b>تاریخ:</b> %s\n"+
					"⏰ <b>ساعت:</b> %s",
				keys, tNowStr, tTimeStr,
			)
			return c.Send(text, getKeyboard(userID), tele.ModeHTML)
		}

		if keys < 30 {
			text := fmt.Sprintf(
				"❌ <b>سلام شما کلید لازم برای شروع ندارید !</b>\n\n"+
					"⏳ <i>سلف روزانه بیلینگ میشه : هر روز یک کلید از حسابت کم میشه !</i>\n\n"+
					"🔑 تعداد کلید های موجود شما <b>%d</b> عدد هست!\n\n"+
					"⚠️ <b>برای فعالسازی حداقل باید 30 کلید داشته باشید ..</b>\n\n"+
					"🛒 <i>لطفا از بخش کیف پول کلید خریداری نمایید.</i>", keys,
			)
			return c.Send(text, tele.ModeHTML)
		}

		text := fmt.Sprintf(
			"🎉 <b>سلام شما کلید لازم برای شروع را دارید !</b>\n\n"+
				"⏳ <i>سلف روزانه بیلینگ میشه : هر روز یک کلید از حسابت کم میشه !</i>\n\n"+
				"🔑 تعداد کلید های موجود شما <b>%d</b> عدد هست!\n\n"+
				"✅ <b>برای فعالسازی سلف و شروع کسر کلید روی دکمه زیر کلیک کنید.</b>", keys,
		)

		return c.Send(text, confirmSelfMenu, tele.ModeHTML)
	})
}
