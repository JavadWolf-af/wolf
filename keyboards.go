package main

import tele "gopkg.in/telebot.v3"

// ==========================================
// تعریف تمامی منوها (کیبوردهای ثابت)
// ==========================================
var (
	userMenu           = &tele.ReplyMarkup{ResizeKeyboard: true}
	adminMenu          = &tele.ReplyMarkup{ResizeKeyboard: true}
	adminPanelMenu     = &tele.ReplyMarkup{ResizeKeyboard: true}
	profileMenu        = &tele.ReplyMarkup{ResizeKeyboard: true}
	wolfPlusMenu       = &tele.ReplyMarkup{ResizeKeyboard: true}
	pvLockMenu         = &tele.ReplyMarkup{ResizeKeyboard: true}
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
	guideTranslateMenu = &tele.ReplyMarkup{ResizeKeyboard: true} // منوی راهنمای مترجم

	walletReplyMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}
	waitingReceiptMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	confirmSelfMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}

	accountConfigMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
	supportConfigMenu  = &tele.ReplyMarkup{ResizeKeyboard: true}
)

// ==========================================
// تعریف تمامی دکمه‌ها (Buttons)
// ==========================================
var (
	btnProfile   = tele.Btn{Text: "👤 پروفایل من"}
	btnWolfPlus  = tele.Btn{Text: "🐺 امکانات ولف +"}
	btnGuide     = tele.Btn{Text: "📚 راهنما و امکانات"}
	btnWallet    = tele.Btn{Text: "👛 کیف پول"}
	btnSupport   = tele.Btn{Text: "پشتیبانی 📞"}
	btnBuy       = tele.Btn{Text: "خرید/تمدید سلف 🛒"}
	btnBack      = tele.Btn{Text: "🔙 بازگشت"}

	btnAdminPanel = tele.Btn{Text: "👑 پنل مدیریت"}

	btnBackToAdminAcc = tele.Btn{Text: "🔙 بازگشت به پنل"}
	btnBackToAdminSup = tele.Btn{Text: "🔙 بازگشت به پنل"}
	btnConfigAccount  = tele.Btn{Text: "تنظیمات شماره کارت"}
	btnConfigCardNum  = tele.Btn{Text: "تغییر شماره کارت"}
	btnConfigSupport  = tele.Btn{Text: "تنظیمات پشتیبانی"}

	btnTurnOnSelf        = tele.Btn{Text: "🟢 روشن کردن سلف"}
	btnTurnOffSelf       = tele.Btn{Text: "🔴 خاموش کردن سلف"}
	btnExitSelf          = tele.Btn{Text: "🛑 خروج از اکانت"}
	btnConfirmSelfAction = tele.Btn{Text: "تایید و ادامه"}
	btnCancelReceipt     = tele.Btn{Text: "لغو پرداخت"}
	btnWalletConfirm     = tele.Btn{Text: "✅ تایید مبلغ و پرداخت"}

	// دکمه‌های امکانات ولف پلاس
	btnWP_Ghost      = tele.Btn{Text: "👻 حالت روح"}
	btnWP_AntiDelete = tele.Btn{Text: "🚫 ضد حذف"}
	btnWP_PVLock     = tele.Btn{Text: "🔒 قفل پیوی"}
	btnWP_Logger     = tele.Btn{Text: "📝 لاگر پیام"}
	btnWP_Back       = tele.Btn{Text: "🔙 بازگشت به منوی اصلی"}

	btnPV_On   = tele.Btn{Text: "🟢 روشن"}
	btnPV_Off  = tele.Btn{Text: "🔴 خاموش"}
	btnPV_Back = tele.Btn{Text: "🔙 بازگشت به ولف +"}

	// دکمه‌های بخش راهنما
	btnGClock        = tele.Btn{Text: "⏱ ساعت زنده"}
	btnGEmoji        = tele.Btn{Text: "🎭 اموجی رندوم"}
	btnGBio          = tele.Btn{Text: "📝 بیو هوشمند"}
	btnGFriend       = tele.Btn{Text: "🌸 سیستم دوست"}
	btnGEnemy        = tele.Btn{Text: "⚔️ سیستم دشمن"}
	btnGFont         = tele.Btn{Text: "✒️ خوشنویسی"}
	btnGAction       = tele.Btn{Text: "🎬 اکشن جعلی"}
	btnGPurge        = tele.Btn{Text: "🗑 پاکسازی (Purge)"}
	btnGTimer        = tele.Btn{Text: "⏳ تایمر زنده"}
	btnGAutoReact    = tele.Btn{Text: "🔥 ری‌اکشن خودکار"}
	btnGPV           = tele.Btn{Text: "📩 پیوی همه"}
	btnGGroup        = tele.Btn{Text: "👥 گروه همه"}
	btnGTranslate    = tele.Btn{Text: "🌍 مترجم هوشمند"} // دکمه جدید مترجم
	btnGBackMain     = tele.Btn{Text: "🔙 بازگشت به منوی اصلی"}

	// دکمه‌های بازگشت در زیرمنوهای راهنما
	btnClockBack     = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnEmojiBack     = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnBioBack       = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnFriendBack    = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnEnemyBack     = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnFontBack      = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnActionBack    = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnPurgeBack     = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnTimerBack     = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnAutoReactBack = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnPVBack        = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnGroupBack     = tele.Btn{Text: "🔙 بازگشت به راهنما"}
	btnTranslateBack = tele.Btn{Text: "🔙 بازگشت به راهنما"} // دکمه برگشت صفحه مترجم

	// تنظیمات داخل راهنما
	btnClockOn  = tele.Btn{Text: "🟢 روشن کردن ساعت"}
	btnClockOff = tele.Btn{Text: "🔴 خاموش کردن ساعت"}
	btnEmojiOn  = tele.Btn{Text: "🟢 روشن کردن اموجی"}
	btnEmojiOff = tele.Btn{Text: "🔴 خاموش کردن اموجی"}
	btnBioOn    = tele.Btn{Text: "🟢 روشن کردن بیو"}
	btnBioOff   = tele.Btn{Text: "🔴 خاموش کردن بیو"}
	btnFriendList  = tele.Btn{Text: "📋 لیست دوستان"}
	btnFriendClear = tele.Btn{Text: "🗑 پاکسازی لیست دوستان"}
	btnEnemyList   = tele.Btn{Text: "📋 لیست دشمنان"}
	btnEnemyClear  = tele.Btn{Text: "🗑 پاکسازی لیست دشمنان"}
	btnFontOn  = tele.Btn{Text: "🟢 روشن کردن فونت"}
	btnFontOff = tele.Btn{Text: "🔴 خاموش کردن فونت"}

	btnFontBoldItalic = tele.Btn{Text: "بولد ایتالیک"}
	btnFontBold       = tele.Btn{Text: "بولد"}
	btnFontItalic     = tele.Btn{Text: "ایتالیک"}
	btnFontUnderline  = tele.Btn{Text: "زیر خط"}
	btnFontStrike     = tele.Btn{Text: "خط خورده"}
	btnFontMono       = tele.Btn{Text: "مونو"}
	btnFontSpoiler    = tele.Btn{Text: "اسپویل"}
)

func init() {
	// Main Menus
	userMenu.Reply(
		userMenu.Row(btnProfile),
		userMenu.Row(btnWolfPlus, btnGuide),
		userMenu.Row(btnWallet, btnBuy),
		userMenu.Row(btnSupport),
	)

	adminMenu.Reply(
		adminMenu.Row(btnAdminPanel),
		adminMenu.Row(btnProfile),
		adminMenu.Row(btnWolfPlus, btnGuide),
		adminMenu.Row(btnWallet, btnBuy),
		adminMenu.Row(btnSupport),
	)

	adminPanelMenu.Reply(
		adminPanelMenu.Row(btnConfigAccount, btnConfigSupport),
		adminPanelMenu.Row(btnBack),
	)

	accountConfigMenu.Reply(
		accountConfigMenu.Row(btnConfigCardNum),
		accountConfigMenu.Row(btnBackToAdminAcc),
	)

	supportConfigMenu.Reply(
		supportConfigMenu.Row(btnBackToAdminSup),
	)

	profileMenu.Reply(
		profileMenu.Row(btnTurnOnSelf, btnTurnOffSelf),
		profileMenu.Row(btnExitSelf),
		profileMenu.Row(btnBack),
	)

	wolfPlusMenu.Reply(
		wolfPlusMenu.Row(btnWP_Ghost, btnWP_AntiDelete),
		wolfPlusMenu.Row(btnWP_PVLock, btnWP_Logger),
		wolfPlusMenu.Row(btnWP_Back),
	)

	pvLockMenu.Reply(
		pvLockMenu.Row(btnPV_On, btnPV_Off),
		pvLockMenu.Row(btnPV_Back),
	)

	// منوی اصلی راهنما
	guideMenu.Reply(
		guideMenu.Row(btnGClock, btnGEmoji),
		guideMenu.Row(btnGBio, btnGFont),
		guideMenu.Row(btnGFriend, btnGEnemy),
		guideMenu.Row(btnGAction, btnGPurge),
		guideMenu.Row(btnGTimer, btnGAutoReact),
		guideMenu.Row(btnGPV, btnGGroup),
		guideMenu.Row(btnGTranslate), // دکمه مترجم در اینجا لود می‌شود
		guideMenu.Row(btnGBackMain),
	)

	guideClockMenu.Reply(guideClockMenu.Row(btnClockOn, btnClockOff), guideClockMenu.Row(btnClockBack))
	guideEmojiMenu.Reply(guideEmojiMenu.Row(btnEmojiOn, btnEmojiOff), guideEmojiMenu.Row(btnEmojiBack))
	guideBioMenu.Reply(guideBioMenu.Row(btnBioOn, btnBioOff), guideBioMenu.Row(btnBioBack))
	guideFriendMenu.Reply(guideFriendMenu.Row(btnFriendList, btnFriendClear), guideFriendMenu.Row(btnFriendBack))
	guideEnemyMenu.Reply(guideEnemyMenu.Row(btnEnemyList, btnEnemyClear), guideEnemyMenu.Row(btnEnemyBack))

	guideFontMenu.Reply(
		guideFontMenu.Row(btnFontOn, btnFontOff),
		guideFontMenu.Row(btnFontBoldItalic, btnFontBold, btnFontItalic),
		guideFontMenu.Row(btnFontUnderline, btnFontStrike, btnFontMono),
		guideFontMenu.Row(btnFontSpoiler),
		guideFontMenu.Row(btnFontBack),
	)

	guideActionMenu.Reply(guideActionMenu.Row(btnActionBack))
	guidePurgeMenu.Reply(guidePurgeMenu.Row(btnPurgeBack))
	guideTimerMenu.Reply(guideTimerMenu.Row(btnTimerBack))
	guideAutoReactMenu.Reply(guideAutoReactMenu.Row(btnAutoReactBack))
	guidePVMenu.Reply(guidePVMenu.Row(btnPVBack))
	guideGroupMenu.Reply(guideGroupMenu.Row(btnGroupBack))
	guideTranslateMenu.Reply(guideTranslateMenu.Row(btnTranslateBack)) // منوی راهنمای مترجم با دکمه بازگشت

	waitingReceiptMenu.Reply(
		waitingReceiptMenu.Row(btnCancelReceipt),
	)

	confirmSelfMenu.Reply(
		confirmSelfMenu.Row(btnConfirmSelfAction),
		confirmSelfMenu.Row(btnBack),
	)

	walletReplyMenu.Reply(
		walletReplyMenu.Row(btnWalletConfirm),
		walletReplyMenu.Row(btnBack),
	)
}

func getWalletInlineKeyboard() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	btn10k := menu.Data("10,000 تومان", "wallet_change", "10000")
	btn20k := menu.Data("20,000 تومان", "wallet_change", "20000")
	btn50k := menu.Data("50,000 تومان", "wallet_change", "50000")

	menu.Inline(
		menu.Row(btn10k, btn20k),
		menu.Row(btn50k),
	)
	return menu
}
