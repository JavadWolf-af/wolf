package main

import (
	tele "gopkg.in/telebot.v3"
)

// ==========================================
// 1. تعریف منوهای اصلی و مدیریت ربات مادر
// ==========================================
var (
	userMenu           = &tele.ReplyMarkup{ResizeKeyboard: true}
	adminMenu          = &tele.ReplyMarkup{ResizeKeyboard: true}
	adminPanelMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	accountConfigMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	supportConfigMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	profileMenu        = &tele.ReplyMarkup{ResizeKeyboard: true}
	confirmSelfMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	walletReplyMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	waitingReceiptMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideTranslateMenu = &tele.ReplyMarkup{ResizeKeyboard: true}

	btnBuy        = userMenu.Text("🛍️ خرید سلف")
	btnProfile    = userMenu.Text("👤 حساب کاربری")
	btnWallet     = userMenu.Text("👛 کیف پول 💳")
	btnWolfPlus   = userMenu.Text("🐺 ولف +")
	btnSupport    = userMenu.Text("🎧 پشتیبانی")
	btnGuide      = userMenu.Text("📚 راهنما")
	btnAdminPanel = adminMenu.Text("⚙️ مدیریت")
	btnBack       = adminMenu.Text("🔙 بازگشت")

	btnConfigAccount  = adminPanelMenu.Text("🛠 تنظیم حساب بانکی")
	btnConfigSupport  = adminPanelMenu.Text("📞 تنظیم پشتیبانی")
	btnConfigKeyPrice = adminPanelMenu.Text("🔑 تنظیم نرخ کلید")

	btnConfigCardNum  = accountConfigMenu.Text("💳 شماره کارت")
	btnConfigCardName = accountConfigMenu.Text("👤 نام صاحب حساب")
	btnConfigCardBank = accountConfigMenu.Text("🏦 نام بانک")
	btnBackToAdminAcc = accountConfigMenu.Text("🔙 بازگشت به مدیریت")

	btnConfigSupportText = supportConfigMenu.Text("📝 تنظیم متن پشتیبانی")
	btnConfigSupportID   = supportConfigMenu.Text("🆔 تنظیم آیدی پشتیبانی")
	btnBackToAdminSup    = supportConfigMenu.Text("🔙 بازگشت به مدیریت")

	btnTurnOnSelf  = profileMenu.Text("🟢 روشن کردن سلف")
	btnTurnOffSelf = profileMenu.Text("🔴 خاموش کردن سلف")
	btnExitSelf    = profileMenu.Text("🛑 خروج سلف")

	btnConfirmSelfAction = confirmSelfMenu.Text("🟢 تایید و فعالسازی")
	btnWalletConfirm     = walletReplyMenu.Text("✅ تایید و ساخت فاکتور")
	btnCancelReceipt     = waitingReceiptMenu.Text("🔙 لغو و بازگشت به منوی اصلی")
)

// ==========================================
// 2. تعریف منوهای بخش راهنمای سلف
// ==========================================
var (
	guideMenu          = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideClockMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideEmojiMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideBioMenu       = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideFriendMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideEnemyMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideFontMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideActionMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	guidePurgeMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideTimerMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideAutoReactMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	guidePVMenu        = &tele.ReplyMarkup{ResizeKeyboard: true}
	guideGroupMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}

	btnGClock     = guideMenu.Text("⏱ ساعت زنده")
	btnGEmoji     = guideMenu.Text("🎭 اموجی رندوم")
	btnGBio       = guideMenu.Text("📝 بیوگرافی هوشمند")
	btnGFont      = guideMenu.Text("✒️ خوشنویسی")
	btnGFriend    = guideMenu.Text("🌸 دوست")
	btnGEnemy     = guideMenu.Text("⚔️ دشمن")
	btnGAction    = guideMenu.Text("🎬 اکشن‌ها")
	btnGPurge     = guideMenu.Text("🗑 پاکسازی")
	btnGTimer     = guideMenu.Text("⏳ تایمر")
	btnGAutoReact = guideMenu.Text("🔥 ری‌اکشن خودکار")
	btnGPV        = guideMenu.Text("📩 پیوی همه")
	btnGGroup     = guideMenu.Text("👥 گروه همه")
	btnGTranslate    = tele.Btn{Text: "🌍 مترجم هوشمند"}
	btnGBackMain  = guideMenu.Text("🔙 بازگشت به منوی اصلی")

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
	btnFontBoldItalic = guideFontMenu.Text("✨ بولد ایتالیک (پیش‌فرض)")
	btnFontBold       = guideFontMenu.Text("🖋 بولد")
	btnFontItalic     = guideFontMenu.Text("🖊 ایتالیک")
	btnFontUnderline  = guideFontMenu.Text("📜 زیر خط")
	btnFontStrike     = guideFontMenu.Text("❌ خط خورده")
	btnFontMono       = guideFontMenu.Text("💻 مونو")
	btnFontSpoiler    = guideFontMenu.Text("🕵️ اسپویل")
	btnFontBack       = guideFontMenu.Text("🔙 بازگشت به راهنما")

	btnActionBack    = guideActionMenu.Text("🔙 بازگشت به راهنما")
	btnPurgeBack     = guidePurgeMenu.Text("🔙 بازگشت به راهنما")
	btnTimerBack     = guideTimerMenu.Text("🔙 بازگشت به راهنما")
	btnAutoReactBack = guideAutoReactMenu.Text("🔙 بازگشت به راهنما")
	btnPVBack        = guidePVMenu.Text("🔙 بازگشت به راهنما")
	btnGroupBack     = guideGroupMenu.Text("🔙 بازگشت به راهنما")

    )

// ==========================================
// 3. تعریف منوهای بخش امکانات ویژه (ولف +)
// ==========================================
var (
	wolfPlusMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	antiDelMenu   = &tele.ReplyMarkup{ResizeKeyboard: true}
	editLogMenu   = &tele.ReplyMarkup{ResizeKeyboard: true}
	timerMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	groupDelMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	targetMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	protectedMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	ghostMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	notifyMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	pvLockMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}

	btnWP_AntiDel   = wolfPlusMenu.Text("🗑 ضد حذف")
	btnWP_EditLog   = wolfPlusMenu.Text("📝 ادیت لاگر")
	btnWP_Timer     = wolfPlusMenu.Text("📸 رسانه تایمردار")
	btnWP_Group     = wolfPlusMenu.Text("👥 ضد حذف گروه")
	btnWP_Target    = wolfPlusMenu.Text("🎯 ردیاب مخاطب")
	btnWP_Protected = wolfPlusMenu.Text("🔓 دانلودر ضدکپی")
	btnWP_Ghost     = wolfPlusMenu.Text("👻 حالت روح")
	btnWP_Notify    = wolfPlusMenu.Text("🔔 اعلان ربات")
	btnWP_PVLock    = wolfPlusMenu.Text("🔒 قفل پیوی")
	btnWP_Refresh   = wolfPlusMenu.Text("🔄 بروزرسانی وضعیت")
	btnWP_BackMain  = wolfPlusMenu.Text("🔙 بازگشت به منوی اصلی")

	btnAD_On   = antiDelMenu.Text("🟢 روشن کردن ضد حذف")
	btnAD_Off  = antiDelMenu.Text("🔴 خاموش کردن ضد حذف")
	btnAD_Back = antiDelMenu.Text("🔙 بازگشت به ولف +")

	btnEL_On   = editLogMenu.Text("🟢 روشن کردن ادیت لاگر")
	btnEL_Off  = editLogMenu.Text("🔴 خاموش کردن ادیت لاگر")
	btnEL_Back = editLogMenu.Text("🔙 بازگشت به ولف +")

	btnTM_On   = timerMenu.Text("🟢 روشن کردن تایمردار")
	btnTM_Off  = timerMenu.Text("🔴 خاموش کردن تایمردار")
	btnTM_Back = timerMenu.Text("🔙 بازگشت به ولف +")

	btnGD_Add   = groupDelMenu.Text("➕ افزودن گروه به ضد حذف")
	btnGD_Clear = groupDelMenu.Text("🗑 پاکسازی لیست گروه‌ها")
	btnGD_Back  = groupDelMenu.Text("🔙 بازگشت به ولف +")

	btnTG_Add          = targetMenu.Text("➕ افزودن مخاطب")
	btnTG_Delete       = targetMenu.Text("➖ حذف مخاطب")
	btnTG_List         = targetMenu.Text("📋 لیست مخاطبان")
	btnTG_Clear        = targetMenu.Text("🗑 پاکسازی لیست اهداف")
	btnTG_Back         = targetMenu.Text("🔙 بازگشت به ولف +")
	btnTG_BackToTarget = targetMenu.Text("🔙 بازگشت به ردیاب")

	btnPC_On             = protectedMenu.Text("🟢 روشن کردن دانلودر ضدکپی")
	btnPC_Off            = protectedMenu.Text("🔴 خاموش کردن دانلودر ضدکپی")
	btnPC_Add            = protectedMenu.Text("➕ افزودن کانال/گروه")
	btnPC_Delete         = protectedMenu.Text("➖ حذف کانال/گروه")
	btnPC_List           = protectedMenu.Text("📋 لیست کانال‌های قفل")
	btnPC_Clear          = protectedMenu.Text("🗑 پاکسازی لیست")
	btnPC_Back           = protectedMenu.Text("🔙 بازگشت به ولف +")
	btnPC_BackToProtMenu = protectedMenu.Text("🔙 بازگشت به منوی ضدکپی")

	btnGH_On   = ghostMenu.Text("🟢 روشن کردن حالت روح")
	btnGH_Off  = ghostMenu.Text("🔴 خاموش کردن حالت روح")
	btnGH_Back = ghostMenu.Text("🔙 بازگشت به ولف +")

	btnNT_On   = notifyMenu.Text("🟢 روشن کردن اعلان‌ها")
	btnNT_Off  = notifyMenu.Text("🔴 خاموش کردن اعلان‌ها")
	btnNT_Back = notifyMenu.Text("🔙 بازگشت به ولف +")

	btnPV_On   = pvLockMenu.Text("🟢 قفل پیوی روشن")
	btnPV_Off  = pvLockMenu.Text("🔴 قفل پیوی خاموش")
	btnPV_Back = pvLockMenu.Text("🔙 بازگشت به ولف +")
)

// ==========================================
// چیدمان و ساختار کیبوردها (تابع init به صورت خودکار اجرا می‌شود)
// ==========================================
func init() {
	// چیدمان منوهای اصلی
	userMenu.Reply(
		userMenu.Row(btnBuy, btnProfile),
		userMenu.Row(btnWallet, btnWolfPlus),
		userMenu.Row(btnSupport, btnGuide),
	)

	adminMenu.Reply(
		adminMenu.Row(btnBuy, btnProfile),
		adminMenu.Row(btnWallet, btnWolfPlus),
		adminMenu.Row(btnSupport, btnGuide),
		adminMenu.Row(btnAdminPanel),
	)

	adminPanelMenu.Reply(
		adminPanelMenu.Row(btnConfigAccount, btnConfigSupport),
		adminPanelMenu.Row(btnConfigKeyPrice),
		adminPanelMenu.Row(btnBack),
	)

	accountConfigMenu.Reply(
		accountConfigMenu.Row(btnConfigCardNum, btnConfigCardName),
		accountConfigMenu.Row(btnConfigCardBank),
		accountConfigMenu.Row(btnBackToAdminAcc),
	)

	supportConfigMenu.Reply(
		supportConfigMenu.Row(btnConfigSupportText, btnConfigSupportID),
		supportConfigMenu.Row(btnBackToAdminSup),
	)

	profileMenu.Reply(
		profileMenu.Row(btnTurnOnSelf, btnTurnOffSelf),
		profileMenu.Row(btnExitSelf),
		profileMenu.Row(btnBack),
	)

	confirmSelfMenu.Reply(
		confirmSelfMenu.Row(btnConfirmSelfAction),
		confirmSelfMenu.Row(btnBack),
	)

	walletReplyMenu.Reply(
		walletReplyMenu.Row(btnWalletConfirm),
		walletReplyMenu.Row(btnBack),
	)

	waitingReceiptMenu.Reply(waitingReceiptMenu.Row(btnCancelReceipt))

	// چیدمان منوهای راهنما
	guideMenu.Reply(
		guideMenu.Row(btnGClock, btnGEmoji),
		guideMenu.Row(btnGBio, btnGFont),
		guideMenu.Row(btnGFriend, btnGEnemy),
		guideMenu.Row(btnGAction, btnGPurge),
		guideMenu.Row(btnGTimer, btnGAutoReact),
		guideMenu.Row(btnGPV, btnGGroup),
		guideMenu.Row(btnGBackMain),
	)
	guideClockMenu.Reply(guideClockMenu.Row(btnClockOn, btnClockOff), guideClockMenu.Row(btnClockBack))
	guideEmojiMenu.Reply(guideEmojiMenu.Row(btnEmojiOn, btnEmojiOff), guideEmojiMenu.Row(btnEmojiBack))
	guideBioMenu.Reply(guideBioMenu.Row(btnBioOn, btnBioOff), guideBioMenu.Row(btnBioBack))
	guideFriendMenu.Reply(guideFriendMenu.Row(btnFriendList, btnFriendClear), guideFriendMenu.Row(btnFriendBack))
	guideEnemyMenu.Reply(guideEnemyMenu.Row(btnEnemyList, btnEnemyClear), guideEnemyMenu.Row(btnEnemyBack))
	guideFontMenu.Reply(
		guideFontMenu.Row(btnFontOn, btnFontOff),
		guideFontMenu.Row(btnFontBoldItalic),
		guideFontMenu.Row(btnFontBold, btnFontItalic),
		guideFontMenu.Row(btnFontUnderline, btnFontStrike),
		guideFontMenu.Row(btnFontMono, btnFontSpoiler),
		guideFontMenu.Row(btnFontBack),
	)
	guideActionMenu.Reply(guideActionMenu.Row(btnActionBack))
	guidePurgeMenu.Reply(guidePurgeMenu.Row(btnPurgeBack))
	guideTimerMenu.Reply(guideTimerMenu.Row(btnTimerBack))
	guideAutoReactMenu.Reply(guideAutoReactMenu.Row(btnAutoReactBack))
	guidePVMenu.Reply(guidePVMenu.Row(btnPVBack))
	guideGroupMenu.Reply(guideGroupMenu.Row(btnGroupBack))

	// چیدمان منوهای ولف پلاس
	wolfPlusMenu.Reply(
		wolfPlusMenu.Row(btnWP_AntiDel, btnWP_EditLog),
		wolfPlusMenu.Row(btnWP_Timer, btnWP_Group),
		wolfPlusMenu.Row(btnWP_Target, btnWP_Protected),
		wolfPlusMenu.Row(btnWP_Ghost, btnWP_Notify),
		wolfPlusMenu.Row(btnWP_PVLock),
		wolfPlusMenu.Row(btnWP_Refresh, btnWP_BackMain),
	)
	antiDelMenu.Reply(antiDelMenu.Row(btnAD_On, btnAD_Off), antiDelMenu.Row(btnAD_Back))
	editLogMenu.Reply(editLogMenu.Row(btnEL_On, btnEL_Off), editLogMenu.Row(btnEL_Back))
	timerMenu.Reply(timerMenu.Row(btnTM_On, btnTM_Off), timerMenu.Row(btnTM_Back))
	groupDelMenu.Reply(groupDelMenu.Row(btnGD_Add, btnGD_Clear), groupDelMenu.Row(btnGD_Back))
	targetMenu.Reply(targetMenu.Row(btnTG_Add, btnTG_Delete), targetMenu.Row(btnTG_List, btnTG_Clear), targetMenu.Row(btnTG_Back))
	protectedMenu.Reply(
		protectedMenu.Row(btnPC_On, btnPC_Off),
		protectedMenu.Row(btnPC_Add, btnPC_Delete),
		protectedMenu.Row(btnPC_List, btnPC_Clear),
		protectedMenu.Row(btnPC_Back),
	)
	ghostMenu.Reply(ghostMenu.Row(btnGH_On, btnGH_Off), ghostMenu.Row(btnGH_Back))
	notifyMenu.Reply(notifyMenu.Row(btnNT_On, btnNT_Off), notifyMenu.Row(btnNT_Back))
	pvLockMenu.Reply(pvLockMenu.Row(btnPV_On, btnPV_Off), pvLockMenu.Row(btnPV_Back))
}

// دکمه‌های شیشه‌ای کیف پول (Inline Keyboard)
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
