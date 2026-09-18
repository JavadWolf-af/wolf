package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	tele "gopkg.in/telebot.v3"
)

func updateClocks() {
	if db == nil { return }
	rows, err := db.Query("SELECT id FROM users WHERE self_status = 'روشن' AND is_clock_enabled = TRUE")
	if err != nil { return }

	var uids []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err == nil {
			uids = append(uids, uid)
		}
	}
	rows.Close()

	if len(uids) == 0 { return }

	boldTime := getTehranBoldTime()

	activeUserbotsMu.RLock()
	for _, uid := range uids {
		if ub, ok := activeUserbots[uid]; ok && ub.Client != nil {
			go func(cl *telegram.Client) {
				cTimeout, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				req := &tg.AccountUpdateProfileRequest{}
				req.SetLastName(boldTime)
				_, _ = cl.API().AccountUpdateProfile(cTimeout, req)
			}(ub.Client)
		}
	}
	activeUserbotsMu.RUnlock()
}

func updateEmojis() {
	if db == nil { return }
	rows, err := db.Query("SELECT id, original_first_name FROM users WHERE self_status = 'روشن' AND is_emoji_enabled = TRUE")
	if err != nil { return }

	type userEmojiInfo struct {
		id        int64
		origFirst string
	}
	var usersList []userEmojiInfo
	for rows.Next() {
		var u userEmojiInfo
		if err := rows.Scan(&u.id, &u.origFirst); err == nil {
			usersList = append(usersList, u)
		}
	}
	rows.Close()

	if len(usersList) == 0 { return }

	activeUserbotsMu.RLock()
	for _, u := range usersList {
		if ub, ok := activeUserbots[u.id]; ok && ub.Client != nil {
			go func(cl *telegram.Client, uid int64, orig string) {
				cTimeout, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if orig == "" {
					self, err := cl.Self(cTimeout)
					if err == nil {
						orig = cleanName(self.FirstName)
						if orig != "" {
							_, _ = db.Exec("UPDATE users SET original_first_name = ? WHERE id = ?", orig, uid)
						}
					}
				}
				if orig != "" {
					req := &tg.AccountUpdateProfileRequest{}
					req.SetFirstName(fmt.Sprintf("%s %s", orig, getRandomEmoji()))
					_, _ = cl.API().AccountUpdateProfile(cTimeout, req)
				}
			}(ub.Client, u.id, u.origFirst)
		}
	}
	activeUserbotsMu.RUnlock()
}

func updateBios() {
	if db == nil { return }
	rows, err := db.Query("SELECT id FROM users WHERE self_status = 'روشن' AND is_bio_enabled = TRUE AND bio_mode = 'random'")
	if err != nil { return }

	var uids []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err == nil {
			uids = append(uids, uid)
		}
	}
	rows.Close()

	if len(uids) == 0 { return }

	activeUserbotsMu.RLock()
	for _, uid := range uids {
		if ub, ok := activeUserbots[uid]; ok && ub.Client != nil {
			go func(cl *telegram.Client) {
				cTimeout, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				req := &tg.AccountUpdateProfileRequest{}
				req.SetAbout(getRandomBio())
				_, _ = cl.API().AccountUpdateProfile(cTimeout, req)
			}(ub.Client)
		}
	}
	activeUserbotsMu.RUnlock()
}

func startClockWorker() {
	go func() {
		now := time.Now()
		nextMinute := now.Truncate(time.Minute).Add(time.Minute)
		time.Sleep(time.Until(nextMinute))
		updateClocks()
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			updateClocks()
		}
	}()
}

func startEmojiWorker() {
	go func() {
		updateEmojis()
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			updateEmojis()
		}
	}()
}

func startBioWorker() {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		for range ticker.C {
			updateBios()
		}
	}()
}

func startBillingWorker(bot *tele.Bot) {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		for range ticker.C {
			processDailyBilling(bot)
		}
	}()
}

func processDailyBilling(bot *tele.Bot) {
	if db == nil { return }

	rows, err := db.Query(`
		SELECT id FROM users 
		WHERE self_status = 'روشن' 
		AND (last_billed_at IS NULL OR last_billed_at <= DATE_SUB(NOW(), INTERVAL 24 HOUR))
	`)
	if err != nil {
		log.Printf("❌ Billing Worker Error: %v", err)
		return
	}
	defer rows.Close()

	var userIDs []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err == nil {
			userIDs = append(userIDs, uid)
		}
	}

	keyPrice := getKeyPrice()

	for _, uid := range userIDs {
		balance := GetUserBalance(uid)
		if balance < keyPrice {
			_, _ = db.Exec("UPDATE users SET self_status = 'خاموش' WHERE id = ?", uid)
			stopUserbot(uid)
			msg := "⚠️ <b>شارژ کلیدهای شما به پایان رسید!</b>\n\nموجودی شما برای کسر هزینه روزانه سلف (۱ کلید) کافی نبود و سلف شما به صورت خودکار خاموش شد.\n\n🛒 <i>لطفاً جهت فعالسازی مجدد، از بخش کیف پول اقدام به شارژ حساب نمایید.</i>"
			_, _ = bot.Send(&tele.User{ID: uid}, msg, tele.ModeHTML)
		} else {
			_, err := db.Exec(`UPDATE wallets SET balance = balance - ? WHERE user_id = ? AND balance >= ?`, keyPrice, uid, keyPrice)
			if err == nil {
				_, _ = db.Exec("UPDATE users SET last_billed_at = NOW() WHERE id = ?", uid)
				newBalance := balance - keyPrice
				remainingKeys := newBalance / keyPrice
				msg := fmt.Sprintf(
					"🔔 <b>تمدید روزانه سلف 🐺</b>\n\n✅ ۱ کلید بابت تمدید ۲۴ ساعته سلف از موجودی شما کسر شد.\n🔑 <b>کلیدهای باقی‌مانده شما:</b> <code>%d</code> عدد",
					remainingKeys,
				)
				_, _ = bot.Send(&tele.User{ID: uid}, msg, tele.ModeHTML)
			}
		}
	}
}
