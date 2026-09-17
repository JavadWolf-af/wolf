package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

var (
	guideMenu           = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideClockMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideEmojiMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideBioMenu        = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideFriendMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideEnemyMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideFontMenu       = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideActionMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guidePurgeMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideTimerMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideAutoReactMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideTranslatorMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	guidePVLockMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guidePVMenu         = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideGroupMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}

	btnGClock      = guideMenu.Text("⏱ ساعت زنده")
	btnGEmoji      = guideMenu.Text("🎭 اموجی رندوم")
	btnGBio        = guideMenu.Text("📝 بیوگرافی هوشمند")
	btnGFont       = guideMenu.Text("✒️ خوشنویسی")
	btnGFriend     = guideMenu.Text("🌸 دوست")
	btnGEnemy      = guideMenu.Text("⚔️ دشمن")
	btnGAction     = guideMenu.Text("🎬 اکشن‌ها")
	btnGPurge      = guideMenu.Text("🗑 پاکسازی")
	btnGTimer      = guideMenu.Text("⏳ تایمر")
	btnGAutoReact  = guideMenu.Text("🔥 ری اکشن خودکار")
	btnGTranslator = guideMenu.Text("🌍 مترجم در لحظه")
	btnGPVLock     = guideMenu.Text("🔐 قفل پیوی")
	btnGPV         = guideMenu.Text("📩 پیوی همه")
	btnGGroup      = guideMenu.Text("👥 گروه همه")
	btnGBackMain   = guideMenu.Text("🔙 بازگشت به منوی اصلی")

	btnClockOn   = guideClockMenu.Text("🟢 روشن کردن ساعت")
	btnClockOff  = guideClockMenu.Text("🔴 خاموش کردن ساعت")
	btnClockBack = guideClockMenu.Text("🔙 بازگشت به راهنما")

	btnEmojiOn   = guideEmojiMenu.Text("🟢 روشن کردن اموجی")
	btnEmojiOff  = guideEmojiMenu.Text("🔴 خاموش کردن اموجی")
	btnEmojiBack = guideEmojiMenu.Text("🔙 بازگشت به راهنما")

	btnBioOn   = guideBioMenu.Text("🟢 روشن کردن بیو (رندوم)")
	btnBioOff  = guideBioMenu.Text("🔴 خاموش کردن بیو")
	btnBioBack = guideBioMenu.Text("🔙 بازگشت به راهنما")

	btnFriendList  = guideFriendMenu.Text("📋 لیست دوستان")
	btnFriendClear = guideFriendMenu.Text("🗑 پاکسازی دوستان")
	btnFriendBack  = guideFriendMenu.Text("🔙 بازگشت به راهنما")

	btnEnemyList  = guideEnemyMenu.Text("📋 لیست دشمنان")
	btnEnemyClear = guideEnemyMenu.Text("🗑 پاکسازی دشمنان")
	btnEnemyBack  = guideEnemyMenu.Text("🔙 بازگشت به راهنما")

	btnFontOn         = guideFontMenu.Text("🟢 روشن کردن خوشنویسی")
	btnFontOff        = guideFontMenu.Text("🔴 خاموش کردن خوشنویسی")
	btnFontBoldItalic = guideFontMenu.Text("✨ بولد ایتالیک")
	btnFontBold       = guideFontMenu.Text("🖋 بولد (پیش‌فرض)")
	btnFontItalic     = guideFontMenu.Text("🖊 ایتالیک")
	btnFontUnderline  = guideFontMenu.Text("📜 زیر خط")
	btnFontStrike     = guideFontMenu.Text("❌ خط خورده")
	btnFontMono       = guideFontMenu.Text("💻 مونو")
	btnFontSpoiler    = guideFontMenu.Text("🕵️ اسپویل")
	btnFontBack       = guideFontMenu.Text("🔙 بازگشت به راهنما")

	btnActionBack     = guideActionMenu.Text("🔙 بازگشت به راهنما")
	btnPurgeBack      = guidePurgeMenu.Text("🔙 بازگشت به راهنما")
	btnTimerBack      = guideTimerMenu.Text("🔙 بازگشت به راهنما")
	btnAutoReactBack  = guideAutoReactMenu.Text("🔙 بازگشت به راهنما")
	btnTranslatorBack = guideTranslatorMenu.Text("🔙 بازگشت به راهنما")
	btnPVLockBack     = guidePVLockMenu.Text("🔙 بازگشت به راهنما")
	btnPVBack         = guidePVMenu.Text("🔙 بازگشت به راهنما")
	btnGroupBack      = guideGroupMenu.Text("🔙 بازگشت به راهنما")
)

func initGuideMarkups() {
	guideMenu.Reply(
		guideMenu.Row(btnGClock, btnGEmoji),
		guideMenu.Row(btnGBio, btnGFont),
		guideMenu.Row(btnGFriend, btnGEnemy),
		guideMenu.Row(btnGAction, btnGPurge),
		guideMenu.Row(btnGTimer, btnGAutoReact),
		guideMenu.Row(btnGTranslator, btnGPVLock),
		guideMenu.Row(btnGPV, btnGGroup),
		guideMenu.Row(btnGBackMain),
	)

	guideClockMenu.Reply(
		guideClockMenu.Row(btnClockOn, btnClockOff),
		guideClockMenu.Row(btnClockBack),
	)

	guideEmojiMenu.Reply(
		guideEmojiMenu.Row(btnEmojiOn, btnEmojiOff),
		guideEmojiMenu.Row(btnEmojiBack),
	)

	guideBioMenu.Reply(
		guideBioMenu.Row(btnBioOn, btnBioOff),
		guideBioMenu.Row(btnBioBack),
	)

	guideFriendMenu.Reply(
		guideFriendMenu.Row(btnFriendList, btnFriendClear),
		guideFriendMenu.Row(btnFriendBack),
	)

	guideEnemyMenu.Reply(
		guideEnemyMenu.Row(btnEnemyList, btnEnemyClear),
		guideEnemyMenu.Row(btnEnemyBack),
	)

	guideFontMenu.Reply(
		guideFontMenu.Row(btnFontOn, btnFontOff),
		guideFontMenu.Row(btnFontBold, btnFontBoldItalic),
		guideFontMenu.Row(btnFontItalic, btnFontUnderline),
		guideFontMenu.Row(btnFontStrike, btnFontMono),
		guideFontMenu.Row(btnFontSpoiler),
		guideFontMenu.Row(btnFontBack),
	)

	guideActionMenu.Reply(guideActionMenu.Row(btnActionBack))
	guidePurgeMenu.Reply(guidePurgeMenu.Row(btnPurgeBack))
	guideTimerMenu.Reply(guideTimerMenu.Row(btnTimerBack))
	guideAutoReactMenu.Reply(guideAutoReactMenu.Row(btnAutoReactBack))
	guideTranslatorMenu.Reply(guideTranslatorMenu.Row(btnTranslatorBack))
	guidePVLockMenu.Reply(guidePVLockMenu.Row(btnPVLockBack))
	guidePVMenu.Reply(guidePVMenu.Row(btnPVBack))
	guideGroupMenu.Reply(guideGroupMenu.Row(btnGroupBack))
}

func buildGuideDashboardText(userID int64) string {
	var isClock, isEmoji, isBio, isFont bool
	var bioMode string
	_ = db.QueryRow("SELECT is_clock_enabled, is_emoji_enabled, is_bio_enabled, bio_mode, is_font_enabled FROM users WHERE id = ?", userID).Scan(&isClock, &isEmoji, &isBio, &bioMode, &isFont)
	var friendCount, enemyCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM wolf_friends WHERE owner_id = ?", userID).Scan(&friendCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM wolf_enemies WHERE owner_id = ?", userID).Scan(&enemyCount)

	clockStatus := "🔴 خاموش"
	if isClock {
		clockStatus = "🟢 روشن"
	}
	emojiStatus := "🔴 خاموش"
	if isEmoji {
		emojiStatus = "🟢 روشن"
	}
	bioStatus := "🔴 خاموش"
	if isBio {
		if bioMode == "custom" {
			bioStatus = "🟢 روشن (دستی)"
		} else {
			bioStatus = "🟢 روشن (رندوم)"
		}
	}
	fontStatus := "🔴 خاموش"
	if isFont {
		fontStatus = "🟢 روشن"
	}
	pvLockStatus := "🔴 خاموش"
	if IsPVLockEnabled(userID) {
		pvLockStatus = "🟢 روشن (امنیتی)"
	}

	return fmt.Sprintf(`📚 <b>بخش راهنما و امکانات سلف ولف 🐺</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>وضعیت لحظه‌ای قابلیت‌ها:</b>
▫️ ⏱ <b>ساعت زنده:</b> %s
▫️ 🎭 <b>اموجی رندوم:</b> %s
▫️ 📝 <b>بیوگرافی هوشمند:</b> %s
▫️ ✒️ <b>خوشنویسی پیام‌ها:</b> %s
▫️ 🌸 <b>سیستم دوست:</b> <code>%d نفر</code>
▫️ ⚔️ <b>سیستم دشمن:</b> <code>%d نفر</code>
▫️ 🎬 <b>اکشن‌های جعلی:</b> فعال
▫️ 🗑 <b>پاکسازی پیام‌ها:</b> فعال
▫️ ⏳ <b>تایمر زنده:</b> فعال
▫️ 🔥 <b>ری اکشن خودکار:</b> فعال
▫️ 🌍 <b>مترجم زنده:</b> فعال
▫️ 🔐 <b>قفل پیوی ضد مزاحم:</b> %s
➖➖➖➖➖➖➖➖➖➖
💡 <i>جهت مطالعه راهنما و تنظیم هر قابلیت، از کیبورد زیر انتخاب کنید:</i>`,
		clockStatus, emojiStatus, bioStatus, fontStatus, friendCount, enemyCount, pvLockStatus,
	)
}

func RegisterGuideHandlers(bot *tele.Bot) {
	initGuideMarkups()

	bot.Handle(&btnGuide, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب شما مسدود است.")
		}
		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Send("❌ <b>دسترسی محدود!</b>\n\nبخش راهنما فقط برای کاربرانی که اشتراک سلف را خریداری کرده‌اند فعال می‌باشد.", getKeyboard(userID), tele.ModeHTML)
		}
		return c.Send(buildGuideDashboardText(userID), guideMenu, tele.ModeHTML)
	})

	// ساعت
	bot.Handle(&btnGClock, func(c tele.Context) error {
		userID := c.Sender().ID
		var isClock bool
		_ = db.QueryRow("SELECT is_clock_enabled FROM users WHERE id = ?", userID).Scan(&isClock)
		statusStr := "🔴 خاموش"
		if isClock {
			statusStr = "🟢 روشن"
		}
		text := fmt.Sprintf(`⏱ <b>راهنمای ساعت زنده روی پروفایل</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 ساعت رسمی تهران به صورت زنده و با فونت بولد روی فامیلی اکانت قرار می‌گیرد.`, statusStr)
		return c.Send(text, guideClockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnClockOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideClockMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleClockOn(ctx, userID, ub.Client)
		return c.Send("🟢 <b>ساعت زنده فعال شد.</b>", guideClockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnClockOff, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideClockMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleClockOff(ctx, userID, ub.Client)
		return c.Send("🔴 <b>ساعت زنده خاموش شد.</b>", guideClockMenu, tele.ModeHTML)
	})

	// اموجی
	bot.Handle(&btnGEmoji, func(c tele.Context) error {
		userID := c.Sender().ID
		var isEmoji bool
		_ = db.QueryRow("SELECT is_emoji_enabled FROM users WHERE id = ?", userID).Scan(&isEmoji)
		statusStr := "🔴 خاموش"
		if isEmoji {
			statusStr = "🟢 روشن"
		}
		return c.Send(fmt.Sprintf("🎭 <b>اموجی رندوم</b>\nوضعیت: %s", statusStr), guideEmojiMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEmojiOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ سلف آنلاین نیست.", guideEmojiMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleEmojiOn(ctx, userID, ub.Client)
		return c.Send("🟢 اموجی روشن شد.", guideEmojiMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEmojiOff, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ سلف آنلاین نیست.", guideEmojiMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleEmojiOff(ctx, userID, ub.Client)
		return c.Send("🔴 اموجی خاموش شد.", guideEmojiMenu, tele.ModeHTML)
	})

	// بیو
	bot.Handle(&btnGBio, func(c tele.Context) error {
		userID := c.Sender().ID
		var isBio bool
		_ = db.QueryRow("SELECT is_bio_enabled FROM users WHERE id = ?", userID).Scan(&isBio)
		statusStr := "🔴 خاموش"
		if isBio {
			statusStr = "🟢 روشن"
		}
		return c.Send(fmt.Sprintf("📝 <b>بیوگرافی هوشمند</b>\nوضعیت: %s", statusStr), guideBioMenu, tele.ModeHTML)
	})

	bot.Handle(&btnBioOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ سلف آنلاین نیست.", guideBioMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleBioOn(ctx, userID, ub.Client)
		return c.Send("🟢 بیوگرافی فعال شد.", guideBioMenu, tele.ModeHTML)
	})

	bot.Handle(&btnBioOff, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ سلف آنلاین نیست.", guideBioMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleBioOff(ctx, userID, ub.Client)
		return c.Send("🔴 بیوگرافی خاموش شد.", guideBioMenu, tele.ModeHTML)
	})

	// دوست و دشمن
	bot.Handle(&btnGFriend, func(c tele.Context) error {
		var cnt int
		_ = db.QueryRow("SELECT COUNT(*) FROM wolf_friends WHERE owner_id = ?", c.Sender().ID).Scan(&cnt)
		return c.Send(fmt.Sprintf("🌸 <b>سیستم دوست</b>\nتعداد: %d نفر", cnt), guideFriendMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFriendList, func(c tele.Context) error {
		rows, err := db.Query("SELECT friend_id, friend_name FROM wolf_friends WHERE owner_id = ?", c.Sender().ID)
		if err != nil {
			return c.Send("❌ خطا در دریافت لیست.", guideFriendMenu, tele.ModeHTML)
		}
		defer rows.Close()
		var list []string
		idx := 1
		for rows.Next() {
			var fid int64
			var fname string
			if err := rows.Scan(&fid, &fname); err == nil {
				list = append(list, fmt.Sprintf("%d. %s (<code>%d</code>)", idx, fname, fid))
				idx++
			}
		}
		txt := "📋 <b>لیست دوستان:</b>\n\n" + strings.Join(list, "\n")
		if len(list) == 0 {
			txt = "⚠️ لیست دوستان خالی است."
		}
		return c.Send(txt, guideFriendMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFriendClear, func(c tele.Context) error {
		_, _ = db.Exec("DELETE FROM wolf_friends WHERE owner_id = ?", c.Sender().ID)
		clearFriendsCache(c.Sender().ID)
		return c.Send("🗑 لیست دوستان پاکسازی شد.", guideFriendMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGEnemy, func(c tele.Context) error {
		var cnt int
		_ = db.QueryRow("SELECT COUNT(*) FROM wolf_enemies WHERE owner_id = ?", c.Sender().ID).Scan(&cnt)
		return c.Send(fmt.Sprintf("⚔️ <b>سیستم دشمن</b>\nتعداد: %d نفر", cnt), guideEnemyMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEnemyList, func(c tele.Context) error {
		rows, err := db.Query("SELECT enemy_id, enemy_name FROM wolf_enemies WHERE owner_id = ?", c.Sender().ID)
		if err != nil {
			return c.Send("❌ خطا در دریافت لیست.", guideEnemyMenu, tele.ModeHTML)
		}
		defer rows.Close()
		var list []string
		idx := 1
		for rows.Next() {
			var eid int64
			var ename string
			if err := rows.Scan(&eid, &ename); err == nil {
				list = append(list, fmt.Sprintf("%d. %s (<code>%d</code>)", idx, ename, eid))
				idx++
			}
		}
		txt := "⚔️ <b>لیست دشمنان:</b>\n\n" + strings.Join(list, "\n")
		if len(list) == 0 {
			txt = "⚠️ لیست دشمنان خالی است."
		}
		return c.Send(txt, guideEnemyMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEnemyClear, func(c tele.Context) error {
		_, _ = db.Exec("DELETE FROM wolf_enemies WHERE owner_id = ?", c.Sender().ID)
		clearEnemiesCache(c.Sender().ID)
		return c.Send("🗑 لیست دشمنان پاکسازی شد.", guideEnemyMenu, tele.ModeHTML)
	})

	// خوشنویسی
	bot.Handle(&btnGFont, func(c tele.Context) error {
		en, mode := getFontSetting(c.Sender().ID)
		st := "🔴 خاموش"
		if en {
			st = "🟢 روشن"
		}
		return c.Send(fmt.Sprintf("✒️ <b>تنظیمات خوشنویسی</b>\nوضعیت: %s\nفونت: %s", st, mode), guideFontMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFontOn, func(c tele.Context) error {
		_, _ = db.Exec("UPDATE users SET is_font_enabled = TRUE WHERE id = ?", c.Sender().ID)
		_, m := getFontSetting(c.Sender().ID)
		updateFontCache(c.Sender().ID, true, m)
		return c.Send("🟢 خوشنویسی روشن شد.", guideFontMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFontOff, func(c tele.Context) error {
		_, _ = db.Exec("UPDATE users SET is_font_enabled = FALSE WHERE id = ?", c.Sender().ID)
		_, m := getFontSetting(c.Sender().ID)
		updateFontCache(c.Sender().ID, false, m)
		return c.Send("🔴 خوشنویسی خاموش شد.", guideFontMenu, tele.ModeHTML)
	})

	setFont := func(mode, label string) tele.HandlerFunc {
		return func(c tele.Context) error {
			_, _ = db.Exec("UPDATE users SET font_mode = ?, is_font_enabled = TRUE WHERE id = ?", mode, c.Sender().ID)
			updateFontCache(c.Sender().ID, true, mode)
			return c.Send(fmt.Sprintf("✅ فونت به «%s» تنظیم و روشن شد.", label), guideFontMenu, tele.ModeHTML)
		}
	}
	bot.Handle(&btnFontBold, setFont("bold", "بولد"))
	bot.Handle(&btnFontBoldItalic, setFont("bold_italic", "بولد ایتالیک"))
	bot.Handle(&btnFontItalic, setFont("italic", "ایتالیک"))
	bot.Handle(&btnFontUnderline, setFont("underline", "زیر خط"))
	bot.Handle(&btnFontStrike, setFont("strike", "خط خورده"))
	bot.Handle(&btnFontMono, setFont("mono", "مونو"))
	bot.Handle(&btnFontSpoiler, setFont("spoiler", "اسپویل"))

	// سایر راهنماها
	bot.Handle(&btnGAction, func(c tele.Context) error {
		return c.Send("🎬 <b>اکشن‌ها:</b>\nدستورات: تایپینگ 20، ضبط صدا 20 و...", guideActionMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGPurge, func(c tele.Context) error {
		return c.Send("🗑 <b>پاکسازی:</b>\nدستور: <code>پاکشو 20</code> یا ریپلای با <code>پاکشو</code>", guidePurgeMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGTimer, func(c tele.Context) error {
		return c.Send("⏳ <b>تایمر:</b>\nدستور: <code>تایمر 10</code>", guideTimerMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGAutoReact, func(c tele.Context) error {
		return c.Send("🔥 <b>ری‌اکشن:</b>\nبا ریپلای: <code>ری اکشن 🔥</code>", guideAutoReactMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGTranslator, func(c tele.Context) error {
		return c.Send("🌍 <b>مترجم:</b>\nبا ریپلای: <code>ترجمه</code> یا <code>انگلیسی شو</code>", guideTranslatorMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGPVLock, func(c tele.Context) error {
		return c.Send("🔐 <b>قفل پیوی:</b>\nپیش‌فرض خاموش است. روشن کردن: <code>قفل پیوی روشن</code>", guidePVLockMenu, tele.ModeHTML)
	})

	// برگشت‌ها
	backHandler := func(c tele.Context) error {
		return c.Send(buildGuideDashboardText(c.Sender().ID), guideMenu, tele.ModeHTML)
	}
	bot.Handle(&btnClockBack, backHandler)
	bot.Handle(&btnEmojiBack, backHandler)
	bot.Handle(&btnBioBack, backHandler)
	bot.Handle(&btnFriendBack, backHandler)
	bot.Handle(&btnEnemyBack, backHandler)
	bot.Handle(&btnFontBack, backHandler)
	bot.Handle(&btnActionBack, backHandler)
	bot.Handle(&btnPurgeBack, backHandler)
	bot.Handle(&btnTimerBack, backHandler)
	bot.Handle(&btnAutoReactBack, backHandler)
	bot.Handle(&btnTranslatorBack, backHandler)
	bot.Handle(&btnPVLockBack, backHandler)
	bot.Handle(&btnPVBack, backHandler)
	bot.Handle(&btnGroupBack, backHandler)
	bot.Handle(&btnGBackMain, func(c tele.Context) error {
		return c.Send("🔙 <b>منوی اصلی:</b>", getMainKeyboard(c.Sender().ID), tele.ModeHTML)
	})
}
