package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	gpc "github.com/yaa110/go-persian-calendar"
	tele "gopkg.in/telebot.v3"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

type Config struct {
	BotToken string
	AdminIDs []int64
	APIID    int
	APIHash  string
	DBUser   string
	DBPass   string
	DBName   string
}

var db *sql.DB
var getMainKeyboard func(userID int64) *tele.ReplyMarkup
var controllerBotID int64

var (
	stateMu        sync.RWMutex
	adminStates    = make(map[int64]AdminAction)
	userStates     = make(map[int64]*UserState)
	userWalletTemp = make(map[int64]int)

	activeUserbotsMu sync.RWMutex
	activeUserbots   = make(map[int64]*UserbotSession)

	// کش حافظه برای دوستان
	friendsCacheMu sync.RWMutex
	friendsCache   = make(map[int64]map[int64]bool)

	// کش حافظه برای دشمنان
	enemiesCacheMu sync.RWMutex
	enemiesCache   = make(map[int64]map[int64]bool)

	// کش حافظه برای ری‌اکشن خودکار
	autoReactsCacheMu sync.RWMutex
	autoReactsCache   = make(map[int64]map[int64]string)

	// کش خوشنویسی (Font Styler)
	fontSettingsMu sync.RWMutex
	fontSettings   = make(map[int64]struct {
		Enabled bool
		Mode    string
	})

	// مدیریت اکشن‌های زنده چت
	activeActionsMu sync.Mutex
	activeActions   = make(map[string]context.CancelFunc)

	channelAccessHashesMu sync.RWMutex
	channelAccessHashes   = make(map[int64]int64)
)

var randomEmojiPool = []string{
	"🐺", "👑", "⚡", "🔥", "💎", "✨", "🚀", "🪐", "🌪", "🦁",
	"🦅", "🎯", "🎲", "🖤", "🤍", "❄️", "🌙", "⭐", "💫", "🗡",
	"🛡", "🧿", "🔮", "🎭", "🌊", "🩸", "🕊", "☘️", "🥀", "🌹",
	"🍒", "☕", "🛸", "⚓", "⏳", "🗝", "⚔️", "🏎", "🐉", "🐾",
}

var randomBioPool = []string{
	"🐺 در سکوت شب، زوزه‌ی گرگ شنیدنی‌تر است.",
	"⚡️ قوی بمان، قصه‌ی تو پایان درخشانی دارد.",
	"🖤 گاهی سکوت، رساترین فریاد درونی است.",
	"👑 پادشاه قلمرو خویشتن باش، نه برده دیگران.",
	"🌙 شب‌های تاریک، نویدبخش سپیده‌دمی روشن‌اند.",
	"🗡️ با زخم‌هایت رشد کن، نه فقط با آرزوهایت.",
	"✨ در عمق تاریکی‌ها نیز می‌توان درخشید.",
	"🕊️ آزادی حقیقی، رهایی از قضاوت بی‌ارزش‌هاست.",
	"🦁 شیر در بند هم که باشد، همچنان سلطانی مغرور است.",
	"🌊 آرام مثل سطح آب، عمیق و سهمگین چون اقیانوس.",
	"🔥 از خاکسترِ شکست‌ها، ققنوسی مقتدر بساز!",
	"💎 اصالت را هیچ بهایی نمی‌تواند بسنجد.",
	"⏳ زمان می‌گذرد و حقیقت‌ها عریان‌تر می‌شوند.",
	"🎯 متمرکز بر هدف؛ صداهای مزاحم را نشنیده بگیر.",
	"🥀 از ریشه‌های خویش جوانه می‌نم؛ استوارتر از دیروز.",
	"🪐 در مدار سرنوشت خود، ستاره‌ای بی‌همتایم.",
	"☕️ تلخ اما سرشار از آرامش، چون خلوت شبانه.",
	"🌪️ طوفان‌ها برپا می‌شوند تا مسیر را هموار سازند.",
	"🧿 از چشم بد دور و در پناه روشنایی امید.",
	"🗝️ کلید پیروزی در صبری سرسختانه نهفته است.",
	"🏎️ شتابان به پیش؛ ایستادن مرگ جریان‌هاست.",
	"❄️ خونسرد چون بلور یخ، استوار چون صخره البرز.",
	"🎭 زندگی صحنه ماست و ما معمار تقدیر خویشیم.",
	"🐉 شعله‌های باور را در سینه زنده نگاه دار.",
	"🌟 رویاهایت را خلق کن، پیش از آنکه دیر شود!",
}

func getRandomEmoji() string {
	return randomEmojiPool[rand.Intn(len(randomEmojiPool))]
}

func getRandomBio() string {
	return randomBioPool[rand.Intn(len(randomBioPool))]
}

func cleanName(name string) string {
	name = strings.TrimSpace(name)
	changed := true
	for changed {
		changed = false
		for _, em := range randomEmojiPool {
			if strings.HasSuffix(name, em) {
				name = strings.TrimSpace(strings.TrimSuffix(name, em))
				changed = true
			}
		}
	}
	return strings.TrimSpace(name)
}

type UserbotSession struct {
	UserID int64
	Client *telegram.Client
	Cancel context.CancelFunc
}

type AdminAction struct {
	Action   string
	TargetID int64
}

type AuthResultType int

const (
	AuthResultSuccess AuthResultType = iota
	AuthResultNeeds2FA
	AuthResultFailed
)

type AuthResult struct {
	Type  AuthResultType
	Error error
}

type UserState struct {
	Action       string
	Phone        string
	CodeChan     chan string
	PasswordChan chan string
	ResultChan   chan AuthResult
	Cancel       context.CancelFunc
}

func loadConfig() Config {
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("❌ خطای پیکربندی: مقدار BOT_TOKEN در فایل .env یافت نشد.")
	}

	apiIDStr := os.Getenv("API_ID")
	apiID, _ := strconv.Atoi(apiIDStr)
	apiHash := os.Getenv("API_HASH")

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
		APIID:    apiID,
		APIHash:  apiHash,
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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT PRIMARY KEY,
		first_name VARCHAR(255),
		username VARCHAR(255),
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		phone VARCHAR(50) DEFAULT 'ثبت نشده',
		is_blocked BOOLEAN DEFAULT FALSE,
		self_status VARCHAR(50) DEFAULT 'خرید نداشته',
		purchases_count INT DEFAULT 0,
		last_billed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		is_clock_enabled BOOLEAN DEFAULT FALSE,
		original_last_name VARCHAR(255) DEFAULT '',
		is_emoji_enabled BOOLEAN DEFAULT FALSE,
		original_first_name VARCHAR(255) DEFAULT '',
		is_timer_media_enabled BOOLEAN DEFAULT FALSE,
		is_bio_enabled BOOLEAN DEFAULT FALSE,
		bio_mode VARCHAR(20) DEFAULT 'random',
		custom_bio VARCHAR(255) DEFAULT '',
		original_bio VARCHAR(255) DEFAULT '',
		is_font_enabled BOOLEAN DEFAULT FALSE,
		font_mode VARCHAR(30) DEFAULT 'bold_italic'
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec("ALTER TABLE users ADD COLUMN last_billed_at DATETIME DEFAULT CURRENT_TIMESTAMP")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_clock_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN original_last_name VARCHAR(255) DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_emoji_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN original_first_name VARCHAR(255) DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_timer_media_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_bio_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN bio_mode VARCHAR(20) DEFAULT 'random'")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN custom_bio VARCHAR(255) DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN original_bio VARCHAR(255) DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_font_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN font_mode VARCHAR(30) DEFAULT 'bold_italic'")

	// جدول دوستان
	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_friends (
		owner_id BIGINT,
		friend_id BIGINT,
		friend_name VARCHAR(255) DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, friend_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	// جدول دشمنان
	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_enemies (
		owner_id BIGINT,
		enemy_id BIGINT,
		enemy_name VARCHAR(255) DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, enemy_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	// جدول ری‌اکشن‌های خودکار
	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_auto_reacts (
		owner_id BIGINT,
		target_id BIGINT,
		target_name VARCHAR(255) DEFAULT '',
		emoji VARCHAR(50) DEFAULT '❤️',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, target_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wallets (
		user_id BIGINT PRIMARY KEY,
		balance INT DEFAULT 0,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS settings (
		setting_key VARCHAR(50) PRIMARY KEY,
		setting_value TEXT
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS transactions (
		id INT AUTO_INCREMENT PRIMARY KEY,
		user_id BIGINT,
		amount INT,
		status VARCHAR(50) DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_number', '6037-9971-XXXX-XXXX')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_name', 'جواد ولف')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('card_bank', 'بانک ملی')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('support_text', '🎧 <b>بخش پشتیبانی</b>\n\nجهت حل مشکلات و پاسخ به سوالات خود، با ما در ارتباط باشید:')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('support_id', '@JavadWolf')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('key_price', '3333')`)

	loadAllFriendsToCache()
	loadAllEnemiesToCache()
	loadAllAutoReactsToCache()
	loadAllFontSettingsToCache()
}

func loadAllAutoReactsToCache() {
	if db == nil {
		return
	}
	rows, err := db.Query("SELECT owner_id, target_id, emoji FROM wolf_auto_reacts")
	if err != nil {
		return
	}
	defer rows.Close()

	autoReactsCacheMu.Lock()
	autoReactsCache = make(map[int64]map[int64]string)
	for rows.Next() {
		var oID, tID int64
		var emoji string
		if err := rows.Scan(&oID, &tID, &emoji); err == nil {
			if _, exists := autoReactsCache[oID]; !exists {
				autoReactsCache[oID] = make(map[int64]string)
			}
			autoReactsCache[oID][tID] = emoji
		}
	}
	autoReactsCacheMu.Unlock()
}

func getAutoReact(ownerID, targetID int64) (string, bool) {
	autoReactsCacheMu.RLock()
	defer autoReactsCacheMu.RUnlock()
	if userSet, exists := autoReactsCache[ownerID]; exists {
		emoji, found := userSet[targetID]
		return emoji, found
	}
	return "", false
}

func setAutoReactToCache(ownerID, targetID int64, emoji string) {
	autoReactsCacheMu.Lock()
	defer autoReactsCacheMu.Unlock()
	if _, exists := autoReactsCache[ownerID]; !exists {
		autoReactsCache[ownerID] = make(map[int64]string)
	}
	autoReactsCache[ownerID][targetID] = emoji
}

func removeAutoReactFromCache(ownerID, targetID int64) {
	autoReactsCacheMu.Lock()
	defer autoReactsCacheMu.Unlock()
	if userSet, exists := autoReactsCache[ownerID]; exists {
		delete(userSet, targetID)
	}
}

func clearAutoReactsCache(ownerID int64) {
	autoReactsCacheMu.Lock()
	defer autoReactsCacheMu.Unlock()
	delete(autoReactsCache, ownerID)
}

func loadAllFriendsToCache() {
	if db == nil {
		return
	}
	rows, err := db.Query("SELECT owner_id, friend_id FROM wolf_friends")
	if err != nil {
		return
	}
	defer rows.Close()

	friendsCacheMu.Lock()
	friendsCache = make(map[int64]map[int64]bool)
	for rows.Next() {
		var oID, fID int64
		if err := rows.Scan(&oID, &fID); err == nil {
			if _, exists := friendsCache[oID]; !exists {
				friendsCache[oID] = make(map[int64]bool)
			}
			friendsCache[oID][fID] = true
		}
	}
	friendsCacheMu.Unlock()
}

func isUserFriend(ownerID, targetID int64) bool {
	friendsCacheMu.RLock()
	defer friendsCacheMu.RUnlock()
	if userSet, exists := friendsCache[ownerID]; exists {
		return userSet[targetID]
	}
	return false
}

func addFriendToCache(ownerID, friendID int64) {
	friendsCacheMu.Lock()
	defer friendsCacheMu.Unlock()
	if _, exists := friendsCache[ownerID]; !exists {
		friendsCache[ownerID] = make(map[int64]bool)
	}
	friendsCache[ownerID][friendID] = true
}

func removeFriendFromCache(ownerID, friendID int64) {
	friendsCacheMu.Lock()
	defer friendsCacheMu.Unlock()
	if userSet, exists := friendsCache[ownerID]; exists {
		delete(userSet, friendID)
	}
}

func clearFriendsCache(ownerID int64) {
	friendsCacheMu.Lock()
	defer friendsCacheMu.Unlock()
	delete(friendsCache, ownerID)
}

func loadAllEnemiesToCache() {
	if db == nil {
		return
	}
	rows, err := db.Query("SELECT owner_id, enemy_id FROM wolf_enemies")
	if err != nil {
		return
	}
	defer rows.Close()

	enemiesCacheMu.Lock()
	enemiesCache = make(map[int64]map[int64]bool)
	for rows.Next() {
		var oID, eID int64
		if err := rows.Scan(&oID, &eID); err == nil {
			if _, exists := enemiesCache[oID]; !exists {
				enemiesCache[oID] = make(map[int64]bool)
			}
			enemiesCache[oID][eID] = true
		}
	}
	enemiesCacheMu.Unlock()
}

func isUserEnemy(ownerID, targetID int64) bool {
	enemiesCacheMu.RLock()
	defer enemiesCacheMu.RUnlock()
	if userSet, exists := enemiesCache[ownerID]; exists {
		return userSet[targetID]
	}
	return false
}

func addEnemyToCache(ownerID, enemyID int64) {
	enemiesCacheMu.Lock()
	defer enemiesCacheMu.Unlock()
	if _, exists := enemiesCache[ownerID]; !exists {
		enemiesCache[ownerID] = make(map[int64]bool)
	}
	enemiesCache[ownerID][enemyID] = true
}

func removeEnemyFromCache(ownerID, enemyID int64) {
	enemiesCacheMu.Lock()
	defer enemiesCacheMu.Unlock()
	if userSet, exists := enemiesCache[ownerID]; exists {
		delete(userSet, enemyID)
	}
}

func clearEnemiesCache(ownerID int64) {
	enemiesCacheMu.Lock()
	defer enemiesCacheMu.Unlock()
	delete(enemiesCache, ownerID)
}

func loadAllFontSettingsToCache() {
	if db == nil {
		return
	}
	rows, err := db.Query("SELECT id, is_font_enabled, font_mode FROM users")
	if err != nil {
		return
	}
	defer rows.Close()

	fontSettingsMu.Lock()
	fontSettings = make(map[int64]struct {
		Enabled bool
		Mode    string
	})
	for rows.Next() {
		var uid int64
		var en bool
		var mode string
		if err := rows.Scan(&uid, &en, &mode); err == nil {
			if mode == "" {
				mode = "bold_italic"
			}
			fontSettings[uid] = struct {
				Enabled bool
				Mode    string
			}{Enabled: en, Mode: mode}
		}
	}
	fontSettingsMu.Unlock()
}

func updateFontCache(uid int64, en bool, mode string) {
	fontSettingsMu.Lock()
	defer fontSettingsMu.Unlock()
	fontSettings[uid] = struct {
		Enabled bool
		Mode    string
	}{Enabled: en, Mode: mode}
}

func getFontSetting(uid int64) (bool, string) {
	fontSettingsMu.RLock()
	defer fontSettingsMu.RUnlock()
	if s, ok := fontSettings[uid]; ok {
		return s.Enabled, s.Mode
	}
	return false, "bold_italic"
}

func PopulateChannelCache(chats []tg.ChatClass) {
	channelAccessHashesMu.Lock()
	defer channelAccessHashesMu.Unlock()
	for _, cClass := range chats {
		if ch, ok := cClass.(*tg.Channel); ok {
			channelAccessHashes[ch.ID] = ch.AccessHash
		}
	}
}

func stopActiveAction(actionKey string) {
	activeActionsMu.Lock()
	if cancel, ok := activeActions[actionKey]; ok {
		cancel()
		delete(activeActions, actionKey)
	}
	activeActionsMu.Unlock()
}

func startFakeAction(ctx context.Context, client *telegram.Client, userID int64, inputPeer tg.InputPeerClass, peerKey string, action tg.SendMessageActionClass, durationSec int) {
	actionKey := fmt.Sprintf("%d_%s", userID, peerKey)
	stopActiveAction(actionKey)

	if durationSec <= 0 {
		durationSec = 20
	}
	if durationSec > 300 {
		durationSec = 300
	}

	actCtx, cancel := context.WithTimeout(context.Background(), time.Duration(durationSec)*time.Second)
	activeActionsMu.Lock()
	activeActions[actionKey] = cancel
	activeActionsMu.Unlock()

	go func() {
		defer func() {
			activeActionsMu.Lock()
			delete(activeActions, actionKey)
			activeActionsMu.Unlock()
			cancel()
		}()

		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()

		_, _ = client.API().MessagesSetTyping(actCtx, &tg.MessagesSetTypingRequest{
			Peer:   inputPeer,
			Action: action,
		})

		for {
			select {
			case <-actCtx.Done():
				cCtx, cCancel := context.WithTimeout(context.Background(), 3*time.Second)
				_, _ = client.API().MessagesSetTyping(cCtx, &tg.MessagesSetTypingRequest{
					Peer:   inputPeer,
					Action: &tg.SendMessageCancelAction{},
				})
				cCancel()
				return
			case <-ticker.C:
				_, err := client.API().MessagesSetTyping(actCtx, &tg.MessagesSetTypingRequest{
					Peer:   inputPeer,
					Action: action,
				})
				if err != nil {
					return
				}
			}
		}
	}()
}

func deleteMessageBatch(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, ids []int) {
	if len(ids) == 0 {
		return
	}
	dCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if ch, ok := inputPeer.(*tg.InputPeerChannel); ok {
		_, _ = client.API().ChannelsDeleteMessages(dCtx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{
				ChannelID:  ch.ChannelID,
				AccessHash: ch.AccessHash,
			},
			ID: ids,
		})
		return
	}
	_, _ = client.API().MessagesDeleteMessages(dCtx, &tg.MessagesDeleteMessagesRequest{
		Revoke: true,
		ID:     ids,
	})
}

func sendTemporaryNotice(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, text string, duration time.Duration) {
	sendReq := &tg.MessagesSendMessageRequest{
		Peer:     inputPeer,
		Message:  text,
		RandomID: rand.Int63(),
		Entities: []tg.MessageEntityClass{
			&tg.MessageEntityBold{Offset: 0, Length: len([]rune(text))},
		},
	}
	res, err := client.API().MessagesSendMessage(ctx, sendReq)
	if err != nil {
		return
	}

	msgID := 0
	if updates, ok := res.(*tg.Updates); ok {
		for _, u := range updates.Updates {
			if nu, ok := u.(*tg.UpdateNewMessage); ok {
				if m, ok := nu.Message.(*tg.Message); ok {
					msgID = m.ID
					break
				}
			} else if ncu, ok := u.(*tg.UpdateNewChannelMessage); ok {
				if m, ok := ncu.Message.(*tg.Message); ok {
					msgID = m.ID
					break
				}
			}
		}
	}

	if msgID != 0 {
		time.Sleep(duration)
		deleteMsg(context.Background(), client, inputPeer, msgID)
	}
}

func handlePurgeAction(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, cmdMsgID int, fromReplyID int, countLimit int) {
	pCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var toDelete []int
	offsetID := cmdMsgID

	for {
		req := &tg.MessagesGetHistoryRequest{
			Peer:     inputPeer,
			OffsetID: offsetID,
			Limit:    100,
		}
		res, err := client.API().MessagesGetHistory(pCtx, req)
		if err != nil {
			break
		}

		var messages []tg.MessageClass
		switch h := res.(type) {
		case *tg.MessagesMessages:
			messages = h.Messages
		case *tg.MessagesMessagesSlice:
			messages = h.Messages
		case *tg.MessagesChannelMessages:
			messages = h.Messages
		}

		if len(messages) == 0 {
			break
		}

		stopSearch := false
		for _, mClass := range messages {
			m, ok := mClass.(*tg.Message)
			if !ok {
				continue
			}

			if fromReplyID > 0 {
				if m.ID < fromReplyID {
					stopSearch = true
					break
				}
				if m.Out {
					toDelete = append(toDelete, m.ID)
				}
				if m.ID == fromReplyID {
					stopSearch = true
					break
				}
			} else {
				if m.Out {
					toDelete = append(toDelete, m.ID)
					if len(toDelete) >= countLimit {
						stopSearch = true
						break
					}
				}
			}
		}

		if stopSearch || len(messages) < 100 {
			break
		}

		if lastMsg, ok := messages[len(messages)-1].(*tg.Message); ok {
			offsetID = lastMsg.ID
		} else {
			break
		}
	}

	chunkSize := 100
	for i := 0; i < len(toDelete); i += chunkSize {
		end := i + chunkSize
		if end > len(toDelete) {
			end = len(toDelete)
		}
		deleteMessageBatch(pCtx, client, inputPeer, toDelete[i:end])
		time.Sleep(100 * time.Millisecond)
	}

	totalDeleted := len(toDelete)
	reportText := fmt.Sprintf("🗑 %d پیام شما با موفقیت پاکسازی شد", totalDeleted)
	if totalDeleted == 0 {
		reportText = "⚠️ پیامی برای پاکسازی یافت نشد"
	}
	sendTemporaryNotice(pCtx, client, inputPeer, reportText, 1500*time.Millisecond)
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
	_, _ = db.Exec(`INSERT INTO users (id, first_name, username) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE first_name=?, username=?`, userID, firstName, username, firstName, username)
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

func IsUserBlocked(userID int64) bool {
	var blocked bool
	err := db.QueryRow("SELECT is_blocked FROM users WHERE id = ?", userID).Scan(&blocked)
	if err != nil {
		return false
	}
	return blocked
}

func GetUserSelfStatus(userID int64) string {
	var status string
	err := db.QueryRow("SELECT self_status FROM users WHERE id = ?", userID).Scan(&status)
	if err != nil {
		return "خرید نداشته"
	}
	return status
}

func SafeAddUserBalance(userID int64, amount int) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("❌ DB Begin Error: %v", err)
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT IGNORE INTO users (id, first_name, username) VALUES (?, 'کاربر', 'ثبت_نشده')`, userID)
	if err != nil {
		log.Printf("❌ DB Insert User Error: %v", err)
		return err
	}

	_, err = tx.Exec(`INSERT INTO wallets (user_id, balance) VALUES (?, ?) ON DUPLICATE KEY UPDATE balance = balance + ?`, userID, amount, amount)
	if err != nil {
		log.Printf("❌ DB Wallet Update Error: %v", err)
		return err
	}

	_, err = tx.Exec(`UPDATE users SET purchases_count = purchases_count + 1 WHERE id = ?`, userID)
	if err != nil {
		log.Printf("❌ DB Purchases Count Error: %v", err)
		return err
	}

	return tx.Commit()
}

func toBoldDigits(t string) string {
	boldDigits := map[rune]string{
		'0': "𝟎", '1': "𝟏", '2': "𝟐", '3': "𝟑", '4': "𝟒",
		'5': "𝟓", '6': "𝟔", '7': "𝟕", '8': "𝟖", '9': "𝟗",
		':': ":",
	}
	var sb strings.Builder
	for _, r := range t {
		if b, ok := boldDigits[r]; ok {
			sb.WriteString(b)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func getTehranBoldTime() string {
	loc := getTehranLocation()
	now := time.Now().In(loc)
	return toBoldDigits(now.Format("15:04"))
}

func handleClockOn(ctx context.Context, userID int64, client *telegram.Client) {
	var isEnabled bool
	var origLast string
	_ = db.QueryRow("SELECT is_clock_enabled, original_last_name FROM users WHERE id = ?", userID).Scan(&isEnabled, &origLast)

	if !isEnabled || origLast == "" {
		self, err := client.Self(ctx)
		if err == nil {
			origLast = self.LastName
			_, _ = db.Exec("UPDATE users SET original_last_name = ? WHERE id = ?", origLast, userID)
		}
	}

	boldTime := getTehranBoldTime()
	req := &tg.AccountUpdateProfileRequest{}
	req.SetLastName(boldTime)
	_, err := client.API().AccountUpdateProfile(ctx, req)
	if err == nil {
		_, _ = db.Exec("UPDATE users SET is_clock_enabled = TRUE WHERE id = ?", userID)
	}
}

func handleClockOff(ctx context.Context, userID int64, client *telegram.Client) {
	var origLastName string
	_ = db.QueryRow("SELECT original_last_name FROM users WHERE id = ?", userID).Scan(&origLastName)

	req := &tg.AccountUpdateProfileRequest{}
	req.SetLastName(origLastName)
	_, _ = client.API().AccountUpdateProfile(ctx, req)

	_, _ = db.Exec("UPDATE users SET is_clock_enabled = FALSE WHERE id = ?", userID)
}

func handleEmojiOn(ctx context.Context, userID int64, client *telegram.Client) {
	var isEnabled bool
	var origFirst string
	_ = db.QueryRow("SELECT is_emoji_enabled, original_first_name FROM users WHERE id = ?", userID).Scan(&isEnabled, &origFirst)

	if !isEnabled || origFirst == "" {
		self, err := client.Self(ctx)
		if err == nil {
			origFirst = cleanName(self.FirstName)
			_, _ = db.Exec("UPDATE users SET original_first_name = ? WHERE id = ?", origFirst, userID)
		}
	}

	emoji := getRandomEmoji()
	newName := fmt.Sprintf("%s %s", origFirst, emoji)
	req := &tg.AccountUpdateProfileRequest{}
	req.SetFirstName(newName)
	_, err := client.API().AccountUpdateProfile(ctx, req)
	if err == nil {
		_, _ = db.Exec("UPDATE users SET is_emoji_enabled = TRUE WHERE id = ?", userID)
	}
}

func handleEmojiOff(ctx context.Context, userID int64, client *telegram.Client) {
	var origFirst string
	_ = db.QueryRow("SELECT original_first_name FROM users WHERE id = ?", userID).Scan(&origFirst)

	if origFirst != "" {
		req := &tg.AccountUpdateProfileRequest{}
		req.SetFirstName(origFirst)
		_, _ = client.API().AccountUpdateProfile(ctx, req)
	}

	_, _ = db.Exec("UPDATE users SET is_emoji_enabled = FALSE WHERE id = ?", userID)
}

func handleBioOn(ctx context.Context, userID int64, client *telegram.Client) {
	var isBioEnabled bool
	var origBio string
	_ = db.QueryRow("SELECT is_bio_enabled, original_bio FROM users WHERE id = ?", userID).Scan(&isBioEnabled, &origBio)

	if !isBioEnabled || origBio == "" {
		full, err := client.API().UsersGetFullUser(ctx, &tg.InputUserSelf{})
		if err == nil {
			origBio = full.FullUser.About
			_, _ = db.Exec("UPDATE users SET original_bio = ? WHERE id = ?", origBio, userID)
		}
	}

	bio := getRandomBio()
	req := &tg.AccountUpdateProfileRequest{}
	req.SetAbout(bio)
	_, err := client.API().AccountUpdateProfile(ctx, req)
	if err == nil {
		_, _ = db.Exec("UPDATE users SET is_bio_enabled = TRUE, bio_mode = 'random' WHERE id = ?", userID)
	}
}

func handleBioOff(ctx context.Context, userID int64, client *telegram.Client) {
	var origBio string
	_ = db.QueryRow("SELECT original_bio FROM users WHERE id = ?", userID).Scan(&origBio)

	req := &tg.AccountUpdateProfileRequest{}
	req.SetAbout(origBio)
	_, _ = client.API().AccountUpdateProfile(ctx, req)

	_, _ = db.Exec("UPDATE users SET is_bio_enabled = FALSE WHERE id = ?", userID)
}

func handleBioRandom(ctx context.Context, userID int64, client *telegram.Client) {
	bio := getRandomBio()
	req := &tg.AccountUpdateProfileRequest{}
	req.SetAbout(bio)
	_, err := client.API().AccountUpdateProfile(ctx, req)
	if err == nil {
		_, _ = db.Exec("UPDATE users SET is_bio_enabled = TRUE, bio_mode = 'random' WHERE id = ?", userID)
	}
}

func handleBioCustom(ctx context.Context, userID int64, client *telegram.Client, customBio string) {
	var origBio string
	_ = db.QueryRow("SELECT original_bio FROM users WHERE id = ?", userID).Scan(&origBio)
	if origBio == "" {
		full, err := client.API().UsersGetFullUser(ctx, &tg.InputUserSelf{})
		if err == nil {
			origBio = full.FullUser.About
			_, _ = db.Exec("UPDATE users SET original_bio = ? WHERE id = ?", origBio, userID)
		}
	}

	runes := []rune(customBio)
	if len(runes) > 70 {
		customBio = string(runes[:70])
	}

	req := &tg.AccountUpdateProfileRequest{}
	req.SetAbout(customBio)
	_, err := client.API().AccountUpdateProfile(ctx, req)
	if err == nil {
		_, _ = db.Exec("UPDATE users SET is_bio_enabled = TRUE, bio_mode = 'custom', custom_bio = ? WHERE id = ?", customBio, userID)
	}
}

func getInputPeer(peer tg.PeerClass, e tg.Entities, selfID int64) tg.InputPeerClass {
	if peer == nil {
		return nil
	}
	switch p := peer.(type) {
	case *tg.PeerUser:
		if p.UserID == selfID {
			return &tg.InputPeerSelf{}
		}
		if u, ok := e.Users[p.UserID]; ok {
			return &tg.InputPeerUser{
				UserID:     u.ID,
				AccessHash: u.AccessHash,
			}
		}
		return &tg.InputPeerUser{UserID: p.UserID}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}
	case *tg.PeerChannel:
		var aHash int64
		if ch, ok := e.Channels[p.ChannelID]; ok {
			aHash = ch.AccessHash
			channelAccessHashesMu.Lock()
			channelAccessHashes[ch.ID] = ch.AccessHash
			channelAccessHashesMu.Unlock()
		} else {
			channelAccessHashesMu.RLock()
			aHash = channelAccessHashes[p.ChannelID]
			channelAccessHashesMu.RUnlock()
		}
		return &tg.InputPeerChannel{
			ChannelID:  p.ChannelID,
			AccessHash: aHash,
		}
	}
	return nil
}

func getRepliedMessageAndUsers(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msgID int) (*tg.Message, []tg.UserClass, error) {
	var res tg.MessagesMessagesClass
	var err error

	if ch, ok := inputPeer.(*tg.InputPeerChannel); ok {
		res, err = client.API().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash},
			ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}},
		})
	} else {
		res, err = client.API().MessagesGetMessages(ctx, []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}})
	}

	if err != nil {
		return nil, nil, err
	}

	switch m := res.(type) {
	case *tg.MessagesChannelMessages:
		if len(m.Messages) > 0 {
			if msg, ok := m.Messages[0].(*tg.Message); ok {
				return msg, m.Users, nil
			}
		}
	case *tg.MessagesMessages:
		if len(m.Messages) > 0 {
			if msg, ok := m.Messages[0].(*tg.Message); ok {
				return msg, m.Users, nil
			}
		}
	case *tg.MessagesMessagesSlice:
		if len(m.Messages) > 0 {
			if msg, ok := m.Messages[0].(*tg.Message); ok {
				return msg, m.Users, nil
			}
		}
	}
	return nil, nil, errors.New("message not found")
}

func deleteMsg(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msgID int) {
	dCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	if ch, ok := inputPeer.(*tg.InputPeerChannel); ok {
		_, _ = client.API().ChannelsDeleteMessages(dCtx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{
				ChannelID:  ch.ChannelID,
				AccessHash: ch.AccessHash,
			},
			ID: []int{msgID},
		})
		return
	}
	_, _ = client.API().MessagesDeleteMessages(dCtx, &tg.MessagesDeleteMessagesRequest{
		Revoke: true,
		ID:     []int{msgID},
	})
}

func notifyAndSelfDestruct(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msgID int, text string) {
	editReq := &tg.MessagesEditMessageRequest{
		Peer:    inputPeer,
		ID:      msgID,
		Message: text,
		Entities: []tg.MessageEntityClass{
			&tg.MessageEntityBold{
				Offset: 0,
				Length: len([]rune(text)),
			},
		},
	}
	_, _ = client.API().MessagesEditMessage(ctx, editReq)

	time.Sleep(100 * time.Millisecond)
	deleteMsg(context.Background(), client, inputPeer, msgID)
}

func getEntitiesForFont(text string, mode string) []tg.MessageEntityClass {
	length := len([]rune(text))
	switch mode {
	case "bold":
		return []tg.MessageEntityClass{&tg.MessageEntityBold{Offset: 0, Length: length}}
	case "italic":
		return []tg.MessageEntityClass{&tg.MessageEntityItalic{Offset: 0, Length: length}}
	case "bold_italic":
		return []tg.MessageEntityClass{
			&tg.MessageEntityBold{Offset: 0, Length: length},
			&tg.MessageEntityItalic{Offset: 0, Length: length},
		}
	case "underline":
		return []tg.MessageEntityClass{&tg.MessageEntityUnderline{Offset: 0, Length: length}}
	case "strike":
		return []tg.MessageEntityClass{&tg.MessageEntityStrike{Offset: 0, Length: length}}
	case "mono":
		return []tg.MessageEntityClass{&tg.MessageEntityCode{Offset: 0, Length: length}}
	case "spoiler":
		return []tg.MessageEntityClass{&tg.MessageEntitySpoiler{Offset: 0, Length: length}}
	default:
		return []tg.MessageEntityClass{
			&tg.MessageEntityBold{Offset: 0, Length: length},
			&tg.MessageEntityItalic{Offset: 0, Length: length},
		}
	}
}

func handleForwardToAllPV(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, dropAuthor bool) {
	if msg.ReplyTo == nil {
		if inputPeer != nil {
			notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است")
		}
		return
	}

	header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || header.ReplyToMsgID == 0 {
		if inputPeer != nil {
			notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است")
		}
		return
	}
	replyMsgID := header.ReplyToMsgID

	dialogsReq := &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
	}
	res, err := client.API().MessagesGetDialogs(ctx, dialogsReq)
	if err != nil {
		return
	}

	var users []tg.UserClass
	var dialogs []tg.DialogClass
	switch d := res.(type) {
	case *tg.MessagesDialogs:
		users = d.Users
		dialogs = d.Dialogs
	case *tg.MessagesDialogsSlice:
		users = d.Users
		dialogs = d.Dialogs
	}

	userMap := make(map[int64]*tg.User)
	for _, uClass := range users {
		if u, ok := uClass.(*tg.User); ok {
			userMap[u.ID] = u
		}
	}

	for _, dlg := range dialogs {
		d, ok := dlg.(*tg.Dialog)
		if !ok {
			continue
		}
		peerUser, ok := d.Peer.(*tg.PeerUser)
		if !ok {
			continue
		}
		u, exists := userMap[peerUser.UserID]
		if !exists || u.Bot || u.Self || u.Deleted {
			continue
		}

		targetPeer := &tg.InputPeerUser{
			UserID:     u.ID,
			AccessHash: u.AccessHash,
		}

		fwdReq := &tg.MessagesForwardMessagesRequest{
			DropAuthor: dropAuthor,
			FromPeer:   inputPeer,
			ID:         []int{replyMsgID},
			RandomID:   []int64{rand.Int63()},
			ToPeer:     targetPeer,
		}
		_, _ = client.API().MessagesForwardMessages(ctx, fwdReq)
		time.Sleep(80 * time.Millisecond)
	}

	if inputPeer != nil {
		notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "انجام شد")
	}
}

func handleForwardToAllGroups(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, dropAuthor bool) {
	if msg.ReplyTo == nil {
		if inputPeer != nil {
			notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است")
		}
		return
	}

	header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || header.ReplyToMsgID == 0 {
		if inputPeer != nil {
			notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است")
		}
		return
	}
	replyMsgID := header.ReplyToMsgID

	dialogsReq := &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
	}
	res, err := client.API().MessagesGetDialogs(ctx, dialogsReq)
	if err != nil {
		return
	}

	var chats []tg.ChatClass
	var dialogs []tg.DialogClass
	switch d := res.(type) {
	case *tg.MessagesDialogs:
		chats = d.Chats
		dialogs = d.Dialogs
	case *tg.MessagesDialogsSlice:
		chats = d.Chats
		dialogs = d.Dialogs
	}

	chatMap := make(map[int64]tg.ChatClass)
	for _, cClass := range chats {
		switch ch := cClass.(type) {
		case *tg.Chat:
			chatMap[ch.ID] = ch
		case *tg.Channel:
			chatMap[ch.ID] = ch
		}
	}

	for _, dlg := range dialogs {
		d, ok := dlg.(*tg.Dialog)
		if !ok {
			continue
		}

		var targetPeer tg.InputPeerClass
		switch p := d.Peer.(type) {
		case *tg.PeerChat:
			targetPeer = &tg.InputPeerChat{ChatID: p.ChatID}
		case *tg.PeerChannel:
			if chObj, exists := chatMap[p.ChannelID]; exists {
				if ch, ok := chObj.(*tg.Channel); ok && !ch.Broadcast {
					targetPeer = &tg.InputPeerChannel{
						ChannelID:  ch.ID,
						AccessHash: ch.AccessHash,
					}
				}
			}
		}

		if targetPeer == nil {
			continue
		}

		fwdReq := &tg.MessagesForwardMessagesRequest{
			DropAuthor: dropAuthor,
			FromPeer:   inputPeer,
			ID:         []int{replyMsgID},
			RandomID:   []int64{rand.Int63()},
			ToPeer:     targetPeer,
		}
		_, _ = client.API().MessagesForwardMessages(ctx, fwdReq)
		time.Sleep(80 * time.Millisecond)
	}

	if inputPeer != nil {
		notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "انجام شد")
	}
}

func startUserbot(userID int64, cfg Config, bot *tele.Bot) {
	activeUserbotsMu.Lock()
	if _, exists := activeUserbots[userID]; exists {
		activeUserbotsMu.Unlock()
		return
	}

	sessionPath := filepath.Join("/opt/wolf/sessions", fmt.Sprintf("user_%d.json", userID))
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		activeUserbotsMu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	loader := &session.FileStorage{Path: sessionPath}
	dispatcher := tg.NewUpdateDispatcher()

	client := telegram.NewClient(cfg.APIID, cfg.APIHash, telegram.Options{
		SessionStorage: loader,
		UpdateHandler:  dispatcher,
		Device: telegram.DeviceConfig{
			DeviceModel:    "PC 64bit",
			SystemVersion:  "Windows 11",
			AppVersion:     "5.4.1 x64",
			LangCode:       "en",
			SystemLangCode: "en",
		},
	})

	activeUserbots[userID] = &UserbotSession{
		UserID: userID,
		Client: client,
		Cancel: cancel,
	}
	activeUserbotsMu.Unlock()

	RegisterWolfPlusDispatcher(&dispatcher, client, userID)

	handleMsg := func(ctx context.Context, e tg.Entities, message tg.MessageClass) {
		msg, ok := message.(*tg.Message)
		if !ok {
			return
		}

		for cid, ch := range e.Channels {
			channelAccessHashesMu.Lock()
			channelAccessHashes[cid] = ch.AccessHash
			channelAccessHashesMu.Unlock()
		}

		WolfPlusHandleIncoming(ctx, client, bot, userID, msg, e)

		var inputPeer tg.InputPeerClass
		self, err := client.Self(ctx)
		selfID := int64(0)
		if err == nil {
			selfID = self.ID
		}
		inputPeer = getInputPeer(msg.PeerID, e, selfID)

		peerKey := "chat"
		switch p := msg.PeerID.(type) {
		case *tg.PeerUser:
			peerKey = fmt.Sprintf("user_%d", p.UserID)
		case *tg.PeerChat:
			peerKey = fmt.Sprintf("chat_%d", p.ChatID)
		case *tg.PeerChannel:
			peerKey = fmt.Sprintf("channel_%d", p.ChannelID)
		}

		// واکنش به پیام‌های دیگران
		if !msg.Out {
			senderID := int64(0)
			if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
				senderID = fromUser.UserID
			} else if peerUser, ok := msg.PeerID.(*tg.PeerUser); ok {
				senderID = peerUser.UserID
			}

			if senderID != 0 {
				// 1. سیستم ری‌اکشن خودکار
				if emoji, exists := getAutoReact(userID, senderID); exists && inputPeer != nil {
					go func(msgID int, p tg.InputPeerClass, em string) {
						time.Sleep(200 * time.Millisecond) // تاخیر طبیعی
						rCtx, rCancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer rCancel()

						_, _ = client.API().MessagesSendReaction(rCtx, &tg.MessagesSendReactionRequest{
							Peer:  p,
							MsgID: msgID,
							Reaction: []tg.ReactionClass{
								&tg.ReactionEmoji{Emoticon: em},
							},
						})
					}(msg.ID, inputPeer, emoji)
				}

				// 2. سیستم دوست
				if isUserFriend(userID, senderID) {
					go func(msgID int, p tg.InputPeerClass) {
						time.Sleep(150 * time.Millisecond)
						replyText := GetRandomFriendMessage()
						rCtx, rCancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer rCancel()

						req := &tg.MessagesSendMessageRequest{
							Peer:     p,
							Message:  replyText,
							RandomID: rand.Int63(),
						}
						req.SetReplyTo(&tg.InputReplyToMessage{
							ReplyToMsgID: msgID,
						})
						_, _ = client.API().MessagesSendMessage(rCtx, req)
					}(msg.ID, inputPeer)
				}

				// 3. سیستم دشمن
				if isUserEnemy(userID, senderID) {
					go func(msgID int, p tg.InputPeerClass) {
						time.Sleep(150 * time.Millisecond)
						replyText := GetRandomEnemyMessage()
						rCtx, rCancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer rCancel()

						req := &tg.MessagesSendMessageRequest{
							Peer:     p,
							Message:  replyText,
							RandomID: rand.Int63(),
						}
						req.SetReplyTo(&tg.InputReplyToMessage{
							ReplyToMsgID: msgID,
						})
						_, _ = client.API().MessagesSendMessage(rCtx, req)
					}(msg.ID, inputPeer)
				}
			}
			return
		}

		text := strings.TrimSpace(msg.Message)

		// پردازش دستورات ری‌اکشن خودکار
		if text == "ری‌اکشن" || strings.HasPrefix(text, "ری‌اکشن ") {
			if msg.ReplyTo == nil {
				if inputPeer != nil {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!")
				}
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 {
				return
			}

			emoji := "❤️"
			if strings.HasPrefix(text, "ری‌اکشن ") {
				em := strings.TrimSpace(strings.TrimPrefix(text, "ری‌اکشن "))
				if em != "" {
					emoji = em
				}
			}

			go func(repID int, p tg.InputPeerClass, mID int, selectedEmoji string) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				repMsg, usersList, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil {
					go notifyAndSelfDestruct(dCtx, client, p, mID, "❌ پیام یافت نشد!")
					return
				}

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok {
					targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok {
					targetUID = peerU.UserID
				}

				if targetUID == 0 || targetUID == selfID {
					return
				}

				targetUName := ""
				for _, uClass := range usersList {
					if u, ok := uClass.(*tg.User); ok && u.ID == targetUID {
						targetUName = formatTelegramUser(u)
						break
					}
				}
				if targetUName == "" {
					targetUName = fmt.Sprintf("کاربر (%d)", targetUID)
				}

				_, _ = db.Exec(`
					INSERT INTO wolf_auto_reacts (owner_id, target_id, target_name, emoji)
					VALUES (?, ?, ?, ?)
					ON DUPLICATE KEY UPDATE target_name = VALUES(target_name), emoji = VALUES(emoji)
				`, userID, targetUID, targetUName, selectedEmoji)

				setAutoReactToCache(userID, targetUID, selectedEmoji)
				go notifyAndSelfDestruct(dCtx, client, p, mID, fmt.Sprintf("✅ ری‌اکشن %s فعال شد", selectedEmoji))
			}(header.ReplyToMsgID, inputPeer, msg.ID, emoji)
			return

		} else if text == "حذف ری‌اکشن" {
			if msg.ReplyTo == nil {
				if inputPeer != nil {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!")
				}
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 {
				return
			}

			go func(repID int, p tg.InputPeerClass, mID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil {
					return
				}

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok {
					targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok {
					targetUID = peerU.UserID
				}

				if targetUID == 0 {
					return
				}

				_, _ = db.Exec("DELETE FROM wolf_auto_reacts WHERE owner_id = ? AND target_id = ?", userID, targetUID)
				removeAutoReactFromCache(userID, targetUID)
				go notifyAndSelfDestruct(dCtx, client, p, mID, "✅ ری‌اکشن این فرد لغو شد")
			}(header.ReplyToMsgID, inputPeer, msg.ID)
			return

		} else if text == "لیست ری‌اکشن" {
			go func(mID int, p tg.InputPeerClass) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				go notifyAndSelfDestruct(dCtx, client, p, mID, "📋 لیست ری‌اکشن‌ها ارسال شد")

				rows, err := db.Query("SELECT target_id, target_name, emoji FROM wolf_auto_reacts WHERE owner_id = ?", userID)
				if err != nil {
					return
				}
				defer rows.Close()

				var list []string
				idx := 1
				for rows.Next() {
					var tid int64
					var tname, emoji string
					if err := rows.Scan(&tid, &tname, &emoji); err == nil {
						list = append(list, fmt.Sprintf("%d. %s (<code>%d</code>) - اموجی: %s", idx, tname, tid, emoji))
						idx++
					}
				}

				msgText := "🔥 <b>لیست ری‌اکشن‌های خودکار شما:</b>\n\n" + strings.Join(list, "\n")
				if len(list) == 0 {
					msgText = "⚠️ <i>لیست ری‌اکشن‌های خودکار شما خالی است!</i>"
				}

				_, _ = client.API().MessagesSendMessage(dCtx, &tg.MessagesSendMessageRequest{
					Peer:     &tg.InputPeerSelf{},
					Message:  msgText,
					RandomID: rand.Int63(),
				})
			}(msg.ID, inputPeer)
			return

		} else if text == "پاکسازی ری‌اکشن" {
			go func(mID int, p tg.InputPeerClass) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				go notifyAndSelfDestruct(dCtx, client, p, mID, "🗑 لیست ری‌اکشن پاکسازی شد")

				_, _ = db.Exec("DELETE FROM wolf_auto_reacts WHERE owner_id = ?", userID)
				clearAutoReactsCache(userID)

				_, _ = client.API().MessagesSendMessage(dCtx, &tg.MessagesSendMessageRequest{
					Peer:     &tg.InputPeerSelf{},
					Message:  "🗑 <b>لیست ری‌اکشن‌های خودکار شما به طور کامل پاکسازی شد 🔥</b>",
					RandomID: rand.Int63(),
				})
			}(msg.ID, inputPeer)
			return
		}

		// پردازش دستور شمارش معکوس (تایمر زنده)
		if strings.HasPrefix(text, "تایمر ") || strings.HasPrefix(text, "شمارش ") {
			prefix := "تایمر "
			if strings.HasPrefix(text, "شمارش ") {
				prefix = "شمارش "
			}
			numStr := strings.TrimSpace(strings.TrimPrefix(text, prefix))
			count, err := strconv.Atoi(numStr)
			if err == nil && count > 0 {
				if count > 60 {
					count = 60
				}
				go func(p tg.InputPeerClass, mID int, startCount int) {
					for i := startCount; i > 0; i-- {
						eCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						msgText := fmt.Sprintf("⏳ %d", i)
						_, _ = client.API().MessagesEditMessage(eCtx, &tg.MessagesEditMessageRequest{
							Peer:    p,
							ID:      mID,
							Message: msgText,
							Entities: []tg.MessageEntityClass{
								&tg.MessageEntityBold{Offset: 0, Length: len([]rune(msgText))},
							},
						})
						cancel()
						time.Sleep(1 * time.Second)
					}
					eCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					_, _ = client.API().MessagesEditMessage(eCtx, &tg.MessagesEditMessageRequest{
						Peer:    p,
						ID:      mID,
						Message: "💥",
					})
					cancel()
				}(inputPeer, msg.ID, count)
				return
			}
		}

		// پردازش دستورات پاکسازی سریع پیام‌ها با دستور «پاکشو»
		if text == "پاکشو" || strings.HasPrefix(text, "پاکشو ") {
			if msg.ReplyTo != nil {
				header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
				if ok && header.ReplyToMsgID != 0 {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🗑 در حال پاکسازی پیام‌ها...")
					go handlePurgeAction(ctx, client, inputPeer, msg.ID, header.ReplyToMsgID, 0)
					return
				}
			}

			numStr := strings.TrimSpace(strings.TrimPrefix(text, "پاکشو"))
			if numStr != "" {
				count, err := strconv.Atoi(numStr)
				if err == nil && count > 0 {
					if count > 100 {
						count = 100
					}
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, fmt.Sprintf("🗑 در حال حذف %d پیام اخیر...", count))
					go handlePurgeAction(ctx, client, inputPeer, msg.ID, 0, count)
					return
				}
			}
		}

		// پردازش دستورات اکشن‌های جعلی
		if text == "لغو اکشن" || text == "توقف اکشن" {
			actionKey := fmt.Sprintf("%d_%s", userID, peerKey)
			stopActiveAction(actionKey)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🛑 اکشن متوقف شد")
			}
			return
		}

		parseActionDuration := func(cmdText, prefix string) int {
			rem := strings.TrimSpace(strings.TrimPrefix(cmdText, prefix))
			if rem == "" {
				return 20
			}
			d, err := strconv.Atoi(rem)
			if err != nil || d <= 0 {
				return 20
			}
			return d
		}

		var actionToRun tg.SendMessageActionClass
		actionDuration := 20
		isActionCmd := false

		if strings.HasPrefix(text, "اکشن تایپ") || strings.HasPrefix(text, "تایپینگ") {
			isActionCmd = true
			actionToRun = &tg.SendMessageTypingAction{}
			if strings.HasPrefix(text, "اکشن تایپ") {
				actionDuration = parseActionDuration(text, "اکشن تایپ")
			} else {
				actionDuration = parseActionDuration(text, "تایپینگ")
			}
		} else if strings.HasPrefix(text, "اکشن وویس") || strings.HasPrefix(text, "ضبط صدا") {
			isActionCmd = true
			actionToRun = &tg.SendMessageRecordAudioAction{}
			if strings.HasPrefix(text, "اکشن وویس") {
				actionDuration = parseActionDuration(text, "اکشن وویس")
			} else {
				actionDuration = parseActionDuration(text, "ضبط صدا")
			}
		} else if strings.HasPrefix(text, "اکشن ویدیوگرد") || strings.HasPrefix(text, "ویدیو گرد") {
			isActionCmd = true
			actionToRun = &tg.SendMessageRecordRoundAction{}
			if strings.HasPrefix(text, "اکشن ویدیوگرد") {
				actionDuration = parseActionDuration(text, "اکشن ویدیوگرد")
			} else {
				actionDuration = parseActionDuration(text, "ویدیو گرد")
			}
		} else if strings.HasPrefix(text, "اکشن ویدیو") || strings.HasPrefix(text, "ضبط ویدیو") {
			isActionCmd = true
			actionToRun = &tg.SendMessageRecordVideoAction{}
			if strings.HasPrefix(text, "اکشن ویدیو") {
				actionDuration = parseActionDuration(text, "اکشن ویدیو")
			} else {
				actionDuration = parseActionDuration(text, "ضبط ویدیو")
			}
		} else if strings.HasPrefix(text, "اکشن عکس") || strings.HasPrefix(text, "ارسال عکس") {
			isActionCmd = true
			actionToRun = &tg.SendMessageUploadPhotoAction{}
			if strings.HasPrefix(text, "اکشن عکس") {
				actionDuration = parseActionDuration(text, "اکشن عکس")
			} else {
				actionDuration = parseActionDuration(text, "ارسال عکس")
			}
		} else if strings.HasPrefix(text, "اکشن فایل") || strings.HasPrefix(text, "ارسال فایل") {
			isActionCmd = true
			actionToRun = &tg.SendMessageUploadDocumentAction{}
			if strings.HasPrefix(text, "اکشن فایل") {
				actionDuration = parseActionDuration(text, "اکشن فایل")
			} else {
				actionDuration = parseActionDuration(text, "ارسال فایل")
			}
		} else if strings.HasPrefix(text, "اکشن بازی") || strings.HasPrefix(text, "بازی") {
			isActionCmd = true
			actionToRun = &tg.SendMessageGamePlayAction{}
			if strings.HasPrefix(text, "اکشن بازی") {
				actionDuration = parseActionDuration(text, "اکشن بازی")
			} else {
				actionDuration = parseActionDuration(text, "بازی")
			}
		}

		if isActionCmd && actionToRun != nil {
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, fmt.Sprintf("✅ اکشن به مدت %d ثانیه فعال شد", actionDuration))
				go startFakeAction(ctx, client, userID, inputPeer, peerKey, actionToRun, actionDuration)
			}
			return
		}

		// دستورات چت سیستم دوست
		if text == "تنظیم دوست" {
			if msg.ReplyTo == nil {
				if inputPeer != nil {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!")
				}
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 {
				return
			}
			replyMsgID := header.ReplyToMsgID

			go func(repID int, p tg.InputPeerClass, mID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				go notifyAndSelfDestruct(dCtx, client, p, mID, "✅ تنظیم دوست شد")

				repMsg, usersList, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil {
					return
				}

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok {
					targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok {
					targetUID = peerU.UserID
				}

				if targetUID == 0 || targetUID == selfID {
					return
				}

				targetUName := ""
				for _, uClass := range usersList {
					if u, ok := uClass.(*tg.User); ok && u.ID == targetUID {
						targetUName = formatTelegramUser(u)
						break
					}
				}
				if targetUName == "" {
					targetUName = fmt.Sprintf("کاربر (%d)", targetUID)
				}

				_, _ = db.Exec(`
					INSERT INTO wolf_friends (owner_id, friend_id, friend_name)
					VALUES (?, ?, ?)
					ON DUPLICATE KEY UPDATE friend_name = VALUES(friend_name)
				`, userID, targetUID, targetUName)

				addFriendToCache(userID, targetUID)

				replyText := GetRandomFriendMessage()
				sendReq := &tg.MessagesSendMessageRequest{
					Peer:     p,
					Message:  replyText,
					RandomID: rand.Int63(),
				}
				sendReq.SetReplyTo(&tg.InputReplyToMessage{
					ReplyToMsgID: repID,
				})
				_, _ = client.API().MessagesSendMessage(dCtx, sendReq)
			}(replyMsgID, inputPeer, msg.ID)
			return

		} else if text == "حذف دوست" {
			if msg.ReplyTo == nil {
				if inputPeer != nil {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!")
				}
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 {
				return
			}
			replyMsgID := header.ReplyToMsgID

			go func(repID int, p tg.InputPeerClass, mID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				go notifyAndSelfDestruct(dCtx, client, p, mID, "✅ حذف دوست شد")

				repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil {
					return
				}

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok {
					targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok {
					targetUID = peerU.UserID
				}

				if targetUID == 0 {
					return
				}

				_, _ = db.Exec("DELETE FROM wolf_friends WHERE owner_id = ? AND friend_id = ?", userID, targetUID)
				removeFriendFromCache(userID, targetUID)
			}(replyMsgID, inputPeer, msg.ID)
			return

		} else if text == "لیست دوست" {
			go func(mID int, p tg.InputPeerClass) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				go notifyAndSelfDestruct(dCtx, client, p, mID, "📋 لیست دوست ارسال شد")

				rows, err := db.Query("SELECT friend_id, friend_name FROM wolf_friends WHERE owner_id = ?", userID)
				if err != nil {
					return
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

				msgText := "📋 <b>لیست دوستان شما:</b>\n\n" + strings.Join(list, "\n")
				if len(list) == 0 {
					msgText = "⚠️ <i>لیست دوستان شما در حال حاضر خالی است!</i>"
				}

				_, _ = client.API().MessagesSendMessage(dCtx, &tg.MessagesSendMessageRequest{
					Peer:     &tg.InputPeerSelf{},
					Message:  msgText,
					RandomID: rand.Int63(),
				})
			}(msg.ID, inputPeer)
			return

		} else if text == "پاکسازی دوست" {
			go func(mID int, p tg.InputPeerClass) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				go notifyAndSelfDestruct(dCtx, client, p, mID, "🗑 پاکسازی دوست شد")

				_, _ = db.Exec("DELETE FROM wolf_friends WHERE owner_id = ?", userID)
				clearFriendsCache(userID)

				_, _ = client.API().MessagesSendMessage(dCtx, &tg.MessagesSendMessageRequest{
					Peer:     &tg.InputPeerSelf{},
					Message:  "🗑 <b>لیست دوستان شما به طور کامل پاکسازی شد 🌸</b>",
					RandomID: rand.Int63(),
				})
			}(msg.ID, inputPeer)
			return
		}

		// دستورات چت سیستم دشمن
		if text == "تنظیم دشمن" {
			if msg.ReplyTo == nil {
				if inputPeer != nil {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!")
				}
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 {
				return
			}
			replyMsgID := header.ReplyToMsgID

			go func(repID int, p tg.InputPeerClass, mID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				go notifyAndSelfDestruct(dCtx, client, p, mID, "⚔️ تنظیم دشمن شد")

				repMsg, usersList, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil {
					return
				}

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok {
					targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok {
					targetUID = peerU.UserID
				}

				if targetUID == 0 || targetUID == selfID {
					return
				}

				targetUName := ""
				for _, uClass := range usersList {
					if u, ok := uClass.(*tg.User); ok && u.ID == targetUID {
						targetUName = formatTelegramUser(u)
						break
					}
				}
				if targetUName == "" {
					targetUName = fmt.Sprintf("کاربر (%d)", targetUID)
				}

				_, _ = db.Exec(`
					INSERT INTO wolf_enemies (owner_id, enemy_id, enemy_name)
					VALUES (?, ?, ?)
					ON DUPLICATE KEY UPDATE enemy_name = VALUES(enemy_name)
				`, userID, targetUID, targetUName)

				addEnemyToCache(userID, targetUID)

				replyText := GetRandomEnemyMessage()
				sendReq := &tg.MessagesSendMessageRequest{
					Peer:     p,
					Message:  replyText,
					RandomID: rand.Int63(),
				}
				sendReq.SetReplyTo(&tg.InputReplyToMessage{
					ReplyToMsgID: repID,
				})
				_, _ = client.API().MessagesSendMessage(dCtx, sendReq)
			}(replyMsgID, inputPeer, msg.ID)
			return

		} else if text == "حذف دشمن" {
			if msg.ReplyTo == nil {
				if inputPeer != nil {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!")
				}
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 {
				return
			}
			replyMsgID := header.ReplyToMsgID

			go func(repID int, p tg.InputPeerClass, mID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				go notifyAndSelfDestruct(dCtx, client, p, mID, "✅ حذف دشمن شد")

				repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil {
					return
				}

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok {
					targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok {
					targetUID = peerU.UserID
				}

				if targetUID == 0 {
					return
				}

				_, _ = db.Exec("DELETE FROM wolf_enemies WHERE owner_id = ? AND enemy_id = ?", userID, targetUID)
				removeEnemyFromCache(userID, targetUID)
			}(replyMsgID, inputPeer, msg.ID)
			return

		} else if text == "لیست دشمن" {
			go func(mID int, p tg.InputPeerClass) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				go notifyAndSelfDestruct(dCtx, client, p, mID, "📋 لیست دشمن ارسال شد")

				rows, err := db.Query("SELECT enemy_id, enemy_name FROM wolf_enemies WHERE owner_id = ?", userID)
				if err != nil {
					return
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

				msgText := "⚔️ <b>لیست دشمنان شما:</b>\n\n" + strings.Join(list, "\n")
				if len(list) == 0 {
					msgText = "⚠️ <i>لیست دشمنان شما در حال حاضر خالی است!</i>"
				}

				_, _ = client.API().MessagesSendMessage(dCtx, &tg.MessagesSendMessageRequest{
					Peer:     &tg.InputPeerSelf{},
					Message:  msgText,
					RandomID: rand.Int63(),
				})
			}(msg.ID, inputPeer)
			return

		} else if text == "پاکسازی دشمن" {
			go func(mID int, p tg.InputPeerClass) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				go notifyAndSelfDestruct(dCtx, client, p, mID, "🗑 پاکسازی دشمن شد")

				_, _ = db.Exec("DELETE FROM wolf_enemies WHERE owner_id = ?", userID)
				clearEnemiesCache(userID)

				_, _ = client.API().MessagesSendMessage(dCtx, &tg.MessagesSendMessageRequest{
					Peer:     &tg.InputPeerSelf{},
					Message:  "🗑 <b>لیست دشمنان شما به طور کامل پاکسازی شد ⚔️</b>",
					RandomID: rand.Int63(),
				})
			}(msg.ID, inputPeer)
			return
		}

		// دستورات چت سیستم خوشنویسی
		if text == "خوشنویسی روشن" {
			_, _ = db.Exec("UPDATE users SET is_font_enabled = TRUE WHERE id = ?", userID)
			_, mode := getFontSetting(userID)
			updateFontCache(userID, true, mode)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🟢 خوشنویسی روشن شد")
			}
			return
		} else if text == "خوشنویسی خاموش" {
			_, _ = db.Exec("UPDATE users SET is_font_enabled = FALSE WHERE id = ?", userID)
			_, mode := getFontSetting(userID)
			updateFontCache(userID, false, mode)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🔴 خوشنویسی خاموش شد")
			}
			return
		} else if strings.HasPrefix(text, "خوشنویسی ") {
			cleanSub := strings.TrimSpace(strings.TrimPrefix(text, "خوشنویسی "))
			mode := ""
			switch cleanSub {
			case "بولد":
				mode = "bold"
			case "ایتالیک":
				mode = "italic"
			case "بولد ایتالیک":
				mode = "bold_italic"
			case "زیر خط":
				mode = "underline"
			case "خط خورده":
				mode = "strike"
			case "مونو":
				mode = "mono"
			case "اسپویل":
				mode = "spoiler"
			}

			if mode != "" {
				_, _ = db.Exec("UPDATE users SET font_mode = ?, is_font_enabled = TRUE WHERE id = ?", mode, userID)
				en, _ := getFontSetting(userID)
				updateFontCache(userID, en, mode)
				if inputPeer != nil {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, fmt.Sprintf("✅ فونت به %s تغییر یافت", text))
				}
				return
			}
		}

		if text == "دانلود" || text == "سیو" {
			if msg.ReplyTo != nil {
				if header, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && header.ReplyToMsgID != 0 {
					go func() {
						HandleProtectedDownloadByReply(ctx, client, inputPeer, header.ReplyToMsgID, userID)
						if inputPeer != nil {
							notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "محتوا به پیام‌های ذخیره‌شده ارسال شد")
						}
					}()
					return
				}
			}
		}

		if text == "سین" || text == "سین بزن" {
			if inputPeer != nil {
				go func() {
					HandleGhostMarkAsRead(ctx, client, inputPeer, msg.ID)
					notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "سین زده شد")
				}()
				return
			}
		}

		if text == "ساعت روشن شو" || text == "ساعت روشن" {
			handleClockOn(ctx, userID, client)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "ساعت روشن شد")
			}
			return
		} else if text == "ساعت خاموش شو" || text == "ساعت خاموش" {
			handleClockOff(ctx, userID, client)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "ساعت خاموش شد")
			}
			return
		} else if text == "اموجی روشن شو" || text == "اموجی روشن" {
			handleEmojiOn(ctx, userID, client)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "اموجی روشن شد")
			}
			return
		} else if text == "اموجی خاموش شو" || text == "اموجی خاموش" {
			handleEmojiOff(ctx, userID, client)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "اموجی خاموش شد")
			}
			return
		} else if text == "بیو روشن شو" || text == "بیو روشن" {
			handleBioOn(ctx, userID, client)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "بیو روشن شد")
			}
			return
		} else if text == "بیو خاموش شو" || text == "بیو خاموش" {
			handleBioOff(ctx, userID, client)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "بیو خاموش شد")
			}
			return
		} else if text == "رندوم شو" {
			handleBioRandom(ctx, userID, client)
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "بیو به حالت رندوم تغییر یافت")
			}
			return
		} else if text == "بیو شو" {
			if msg.ReplyTo == nil {
				if inputPeer != nil {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی یک پیام ریپلای کنید!")
				}
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 {
				if inputPeer != nil {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ پیام معتبر نیست!")
				}
				return
			}

			go func(replyID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer dCancel()

				res, err := client.API().MessagesGetMessages(dCtx, []tg.InputMessageClass{&tg.InputMessageID{ID: replyID}})
				if err != nil {
					if inputPeer != nil {
						notifyAndSelfDestruct(dCtx, client, inputPeer, msg.ID, "❌ خطا در خواندن پیام!")
					}
					return
				}

				var targetText string
				switch mSlice := res.(type) {
				case *tg.MessagesMessages:
					if len(mSlice.Messages) > 0 {
						if m, ok := mSlice.Messages[0].(*tg.Message); ok {
							targetText = m.Message
						}
					}
				case *tg.MessagesMessagesSlice:
					if len(mSlice.Messages) > 0 {
						if m, ok := mSlice.Messages[0].(*tg.Message); ok {
							targetText = m.Message
						}
					}
				case *tg.MessagesChannelMessages:
					if len(mSlice.Messages) > 0 {
						if m, ok := mSlice.Messages[0].(*tg.Message); ok {
							targetText = m.Message
						}
					}
				}

				targetText = strings.TrimSpace(targetText)
				if targetText == "" {
					if inputPeer != nil {
						notifyAndSelfDestruct(dCtx, client, inputPeer, msg.ID, "⚠️ پیام متنی یافت نشد!")
					}
					return
				}

				handleBioCustom(dCtx, userID, client, targetText)
				if inputPeer != nil {
					notifyAndSelfDestruct(dCtx, client, inputPeer, msg.ID, "بیو با موفقیت تنظیم شد")
				}
			}(header.ReplyToMsgID)
			return

		} else if text == "بفرست پیوی همه" {
			go func() {
				bCtx, bCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer bCancel()
				handleForwardToAllPV(bCtx, client, inputPeer, msg, true)
			}()
			return
		} else if text == "پیوی همه" {
			go func() {
				bCtx, bCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer bCancel()
				handleForwardToAllPV(bCtx, client, inputPeer, msg, false)
			}()
			return
		} else if text == "بفرست گروه همه" {
			go func() {
				gCtx, gCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer gCancel()
				handleForwardToAllGroups(gCtx, client, inputPeer, msg, true)
			}()
			return
		} else if text == "گروه همه" {
			go func() {
				gCtx, gCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer gCancel()
				handleForwardToAllGroups(gCtx, client, inputPeer, msg, false)
			}()
			return
		}

		// اعمال بلادرنگ فونت و خوشنویسی روی تمامی پیام‌های ارسالی کاربر
		fontEnabled, fontMode := getFontSetting(userID)
		if fontEnabled && text != "" && msg.Media == nil {
			go func(p tg.InputPeerClass, mID int, origText string, fMode string) {
				eCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				entities := getEntitiesForFont(origText, fMode)
				_, _ = client.API().MessagesEditMessage(eCtx, &tg.MessagesEditMessageRequest{
					Peer:     p,
					ID:      mID,
					Message:  origText,
					Entities: entities,
				})
			}(inputPeer, msg.ID, text, fontMode)
		}
	}

	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		handleMsg(ctx, e, u.Message)
		return nil
	})
	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		handleMsg(ctx, e, u.Message)
		return nil
	})

	go func() {
		err := client.Run(ctx, func(ctx context.Context) error {
			go func() {
				time.Sleep(1 * time.Second)
				cInit, cancelInit := context.WithTimeout(ctx, 15*time.Second)
				defer cancelInit()
				InitUserbotPeerCache(cInit, client)
			}()

			go func() {
				time.Sleep(2 * time.Second)
				StartTargetTrackerWorker(ctx, client, userID)
			}()

			<-ctx.Done()
			return ctx.Err()
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("⚠️ Userbot %d error: %v", userID, err)
		}
		activeUserbotsMu.Lock()
		delete(activeUserbots, userID)
		activeUserbotsMu.Unlock()
	}()
}

func stopUserbot(userID int64) {
	activeUserbotsMu.Lock()
	defer activeUserbotsMu.Unlock()
	if ub, exists := activeUserbots[userID]; exists {
		if ub.Cancel != nil {
			ub.Cancel()
		}
		delete(activeUserbots, userID)
	}
}

func updateClocks() {
	if db == nil {
		return
	}
	rows, err := db.Query("SELECT id FROM users WHERE self_status = 'روشن' AND is_clock_enabled = TRUE")
	if err != nil {
		return
	}

	var uids []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err == nil {
			uids = append(uids, uid)
		}
	}
	rows.Close()

	if len(uids) == 0 {
		return
	}

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
	if db == nil {
		return
	}
	rows, err := db.Query("SELECT id, original_first_name FROM users WHERE self_status = 'روشن' AND is_emoji_enabled = TRUE")
	if err != nil {
		return
	}

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

	if len(usersList) == 0 {
		return
	}

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
	if db == nil {
		return
	}
	rows, err := db.Query("SELECT id FROM users WHERE self_status = 'روشن' AND is_bio_enabled = TRUE AND bio_mode = 'random'")
	if err != nil {
		return
	}

	var uids []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err == nil {
			uids = append(uids, uid)
		}
	}
	rows.Close()

	if len(uids) == 0 {
		return
	}

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
	if db == nil {
		return
	}

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
			msg := "⚠️ <b>شارژ کلیدهای شما به پایان رسید!</b>\n\n" +
				"موجودی شما برای کسر هزینه روزانه سلف (۱ کلید) کافی نبود و سلف شما به صورت خودکار خاموش شد.\n\n" +
				"🛒 <i>لطفاً جهت فعالسازی مجدد، از بخش کیف پول اقدام به شارژ حساب نمایید.</i>"
			_, _ = bot.Send(&tele.User{ID: uid}, msg, tele.ModeHTML)
		} else {
			_, err := db.Exec(`UPDATE wallets SET balance = balance - ? WHERE user_id = ? AND balance >= ?`, keyPrice, uid, keyPrice)
			if err == nil {
				_, _ = db.Exec("UPDATE users SET last_billed_at = NOW() WHERE id = ?", uid)
				newBalance := balance - keyPrice
				remainingKeys := newBalance / keyPrice
				msg := fmt.Sprintf(
					"🔔 <b>تمدید روزانه سلف 🐺</b>\n\n"+
						"✅ ۱ کلید بابت تمدید ۲۴ ساعته سلف از موجودی شما کسر شد.\n"+
						"🔑 <b>کلیدهای باقی‌مانده شما:</b> <code>%d</code> عدد",
					remainingKeys,
				)
				_, _ = bot.Send(&tele.User{ID: uid}, msg, tele.ModeHTML)
			}
		}
	}
}

type botAuthenticator struct {
	phone        string
	codeChan     chan string
	passwordChan chan string
	resultChan   chan AuthResult
}

func (b *botAuthenticator) Phone(ctx context.Context) (string, error) {
	return b.phone, nil
}

func (b *botAuthenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	select {
	case code, ok := <-b.codeChan:
		if !ok {
			return "", errors.New("auth canceled")
		}
		return code, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (b *botAuthenticator) Password(ctx context.Context) (string, error) {
	b.resultChan <- AuthResult{Type: AuthResultNeeds2FA}
	select {
	case pwd, ok := <-b.passwordChan:
		if !ok {
			return "", errors.New("auth canceled")
		}
		return pwd, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (b *botAuthenticator) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	return nil
}

func (b *botAuthenticator) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, errors.New("ثبت‌نام حساب جدید پشتیبانی نمی‌شود")
}

func startTelegramLogin(ctx context.Context, userID int64, cfg Config, authHandler *botAuthenticator) {
	sessionDir := "/opt/wolf/sessions"
	_ = os.MkdirAll(sessionDir, 0700)
	sessionPath := filepath.Join(sessionDir, fmt.Sprintf("user_%d.json", userID))

	loader := &session.FileStorage{Path: sessionPath}

	client := telegram.NewClient(cfg.APIID, cfg.APIHash, telegram.Options{
		SessionStorage: loader,
		Device: telegram.DeviceConfig{
			DeviceModel:    "PC 64bit",
			SystemVersion:  "Windows 11",
			AppVersion:     "5.4.1 x64",
			LangCode:       "en",
			SystemLangCode: "en",
		},
	})

	flow := auth.NewFlow(authHandler, auth.SendCodeOptions{})

	err := client.Run(ctx, func(ctx context.Context) error {
		return client.Auth().IfNecessary(ctx, flow)
	})

	if err != nil {
		authHandler.resultChan <- AuthResult{Type: AuthResultFailed, Error: err}
	} else {
		authHandler.resultChan <- AuthResult{Type: AuthResultSuccess}
	}
}

func toPersianDigits(s string) string {
	persianDigits := []string{"۰", "۱", "۲", "۳", "۴", "۵", "۶", "۷", "۸", "۹"}
	for i, d := range persianDigits {
		s = strings.ReplaceAll(s, strconv.Itoa(i), d)
	}
	return s
}

func extractDigits(s string) string {
	persianDigits := map[rune]rune{
		'۰': '0', '۱': '1', '۲': '2', '۳': '3', '۴': '4',
		'۵': '5', '۶': '6', '۷': '7', '۸': '8', '۹': '9',
	}
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			if en, ok := persianDigits[r]; ok {
				sb.WriteRune(en)
			} else {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
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

func getTehranLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		return time.FixedZone("Asia/Tehran", 12600)
	}
	return loc
}

func main() {
	cfg := loadConfig()

	if parts := strings.Split(cfg.BotToken, ":"); len(parts) > 0 {
		controllerBotID, _ = strconv.ParseInt(parts[0], 10, 64)
	}

	InitDB(cfg)
	defer db.Close()

	InitWolfPlusDB()

	pref := tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatalf("❌ خطا در راه‌اندازی ربات: %v", err)
	}

	userMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	adminMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	adminPanelMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	accountConfigMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	supportConfigMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	profileMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	confirmSelfMenu := &tele.ReplyMarkup{ResizeKeyboard: true}

	walletReplyMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	waitingReceiptMenu := &tele.ReplyMarkup{ResizeKeyboard: true}

	btnBuy := userMenu.Text("🛍️ خرید سلف")
	btnProfile := userMenu.Text("👤 حساب کاربری")
	btnWallet := userMenu.Text("👛 کیف پول 💳")
	btnWolfPlus := userMenu.Text("🐺 ولف +")
	btnSupport := userMenu.Text("🎧 پشتیبانی")
	btnGuide := userMenu.Text("📚 راهنما")
	btnAdminPanel := adminMenu.Text("⚙️ مدیریت")
	btnBack := adminMenu.Text("🔙 بازگشت")

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

	guideMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guideClockMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guideEmojiMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guideBioMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guideFriendMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guideEnemyMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guideFontMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guideActionMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guidePurgeMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guideTimerMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guideAutoReactMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guidePVMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	guideGroupMenu := &tele.ReplyMarkup{ResizeKeyboard: true}

	btnGClock := guideMenu.Text("⏱ ساعت زنده")
	btnGEmoji := guideMenu.Text("🎭 اموجی رندوم")
	btnGBio := guideMenu.Text("📝 بیوگرافی هوشمند")
	btnGFont := guideMenu.Text("✒️ خوشنویسی")
	btnGFriend := guideMenu.Text("🌸 دوست")
	btnGEnemy := guideMenu.Text("⚔️ دشمن")
	btnGAction := guideMenu.Text("🎬 اکشن‌ها")
	btnGPurge := guideMenu.Text("🗑 پاکسازی")
	btnGTimer := guideMenu.Text("⏳ تایمر")
	btnGAutoReact := guideMenu.Text("🔥 ری‌اکشن خودکار")
	btnGPV := guideMenu.Text("📩 پیوی همه")
	btnGGroup := guideMenu.Text("👥 گروه همه")
	btnGBackMain := guideMenu.Text("🔙 بازگشت به منوی اصلی")

	// چیدمان تمیز و متقارن کیبورد راهنما
	guideMenu.Reply(
		guideMenu.Row(btnGClock, btnGEmoji),
		guideMenu.Row(btnGBio, btnGFont),
		guideMenu.Row(btnGFriend, btnGEnemy),
		guideMenu.Row(btnGAction, btnGPurge),
		guideMenu.Row(btnGTimer, btnGAutoReact),
		guideMenu.Row(btnGPV, btnGGroup),
		guideMenu.Row(btnGBackMain),
	)

	btnClockOn := guideClockMenu.Text("🟢 روشن کردن ساعت")
	btnClockOff := guideClockMenu.Text("🔴 خاموش کردن ساعت")
	btnClockBack := guideClockMenu.Text("🔙 بازگشت به راهنما")
	guideClockMenu.Reply(
		guideClockMenu.Row(btnClockOn, btnClockOff),
		guideClockMenu.Row(btnClockBack),
	)

	btnEmojiOn := guideEmojiMenu.Text("🟢 روشن کردن اموجی")
	btnEmojiOff := guideEmojiMenu.Text("🔴 خاموش کردن اموجی")
	btnEmojiBack := guideEmojiMenu.Text("🔙 بازگشت به راهنما")
	guideEmojiMenu.Reply(
		guideEmojiMenu.Row(btnEmojiOn, btnEmojiOff),
		guideEmojiMenu.Row(btnEmojiBack),
	)

	btnBioOn := guideBioMenu.Text("🟢 روشن کردن بیو (رندوم)")
	btnBioOff := guideBioMenu.Text("🔴 خاموش کردن بیو")
	btnBioBack := guideBioMenu.Text("🔙 بازگشت به راهنما")
	guideBioMenu.Reply(
		guideBioMenu.Row(btnBioOn, btnBioOff),
		guideBioMenu.Row(btnBioBack),
	)

	btnFriendList := guideFriendMenu.Text("📋 لیست دوستان")
	btnFriendClear := guideFriendMenu.Text("🗑 پاکسازی دوستان")
	btnFriendBack := guideFriendMenu.Text("🔙 بازگشت به راهنما")
	guideFriendMenu.Reply(
		guideFriendMenu.Row(btnFriendList, btnFriendClear),
		guideFriendMenu.Row(btnFriendBack),
	)

	btnEnemyList := guideEnemyMenu.Text("📋 لیست دشمنان")
	btnEnemyClear := guideEnemyMenu.Text("🗑 پاکسازی دشمنان")
	btnEnemyBack := guideEnemyMenu.Text("🔙 بازگشت به راهنما")
	guideEnemyMenu.Reply(
		guideEnemyMenu.Row(btnEnemyList, btnEnemyClear),
		guideEnemyMenu.Row(btnEnemyBack),
	)

	btnFontOn := guideFontMenu.Text("🟢 روشن کردن خوشنویسی")
	btnFontOff := guideFontMenu.Text("🔴 خاموش کردن خوشنویسی")
	btnFontBoldItalic := guideFontMenu.Text("✨ بولد ایتالیک (پیش‌فرض)")
	btnFontBold := guideFontMenu.Text("🖋 بولد")
	btnFontItalic := guideFontMenu.Text("🖊 ایتالیک")
	btnFontUnderline := guideFontMenu.Text("📜 زیر خط")
	btnFontStrike := guideFontMenu.Text("❌ خط خورده")
	btnFontMono := guideFontMenu.Text("💻 مونو")
	btnFontSpoiler := guideFontMenu.Text("🕵️ اسپویل")
	btnFontBack := guideFontMenu.Text("🔙 بازگشت به راهنما")

	guideFontMenu.Reply(
		guideFontMenu.Row(btnFontOn, btnFontOff),
		guideFontMenu.Row(btnFontBoldItalic),
		guideFontMenu.Row(btnFontBold, btnFontItalic),
		guideFontMenu.Row(btnFontUnderline, btnFontStrike),
		guideFontMenu.Row(btnFontMono, btnFontSpoiler),
		guideFontMenu.Row(btnFontBack),
	)

	btnActionBack := guideActionMenu.Text("🔙 بازگشت به راهنما")
	guideActionMenu.Reply(guideActionMenu.Row(btnActionBack))

	btnPurgeBack := guidePurgeMenu.Text("🔙 بازگشت به راهنما")
	guidePurgeMenu.Reply(guidePurgeMenu.Row(btnPurgeBack))

	btnTimerBack := guideTimerMenu.Text("🔙 بازگشت به راهنما")
	guideTimerMenu.Reply(guideTimerMenu.Row(btnTimerBack))

	btnAutoReactBack := guideAutoReactMenu.Text("🔙 بازگشت به راهنما")
	guideAutoReactMenu.Reply(guideAutoReactMenu.Row(btnAutoReactBack))

	btnPVBack := guidePVMenu.Text("🔙 بازگشت به راهنما")
	guidePVMenu.Reply(guidePVMenu.Row(btnPVBack))

	btnGroupBack := guideGroupMenu.Text("🔙 بازگشت به راهنما")
	guideGroupMenu.Reply(guideGroupMenu.Row(btnGroupBack))

	buildGuideDashboardText := func(userID int64) string {
		var isClock, isEmoji, isBio, isFont bool
		var bioMode string
		// اصلاح نام ستون از bioMode به bio_mode
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

		return fmt.Sprintf(`📚 <b>بخش راهنما و امکانات سلف ولف 🐺</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>وضعیت لحظه‌ای قابلیت‌ها:</b>
▫️ ⏱ <b>ساعت زنده:</b> %s
▫️ 🎭 <b>اموجی رندوم:</b> %s
▫️ 📝 <b>بیوگرافی هوشمند:</b> %s
▫️ ✒️ <b>خوشنویسی پیام‌ها:</b> %s
▫️ 🌸 <b>سیستم دوست:</b> <code>%d نفر</code> (همیشه فعال)
▫️ ⚔️ <b>سیستم دشمن:</b> <code>%d نفر</code> (همیشه فعال)
▫️ 🎬 <b>اکشن‌های جعلی:</b> فعال و آماده
▫️ 🗑 <b>پاکسازی پیام‌ها:</b> فعال و آماده
▫️ ⏳ <b>تایمر زنده:</b> فعال و آماده
▫️ 🔥 <b>ری‌اکشن خودکار:</b> فعال و آماده
➖➖➖➖➖➖➖➖➖➖
💡 <i>جهت مطالعه راهنما و تنظیم هر قابلیت، از کیبورد ثابت زیر گزینه مورد نظر را انتخاب کنید:</i>`,
			clockStatus, emojiStatus, bioStatus, fontStatus, friendCount, enemyCount,
		)
	}

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

	btnCancelReceipt := waitingReceiptMenu.Text("🔙 لغو و بازگشت به منوی اصلی")
	waitingReceiptMenu.Reply(
		waitingReceiptMenu.Row(btnCancelReceipt),
	)

	getKeyboard := func(userID int64) *tele.ReplyMarkup {
		if cfg.IsAdmin(userID) {
			return adminMenu
		}
		return userMenu
	}
	getMainKeyboard = getKeyboard

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

		stateMu.Lock()
		delete(adminStates, userID)
		if uState, exists := userStates[userID]; exists {
			if uState.Cancel != nil {
				uState.Cancel()
			}
			delete(userStates, userID)
		}
		userWalletTemp[userID] = 0
		stateMu.Unlock()

		return c.Send("🔙 <b>به منوی اصلی بازگشتید.</b>", getKeyboard(userID), tele.ModeHTML)
	})

	backToAdminHandler := func(c tele.Context) error {
		stateMu.Lock()
		delete(adminStates, c.Sender().ID)
		stateMu.Unlock()
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

		err := db.QueryRow("SELECT joined_at, self_status FROM users WHERE id = ?", user.ID).Scan(&joinedAt, &selfStatus)
		if err != nil || joinedAt.IsZero() {
			joinedAt = time.Now()
		}

		loc := getTehranLocation()
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

		tNowStr := toPersianDigits(tNow.Format("yyyy/MM/dd"))
		tTimeStr := toPersianDigits(tNow.Format("HH:mm:ss"))
		tJoinedStr := toPersianDigits(tJoined.Format("yyyy/MM/dd"))

		text := fmt.Sprintf("💙 تاریخ امروز: %s\n\n⏰ ساعت: %s\n\n🔒 اطلاعات حساب کاربری\n\n⭐ آیدی عددی: <code>%d</code>\n📅 تاریخ عضویت در ربات: %s\n👀 فعالیت در ربات: %d روز\n💰 موجودی: %s تومان\n🔥 وضعیت سلف: %s %s",
			tNowStr, tTimeStr, user.ID, tJoinedStr, daysActive, formatMoney(GetUserBalance(user.ID)), statusIcon, selfStatus)

		return c.Send(text, profileMenu, tele.ModeHTML)
	})

	bot.Handle(&btnWolfPlus, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Send("❌ <b>دسترسی محدود!</b>\n\nامکانات ویژه ولف + فقط برای کاربرانی که اشتراک سلف را فعال دارند در دسترس است.", getKeyboard(userID), tele.ModeHTML)
		}

		return c.Send(buildWolfPlusDashboardText(userID), wolfPlusMenu, tele.ModeHTML)
	})

	RegisterWolfPlusHandlers(bot)

	// ثبت هندلرهای بخش راهنما
	bot.Handle(&btnGuide, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Send("❌ <b>دسترسی محدود!</b>\n\nبخش راهنما فقط برای کاربرانی که اشتراک سلف را خریداری کرده‌اند فعال می‌باشد.", getKeyboard(userID), tele.ModeHTML)
		}

		return c.Send(buildGuideDashboardText(userID), guideMenu, tele.ModeHTML)
	})

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
📖 <b>توضیحات:</b>
با فعال‌سازی این قابلیت، ساعت رسمی تهران به صورت زنده و با فونت بولد شکیل روی نام خانوادگی (Last Name) اکانت شما قرار می‌گیرد و هر دقیقه تغییر می‌کند.

💬 <b>دستورات چت:</b>
▫️ روشن کردن: <code>ساعت روشن شو</code> یا <code>ساعت روشن</code>
▫️ خاموش کردن: <code>ساعت خاموش شو</code> یا <code>ساعت خاموش</code>

👇 همچنین می‌توانید مستقیماً از کلیدهای زیر جهت کنترل استفاده کنید:`, statusStr)
		return c.Send(text, guideClockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnClockOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ <b>سلف شما آنلاین نیست!</b> لطفاً ابتدا از بخش پروفایل سلف خود را روشن کنید.", guideClockMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleClockOn(ctx, userID, ub.Client)
		return c.Send("🟢 <b>ساعت زنده با موفقیت فعال شد و روی فامیلی اکانت قرار گرفت.</b>", guideClockMenu, tele.ModeHTML)
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
		return c.Send("🔴 <b>ساعت زنده خاموش شد و نام خانوادگی قبلی شما بازگردانده شد.</b>", guideClockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGEmoji, func(c tele.Context) error {
		userID := c.Sender().ID
		var isEmoji bool
		_ = db.QueryRow("SELECT is_emoji_enabled FROM users WHERE id = ?", userID).Scan(&isEmoji)
		statusStr := "🔴 خاموش"
		if isEmoji {
			statusStr = "🟢 روشن"
		}
		text := fmt.Sprintf(`🎭 <b>راهنمای اموجی رندوم کنار اسم</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>توضیحات:</b>
یک اموجی رندوم و جذاب کنار نام شما (First Name) قرار می‌گیرد و هر ۱۰ دقیقه یک‌بار به صورت خودکار تغییر می‌کند.

💬 <b>دستورات چت:</b>
▫️ روشن کردن: <code>اموجی روشن شو</code> یا <code>اموجی روشن</code>
▫️ خاموش کردن: <code>اموجی خاموش شو</code> یا <code>اموجی خاموش</code>

👇 همچنین می‌توانید از دکمه‌های زیر برای روشن/خاموش کردن استفاده کنید:`, statusStr)
		return c.Send(text, guideEmojiMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEmojiOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideEmojiMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleEmojiOn(ctx, userID, ub.Client)
		return c.Send("🟢 <b>اموجی رندوم کنار اسم با موفقیت روشن شد.</b>", guideEmojiMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEmojiOff, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideEmojiMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleEmojiOff(ctx, userID, ub.Client)
		return c.Send("🔴 <b>اموجی خاموش شد و اسم قبلی شما بازگردانده شد.</b>", guideEmojiMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGBio, func(c tele.Context) error {
		userID := c.Sender().ID
		var isBio bool
		var bioMode string
		_ = db.QueryRow("SELECT is_bio_enabled, bio_mode FROM users WHERE id = ?", userID).Scan(&isBio, &bioMode)
		statusStr := "🔴 خاموش"
		if isBio {
			if bioMode == "custom" {
				statusStr = "🟢 روشن (متن انتخابی)"
			} else {
				statusStr = "🟢 روشن (رندوم چرخشی)"
			}
		}
		text := fmt.Sprintf(`📝 <b>راهنمای بیوگرافی هوشمند و چرخشی</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>توضیحات:</b>
این قابلیت هر ۳۰ دقیقه بیوگرافی اکانت را از میان ۲۵ جمله و تک‌بیت مفهومی تغییر می‌دهد، یا هر پیامی را به بیو تبدیل می‌کند.

💬 <b>دستورات چت:</b>
▫️ روشن کردن بیو رندوم: <code>بیو روشن شو</code> یا <code>بیو روشن</code>
▫️ خاموش کردن و بازگردانی بیو قبلی: <code>بیو خاموش شو</code> یا <code>بیو خاموش</code>
▫️ تعویض فوری به بیو رندوم دیگر: <code>رندوم شو</code>
▫️ تبدیل متن پیام به بیو: ریپلای روی پیام و ارسال دستور <code>بیو شو</code>

👇 کنترل سریع بیو با دکمه‌های زیر:`, statusStr)
		return c.Send(text, guideBioMenu, tele.ModeHTML)
	})

	bot.Handle(&btnBioOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideBioMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleBioOn(ctx, userID, ub.Client)
		return c.Send("🟢 <b>بیوگرافی رندوم و چرخشی فعال شد.</b>", guideBioMenu, tele.ModeHTML)
	})

	bot.Handle(&btnBioOff, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil {
			return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideBioMenu, tele.ModeHTML)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleBioOff(ctx, userID, ub.Client)
		return c.Send("🔴 <b>بیوگرافی خاموش شد و بیوی اولیه شما بازگردانده شد.</b>", guideBioMenu, tele.ModeHTML)
	})

	// منوی دوست
	bot.Handle(&btnGFriend, func(c tele.Context) error {
		userID := c.Sender().ID
		var friendCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM wolf_friends WHERE owner_id = ?", userID).Scan(&friendCount)

		text := fmt.Sprintf(`🌸 <b>مدیریت سیستم هوشمند دوست</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>تعداد دوستان فعال:</b> <code>%d نفر</code> (همیشه فعال)
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
این قابلیت همیشه فعال است. به محض اینکه مخاطبی را با دستور <code>تنظیم دوست</code> ثبت کنید، هر پیامی در گروه‌ها بفرستد سلف‌بات شما بلافاصله روی پیامش ریپلای زده و یک متن دوستانه همراه با گل برایش ارسال می‌کند.

💬 <b>دستورات چت (با ریپلای روی پیام فرد):</b>
▫️ <b>افزودن دوست:</b> ریپلای روی پیام و ارسال <code>تنظیم دوست</code>
▫️ <b>حذف دوست:</b> ریپلای روی پیام دوست و ارسال <code>حذف دوست</code>
▫️ <b>لیست دوستان:</b> ارسال <code>لیست دوست</code>
▫️ <b>پاکسازی همه:</b> ارسال <code>پاکسازی دوست</code>`, friendCount)
		return c.Send(text, guideFriendMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFriendList, func(c tele.Context) error {
		userID := c.Sender().ID
		rows, err := db.Query("SELECT friend_id, friend_name FROM wolf_friends WHERE owner_id = ?", userID)
		if err != nil {
			return c.Send("❌ خطا در دریافت اطلاعات دوستان.", guideFriendMenu, tele.ModeHTML)
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

		msgText := "📋 <b>لیست دوستان شما:</b>\n\n" + strings.Join(list, "\n")
		if len(list) == 0 {
			msgText = "⚠️ <i>لیست دوستان شما در حال حاضر خالی است!</i>"
		}
		return c.Send(msgText, guideFriendMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFriendClear, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("DELETE FROM wolf_friends WHERE owner_id = ?", userID)
		clearFriendsCache(userID)
		return c.Send("🗑 <b>لیست دوستان شما به طور کامل پاکسازی شد 🌸</b>", guideFriendMenu, tele.ModeHTML)
	})

	// منوی دشمن
	bot.Handle(&btnGEnemy, func(c tele.Context) error {
		userID := c.Sender().ID
		var enemyCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM wolf_enemies WHERE owner_id = ?", userID).Scan(&enemyCount)

		text := fmt.Sprintf(`⚔️ <b>مدیریت سیستم هوشمند دشمن</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>تعداد دشمنان فعال:</b> <code>%d نفر</code> (همیشه فعال)
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
این قابلیت همیشه فعال است. با ریپلای روی پیام فرد و ارسال دستور <code>تنظیم دشمن</code>، از این پس هر پیامی در گروه‌ها بفرستد سلف‌بات شما بلافاصله با متن‌های تیکه‌دار و سنگین به او پاسخ می‌دهد.

💬 <b>دستورات چت (با ریپلای روی پیام فرد):</b>
▫️ <b>افزودن دشمن:</b> ریپلای روی پیام و ارسال <code>تنظیم دشمن</code>
▫️ <b>حذف دشمن:</b> ریپلای روی پیام و ارسال <code>حذف دشمن</code>
▫️ <b>لیست دشمنان:</b> ارسال <code>لیست دشمن</code>
▫️ <b>پاکسازی همه:</b> ارسال <code>پاکسازی دشمن</code>`, enemyCount)
		return c.Send(text, guideEnemyMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEnemyList, func(c tele.Context) error {
		userID := c.Sender().ID
		rows, err := db.Query("SELECT enemy_id, enemy_name FROM wolf_enemies WHERE owner_id = ?", userID)
		if err != nil {
			return c.Send("❌ خطا در دریافت اطلاعات دشمنان.", guideEnemyMenu, tele.ModeHTML)
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

		msgText := "⚔️ <b>لیست دشمنان شما:</b>\n\n" + strings.Join(list, "\n")
		if len(list) == 0 {
			msgText = "⚠️ <i>لیست دشمنان شما در حال حاضر خالی است!</i>"
		}
		return c.Send(msgText, guideEnemyMenu, tele.ModeHTML)
	})

	bot.Handle(&btnEnemyClear, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("DELETE FROM wolf_enemies WHERE owner_id = ?", userID)
		clearEnemiesCache(userID)
		return c.Send("🗑 <b>لیست دشمنان شما به طور کامل پاکسازی شد ⚔️</b>", guideEnemyMenu, tele.ModeHTML)
	})

	// منوی خوشنویسی
	formatFontModeName := func(mode string) string {
		switch mode {
		case "bold":
			return "بولد"
		case "italic":
			return "ایتالیک"
		case "bold_italic":
			return "بولد ایتالیک"
		case "underline":
			return "زیر خط"
		case "strike":
			return "خط خورده"
		case "mono":
			return "مونو"
		case "spoiler":
			return "اسپویل"
		default:
			return "بولد ایتالیک"
		}
	}

	bot.Handle(&btnGFont, func(c tele.Context) error {
		userID := c.Sender().ID
		en, mode := getFontSetting(userID)
		statusStr := "🔴 خاموش"
		if en {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf(`✒️ <b>مدیریت سیستم خوشنویسی و استایل پیام‌ها</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت:</b> %s
🔤 <b>فونت انتخابی فعلی:</b> <b>%s</b>
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
با روشن بودن این قابلیت، هر پیامی که در گروه‌ها یا پیوی ارسال کنید بلافاصله به فونت و استایل انتخابی شما تبدیل (Edit) می‌شود.

💬 <b>دستورات چت:</b>
▫️ روشن کردن: <code>خوشنویسی روشن</code>
▫️ خاموش کردن: <code>خوشنویسی خاموش</code>

▫️ <code>خوشنویسی بولد</code>
▫️ <code>خوشنویسی ایتالیک</code>
▫️ <code>خوشنویسی بولد ایتالیک</code> (پیش‌فرض)
▫️ <code>خوشنویسی زیر خط</code>
▫️ <code>خوشنویسی خط خورده</code>
▫️ <code>خوشنویسی مونو</code>
▫️ <code>خوشنویسی اسپویل</code>

👇 <i>همچنین می‌توانید مستقیماً از کلیدهای کیبورد زیر فونت دلخواه را تنظیم کنید:</i>`, statusStr, formatFontModeName(mode))

		return c.Send(text, guideFontMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFontOn, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_font_enabled = TRUE WHERE id = ?", userID)
		_, mode := getFontSetting(userID)
		updateFontCache(userID, true, mode)
		return c.Send("🟢 <b>سیستم خوشنویسی روشن شد.</b>", guideFontMenu, tele.ModeHTML)
	})

	bot.Handle(&btnFontOff, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_font_enabled = FALSE WHERE id = ?", userID)
		_, mode := getFontSetting(userID)
		updateFontCache(userID, false, mode)
		return c.Send("🔴 <b>سیستم خوشنویسی خاموش شد.</b>", guideFontMenu, tele.ModeHTML)
	})

	setFontHandler := func(mode, label string) tele.HandlerFunc {
		return func(c tele.Context) error {
			userID := c.Sender().ID
			_, _ = db.Exec("UPDATE users SET font_mode = ?, is_font_enabled = TRUE WHERE id = ?", mode, userID)
			updateFontCache(userID, true, mode)
			return c.Send(fmt.Sprintf("✅ <b>فونت خوشنویسی به «%s» تغییر یافت و روشن شد.</b>", label), guideFontMenu, tele.ModeHTML)
		}
	}

	bot.Handle(&btnFontBoldItalic, setFontHandler("bold_italic", "بولد ایتالیک"))
	bot.Handle(&btnFontBold, setFontHandler("bold", "بولد"))
	bot.Handle(&btnFontItalic, setFontHandler("italic", "ایتالیک"))
	bot.Handle(&btnFontUnderline, setFontHandler("underline", "زیر خط"))
	bot.Handle(&btnFontStrike, setFontHandler("strike", "خط خورده"))
	bot.Handle(&btnFontMono, setFontHandler("mono", "مونو"))
	bot.Handle(&btnFontSpoiler, setFontHandler("spoiler", "اسپویل"))

	// منوی اکشن‌ها
	bot.Handle(&btnGAction, func(c tele.Context) error {
		text := `🎬 <b>راهنمای اکشن‌های جعلی چت (Fake Actions)</b>
➖➖➖➖➖➖➖➖➖➖
📖 <b>عملکرد:</b>
با ارسال هر دستور، سلف‌بات وضعیت مورد نظر را در بالای صفحه چت (برای طرف مقابل یا گروه) شبیه‌سازی می‌کند.
هر دستور به طور خودکار هر ۴ ثانیه تمدید می‌شود تا قبل از پایان زمان قطع نشود.

💬 <b>دستورات چت (همراه با زمان دلخواه به ثانیه):</b>
<i>اگر زمان را وارد نکنید، به طور خودکار ۲۰ ثانیه در نظر گرفته می‌شود.</i>

▫️ ✍️ <b>در حال نوشتن:</b>
<code>تایپینگ 30</code> یا <code>اکشن تایپ 30</code>

▫️ 🎙 <b>در حال ضبط صدا (وویس):</b>
<code>اکشن وویس 20</code> یا <code>ضبط صدا 20</code>

▫️ 🔘 <b>در حال ضبط ویدیو دایره‌ای:</b>
<code>اکشن ویدیوگرد 20</code> یا <code>ویدیو گرد 20</code>

▫️ 🎥 <b>در حال ضبط ویدیو:</b>
<code>اکشن ویدیو 20</code> یا <code>ضبط ویدیو 20</code>

▫️ 📸 <b>در حال ارسال عکس:</b>
<code>اکشن عکس 20</code> یا <code>ارسال عکس 20</code>

▫️ 📁 <b>در حال ارسال فایل:</b>
<code>اکشن فایل 20</code> یا <code>ارسال فایل 20</code>

▫️ 🎮 <b>در حال بازی:</b>
<code>اکشن بازی 20</code> یا <code>بازی 20</code>

▫️ 🛑 <b>لغو فوری وضعیت:</b>
<code>لغو اکشن</code> یا <code>توقف اکشن</code>`

		return c.Send(text, guideActionMenu, tele.ModeHTML)
	})

	// منوی پاکسازی هوشمند
	bot.Handle(&btnGPurge, func(c tele.Context) error {
		text := `🗑 <b>راهنمای پاکسازی سریع و هوشمند پیام‌ها (پاکشو)</b>
➖➖➖➖➖➖➖➖➖➖
📖 <b>عملکرد:</b>
این قابلیت به شما امکان می‌دهد پیام‌های ارسالی خودتان را در هر گروه یا چت خصوصی با بیشترین سرعت و بدون باقی ماندن ردپا پاکسازی کنید.

💬 <b>دستورات چت:</b>

▫️ <b>۱. حذف بر اساس تعداد:</b>
ارسال دستور <code>پاکشو 20</code>
<i>(تعداد پیام‌های مشخص شده از آخرین پیام‌های خودتان را پاک می‌کند - حداکثر ۱۰۰ عدد در هر بار)</i>

▫️ <b>۲. حذف از یک نقطه خاص (با ریپلای):</b>
روی پیام قدیمی خودت ریپلای کن و بفرست:
<code>پاکشو</code>
<i>(تمام پیام‌های ارسالی شما از آن پیام ریپلای‌شده تا پیام فعلی پاک خواهند شد)</i>`

		return c.Send(text, guidePurgeMenu, tele.ModeHTML)
	})

	// منوی تایمر زنده
	bot.Handle(&btnGTimer, func(c tele.Context) error {
		text := `⏳ <b>راهنمای شمارش معکوس زنده (تایمر)</b>
➖➖➖➖➖➖➖➖➖➖
📖 <b>عملکرد:</b>
این قابلیت پیام شما را به یک تایمر شمارش معکوس زنده تبدیل می‌کند که ثانیه به ثانیه تغییر کرده و در نهایت با یک اموجی هیجان‌انگیز (💥) به پایان می‌رسد!

💬 <b>دستورات چت:</b>

▫️ <code>تایمر 10</code>
▫️ <code>شمارش 5</code>
<i>(عدد مقابل دستور، زمان تایمر به ثانیه است. برای جلوگیری از محدودیت تلگرام، حداکثر زمان مجاز ۶۰ ثانیه می‌باشد)</i>

⚡ <i>این دستور مستقیماً روی پیام خودتان اعمال شده و به صورت زنده ویرایش می‌شود.</i>`

		return c.Send(text, guideTimerMenu, tele.ModeHTML)
	})

	// منوی ری‌اکشن خودکار
	bot.Handle(&btnGAutoReact, func(c tele.Context) error {
		text := `🔥 <b>راهنمای ری‌اکشن خودکار (Auto-React)</b>
➖➖➖➖➖➖➖➖➖➖
📖 <b>عملکرد:</b>
با این قابلیت بسیار جذاب، می‌توانید کاری کنید که به محض اینکه فرد خاصی پیامی ارسال کرد، سلف‌بات شما در کمتر از کسر ثانیه (سریع‌تر از هر انسانی) دقیقاً روی پیام او ری‌اکشن دلخواهتان (مانند ❤️ یا 🔥) را بزند!

💬 <b>دستورات چت (با ریپلای روی پیام فرد):</b>

▫️ <b>ثبت ری‌اکشن:</b>
روی پیام شخص ریپلای کنید و بفرستید:
<code>ری‌اکشن 🔥</code> یا <code>ری‌اکشن 👎</code>
<i>(اگر فقط کلمه «ری‌اکشن» را بفرستید، به طور پیش‌فرض ❤️ تنظیم می‌شود)</i>

▫️ <b>لغو برای یک فرد:</b>
روی پیام شخص ریپلای کنید و بفرستید:
<code>حذف ری‌اکشن</code>

▫️ <b>مشاهده لیست افراد:</b>
ارسال دستور <code>لیست ری‌اکشن</code>

▫️ <b>پاکسازی همه:</b>
ارسال دستور <code>پاکسازی ری‌اکشن</code>`

		return c.Send(text, guideAutoReactMenu, tele.ModeHTML)
	})

	bot.Handle(&btnPVBack, func(c tele.Context) error {
		text := `📩 <b>راهنمای فوروارد همگانی به پیوی‌ها (Broadcast PV)</b>
➖➖➖➖➖➖➖➖➖➖
📖 <b>نحوه ارسال:</b>
روی پیام مورد نظر خود در هر چتی ریپلای (Reply) کرده و یکی از دستورات زیر را بفرستید:

🔸 <code>بفرست پیوی همه</code>
پیام بدون درج نام فرستنده اصلی برای تمام مخاطبان خصوصی فوروارد می‌شود.

🔸 <code>پیوی همه</code>
پیام با حفظ نام فرستنده برای تمامی پیوی‌ها فوروارد می‌گردد.`
		return c.Send(text, guidePVMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGGroup, func(c tele.Context) error {
		text := `👥 <b>راهنمای فوروارد همگانی به گروه‌ها (Broadcast Groups)</b>
➖➖➖➖➖➖➖➖➖➖
📖 <b>نحوه ارسال:</b>
روی پیام مورد نظر در هر چتی ریپلای (Reply) کرده و یکی از دستورات زیر را بفرستید:

🔸 <code>بفرست گروه همه</code>
پیام بدون درج نام فرستنده اصلی برای تمام گروه‌ها فوروارد می‌شود.

🔸 <code>گروه همه</code>
پیام با حفظ نام فرستنده برای تمامی گروه‌ها ارسال می‌گردد.`
		return c.Send(text, guideGroupMenu, tele.ModeHTML)
	})

	backToGuideHandler := func(c tele.Context) error {
		userID := c.Sender().ID
		return c.Send(buildGuideDashboardText(userID), guideMenu, tele.ModeHTML)
	}

	bot.Handle(&btnClockBack, backToGuideHandler)
	bot.Handle(&btnEmojiBack, backToGuideHandler)
	bot.Handle(&btnBioBack, backToGuideHandler)
	bot.Handle(&btnFriendBack, backToGuideHandler)
	bot.Handle(&btnEnemyBack, backToGuideHandler)
	bot.Handle(&btnFontBack, backToGuideHandler)
	bot.Handle(&btnActionBack, backToGuideHandler)
	bot.Handle(&btnPurgeBack, backToGuideHandler)
	bot.Handle(&btnTimerBack, backToGuideHandler)
	bot.Handle(&btnAutoReactBack, backToGuideHandler)
	bot.Handle(&btnPVBack, backToGuideHandler)
	bot.Handle(&btnGroupBack, backToGuideHandler)

	bot.Handle(&btnGBackMain, func(c tele.Context) error {
		return c.Send("🔙 <b>به منوی اصلی بازگشتید.</b>", getMainKeyboard(c.Sender().ID), tele.ModeHTML)
	})

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

	bot.Handle(&btnCancelReceipt, func(c tele.Context) error {
		userID := c.Sender().ID
		stateMu.Lock()
		userWalletTemp[userID] = 0
		stateMu.Unlock()
		return c.Send("❌ <b>فرآیند پرداخت لغو گردید.</b>", getMainKeyboard(userID), tele.ModeHTML)
	})

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

	bot.Handle(&btnTurnOnSelf, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		selfStatus := GetUserSelfStatus(userID)

		if selfStatus == "روشن" {
			return c.Send("⚠️ <b>سلف روشن است.</b>", getKeyboard(userID), tele.ModeHTML)
		}

		sessionPath := fmt.Sprintf("/opt/wolf/sessions/user_%d.json", userID)
		_, statErr := os.Stat(sessionPath)
		hasSession := (statErr == nil)

		if selfStatus == "خاموش" && hasSession {
			_, _ = db.Exec("UPDATE users SET self_status = 'روشن' WHERE id = ?", userID)
			startUserbot(userID, cfg, bot)

			return c.Send("🟢 <b>سلف شما با موفقیت روشن شد و امکانات مجدداً فعال گردید.</b>", getKeyboard(userID), tele.ModeHTML)
		}

		text := "❌ <b>سلف شما فعال نیست!</b>\n\n" +
			"لطفاً برای راه‌اندازی و اتصال سلف، ابتدا از بخش <b>🛍️ خرید سلف</b> اقدام نمایید."
		return c.Send(text, getKeyboard(userID), tele.ModeHTML)
	})

	bot.Handle(&btnTurnOffSelf, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خاموش" {
			return c.Send("⚠️ <b>سلف خاموش هست.</b>", getKeyboard(userID), tele.ModeHTML)
		}
		if selfStatus != "روشن" {
			return c.Send("❌ <b>شما سلف فعالی ندارید.</b>", getKeyboard(userID), tele.ModeHTML)
		}

		var isClock, isEmoji, isBio bool
		var origLast, origFirst, origBio string
		_ = db.QueryRow("SELECT is_clock_enabled, original_last_name, is_emoji_enabled, original_first_name, is_bio_enabled, original_bio FROM users WHERE id = ?", userID).Scan(&isClock, &origLast, &isEmoji, &origFirst, &isBio, &origBio)

		activeUserbotsMu.RLock()
		if ub, ok := activeUserbots[userID]; ok && ub.Client != nil {
			req := &tg.AccountUpdateProfileRequest{}
			needRevert := false
			if isClock {
				req.SetLastName(origLast)
				needRevert = true
			}
			if isEmoji && origFirst != "" {
				req.SetFirstName(origFirst)
				needRevert = true
			}
			if isBio && origBio != "" {
				req.SetAbout(origBio)
				needRevert = true
			}
			if needRevert {
				cTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, _ = ub.Client.API().AccountUpdateProfile(cTimeout, req)
				cancel()
			}
		}
		activeUserbotsMu.RUnlock()

		_, _ = db.Exec("UPDATE users SET self_status = 'خاموش' WHERE id = ?", userID)
		stopUserbot(userID)

		return c.Send("🔴 <b>سلف شما خاموش شد.</b>\nامکانات سلف غیرفعال گردید، اما اتصال اکانت شما برقرار است.", getKeyboard(userID), tele.ModeHTML)
	})

	bot.Handle(&btnExitSelf, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Send("❌ <b>شما سلف فعالی ندارید که از آن خارج شوید.</b>", getKeyboard(userID), tele.ModeHTML)
		}

		exitMenu := &tele.ReplyMarkup{}
		btnConfirm := exitMenu.Data("🛑 بله، خروج قطعی", "exit_confirm")
		btnCancel := exitMenu.Data("❌ انصراف", "exit_cancel")
		exitMenu.Inline(exitMenu.Row(btnConfirm, btnCancel))

		text := "⚠️ <b>آیا مطمئن هستید که می‌خواهید از سلف خارج شوید؟</b>\n\nبا تایید این گزینه، اتصال اکانت شما به طور کامل قطع شده و فایل نشست (Session) شما از سرور حذف خواهد شد."
		return c.Send(text, exitMenu, tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "exit_confirm"}, func(c tele.Context) error {
		userID := c.Sender().ID

		stopUserbot(userID)

		stateMu.Lock()
		if uState, exists := userStates[userID]; exists {
			if uState.Cancel != nil {
				uState.Cancel()
			}
			delete(userStates, userID)
		}
		stateMu.Unlock()

		sessionPath := fmt.Sprintf("/opt/wolf/sessions/user_%d.json", userID)
		_ = os.Remove(sessionPath)

		_, _ = db.Exec("UPDATE users SET self_status = 'خروج', phone = 'ثبت نشده', is_clock_enabled = FALSE, is_emoji_enabled = FALSE, is_timer_media_enabled = FALSE, is_bio_enabled = FALSE, is_anti_delete_enabled = FALSE, is_edit_logger_enabled = FALSE, is_protected_saver_enabled = FALSE, is_ghost_mode_enabled = FALSE, is_font_enabled = FALSE WHERE id = ?", userID)

		if c.Message() != nil {
			_ = bot.Delete(c.Message())
		}

		return c.Send("🛑 <b>شما با موفقیت از سیستم سلف خارج شدید و نشست اکانت شما حذف شد.</b>", getKeyboard(userID), tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "exit_cancel"}, func(c tele.Context) error {
		if c.Message() != nil {
			_ = bot.Delete(c.Message())
		}
		return c.Send("✅ <b>عملیات خروج لغو شد و سلف شما دست‌نخورده باقی ماند.</b>", getKeyboard(c.Sender().ID), tele.ModeHTML)
	})

	bot.Handle(&btnConfirmSelfAction, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		price := getKeyPrice()
		balance := GetUserBalance(userID)
		keys := balance / price

		if keys < 30 {
			return c.Send("❌ <b>شما کلید کافی برای فعالسازی ندارید!</b>", getKeyboard(userID), tele.ModeHTML)
		}

		stateMu.Lock()
		userStates[userID] = &UserState{Action: "waiting_for_contact"}
		stateMu.Unlock()

		shareMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
		btnShare := shareMenu.Contact("📱 ارسال شماره اکانت (Share Contact)")
		btnBackShare := shareMenu.Text("🔙 بازگشت")
		shareMenu.Reply(shareMenu.Row(btnShare), shareMenu.Row(btnBackShare))

		text := "📞 <b>مرحله اول: تایید اکانت تلگرام</b>\n\n" +
			"برای اتصال ربات به اکانت شما، لطفاً شماره تلگرام خود را از طریق دکمه پایین صفحه <b>(📱 ارسال شماره اکانت)</b> با ما به اشتراک بگذارید."

		return c.Send(text, shareMenu, tele.ModeHTML)
	})

	bot.Handle(tele.OnContact, func(c tele.Context) error {
		userID := c.Sender().ID

		stateMu.RLock()
		state, exists := userStates[userID]
		stateMu.RUnlock()

		if !exists || state.Action != "waiting_for_contact" {
			return nil
		}

		contact := c.Message().Contact
		if contact.UserID != userID {
			return c.Send("❌ <b>خطا!</b> لطفاً شماره خودتان را ارسال کنید، نه شخص دیگر!", tele.ModeHTML)
		}

		codeChan := make(chan string, 1)
		passwordChan := make(chan string, 1)
		resultChan := make(chan AuthResult, 1)
		ctx, cancel := context.WithCancel(context.Background())

		authHandler := &botAuthenticator{
			phone:        contact.PhoneNumber,
			codeChan:     codeChan,
			passwordChan: passwordChan,
			resultChan:   resultChan,
		}

		stateMu.Lock()
		userStates[userID] = &UserState{
			Action:       "waiting_for_code",
			Phone:        contact.PhoneNumber,
			CodeChan:     codeChan,
			PasswordChan: passwordChan,
			ResultChan:   resultChan,
			Cancel:       cancel,
		}
		stateMu.Unlock()

		go startTelegramLogin(ctx, userID, cfg, authHandler)

		codeMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
		btnB := codeMenu.Text("🔙 بازگشت")
		codeMenu.Reply(codeMenu.Row(btnB))

		text := fmt.Sprintf("✅ <b>شماره %s تایید شد و درخواست کد به تلگرام ارسال گردید.</b>\n\n"+
			"📲 <b>مرحله دوم: ورود کد تایید</b>\n\n"+
			"لطفاً کد ۵ رقمی ارسال شده توسط تلگرام را <b>با فاصله</b> ارسال کنید:\n\n"+
			"مثال: <code>1 2 3 4 5</code>", contact.PhoneNumber)

		return c.Send(text, codeMenu, tele.ModeHTML)
	})

	bot.Handle(&btnAdminPanel, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send("❌ شما دسترسی به بخش مدیریت را ندارید.")
		}
		return c.Send(getAdminDashboard(), adminPanelMenu, tele.ModeHTML)
	})

	bot.Handle(&btnConfigAccount, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send("❌ شما دسترسی ندارید.")
		}
		text := "💳 <b>بخش تنظیمات اطلاعات بانکی</b>\n\nلطفاً برای مشاهده و تغییر اطلاعات، از دکمه‌های زیر استفاده کنید:"
		return c.Send(text, accountConfigMenu, tele.ModeHTML)
	})

	bot.Handle(&btnConfigCardNum, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return nil
		}
		current := GetSetting("card_number")
		text := fmt.Sprintf("💳 <b>تنظیم شماره کارت</b>\n\n🔹 مقدار فعلی: <code>%s</code>\n\n✏️ <i>لطفاً شماره کارت جدید را ارسال کنید:</i>", current)
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_card_num"}
		stateMu.Unlock()
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnConfigCardName, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return nil
		}
		current := GetSetting("card_name")
		text := fmt.Sprintf("👤 <b>تنظیم نام صاحب حساب</b>\n\n🔹 مقدار فعلی: <b>%s</b>\n\n✏️ <i>لطفاً نام جدید دارنده حساب را ارسال کنید:</i>", current)
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_card_name"}
		stateMu.Unlock()
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnConfigCardBank, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return nil
		}
		current := GetSetting("card_bank")
		text := fmt.Sprintf("🏦 <b>تنظیم نام بانک</b>\n\n🔹 مقدار فعلی: <b>%s</b>\n\n✏️ <i>لطفاً نام بانک جدید را ارسال کنید:</i>", current)
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_card_bank"}
		stateMu.Unlock()
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnConfigSupport, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send("❌ شما دسترسی ندارید.")
		}
		text := "📞 <b>بخش تنظیمات پشتیبانی</b>\n\nاز طریق منوی زیر می‌توانید متن و آیدی پشتیبانی که به کاربران نمایش داده می‌شود را تغییر دهید:"
		return c.Send(text, supportConfigMenu, tele.ModeHTML)
	})

	bot.Handle(&btnConfigSupportText, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return nil
		}
		current := GetSetting("support_text")
		text := fmt.Sprintf("📝 <b>تنظیم متن پشتیبانی</b>\n\n🔹 مقدار فعلی:\n<i>%s</i>\n\n✏️ <i>لطفاً متن جدید پشتیبانی را ارسال کنید:</i>", current)
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_support_text"}
		stateMu.Unlock()
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnConfigSupportID, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return nil
		}
		current := GetSetting("support_id")
		text := fmt.Sprintf("🆔 <b>تنظیم آیدی پشتیبانی</b>\n\n🔹 مقدار فعلی: <b>%s</b>\n\n✏️ <i>لطفاً آیدی جدید پشتیبانی (مثال: @YourID) را ارسال کنید:</i>", current)
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_support_id"}
		stateMu.Unlock()
		return c.Send(text, tele.ModeHTML)
	})

	bot.Handle(&btnConfigKeyPrice, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return nil
		}
		currentPrice := getKeyPrice()
		text := fmt.Sprintf("🔑 <b>تنظیم نرخ کلید</b>\n\n🔹 قیمت فعلی: <code>%s تومان</code>\n\n✏️ <i>لطفاً مبلغ جدید را (فقط عدد به تومان) ارسال کنید:</i>", formatMoney(currentPrice))
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_key_price"}
		stateMu.Unlock()
		return c.Send(text, tele.ModeHTML)
	})

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

	bot.Handle(&tele.Btn{Unique: "admin_block"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}
		targetUserID, _ := strconv.ParseInt(c.Data(), 10, 64)
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "block_reason", TargetID: targetUserID}
		stateMu.Unlock()

		if c.Message() != nil {
			_ = c.Reply("🚫 <b>لطفاً دلیل مسدودی را ارسال کنید تا به همراه پیام مسدودی به صورت بولد برای کاربر ارسال شود:</b>", tele.ModeHTML)
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
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "admin_msg"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}
		targetUserID, _ := strconv.ParseInt(c.Data(), 10, 64)
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "msg", TargetID: targetUserID}
		stateMu.Unlock()

		if c.Message() != nil {
			_ = c.Reply("💬 <b>لطفاً متن پیام خود برای کاربر را ارسال کنید:</b>", tele.ModeHTML)
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "admin_manual"}, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید."})
		}
		targetUserID, _ := strconv.ParseInt(c.Data(), 10, 64)
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "manual_add", TargetID: targetUserID}
		stateMu.Unlock()

		if c.Message() != nil {
			_ = c.Reply("💰 <b>لطفاً مبلغ مورد نظر برای افزایش دستی موجودی را (فقط عدد به تومان) ارسال کنید:</b>", tele.ModeHTML)
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
		if HandleWolfPlusText(c) {
			return nil
		}

		userID := c.Sender().ID
		text := strings.TrimSpace(c.Text())

		stateMu.RLock()
		uState, userHasState := userStates[userID]
		stateMu.RUnlock()

		if userHasState && uState != nil {
			if uState.Action == "waiting_for_code" {
				cleanCode := extractDigits(text)
				if len(cleanCode) < 5 {
					return c.Send("❌ <b>کد وارد شده نامعتبر است!</b>\nلطفاً کد ۵ رقمی را با فاصله ارسال کنید (مثال: <code>1 2 3 4 5</code>):", tele.ModeHTML)
				}

				select {
				case uState.CodeChan <- cleanCode:
					select {
					case res := <-uState.ResultChan:
						if res.Type == AuthResultNeeds2FA {
							stateMu.Lock()
							uState.Action = "waiting_for_password"
							stateMu.Unlock()
							return c.Send("🔒 <b>حساب شما دارای رمز عبور تایید دو مرحله‌ای (2FA) است.</b>\n\nلطفاً رمز عبور خود را ارسال کنید:", tele.ModeHTML)
						} else if res.Type == AuthResultSuccess {
							_, _ = db.Exec("UPDATE users SET self_status = 'روشن', phone = ?, last_billed_at = NOW() WHERE id = ?", uState.Phone, userID)
							stateMu.Lock()
							delete(userStates, userID)
							stateMu.Unlock()

							startUserbot(userID, cfg, bot)

							successText := "🎉 <b>تبریک! سلف شما با موفقیت به اکانت متصل و فعال شد!</b> 🐺\n\n" +
								"✅ <i>وضعیت اکانت شما هم‌اکنون به حالت روشن تغییر یافت.</i>\n" +
								"از این پس روزانه ۱ کلید از حساب شما کسر خواهد شد و امکانات سلف فعال است. 🚀"
							return c.Send(successText, getKeyboard(userID), tele.ModeHTML)
						} else {
							stateMu.Lock()
							if uState.Cancel != nil {
								uState.Cancel()
							}
							delete(userStates, userID)
							stateMu.Unlock()
							return c.Send(fmt.Sprintf("❌ <b>خطا در ورود به اکانت:</b> %v\n\nلطفاً دوباره از بخش فعالسازی سلف اقدام فرمایید.", res.Error), getKeyboard(userID), tele.ModeHTML)
						}
					case <-time.After(35 * time.Second):
						stateMu.Lock()
						if uState.Cancel != nil {
							uState.Cancel()
						}
						delete(userStates, userID)
						stateMu.Unlock()
						return c.Send("⏱️ <b>زمان پاسخ تلگرام به پایان رسید.</b>\nلطفاً مجدداً تلاش فرمایید.", getKeyboard(userID), tele.ModeHTML)
					}
				default:
					return c.Send("⏳ در حال پردازش کد...")
				}
			} else if uState.Action == "waiting_for_password" {
				select {
				case uState.PasswordChan <- text:
					select {
					case res := <-uState.ResultChan:
						if res.Type == AuthResultSuccess {
							_, _ = db.Exec("UPDATE users SET self_status = 'روشن', phone = ?, last_billed_at = NOW() WHERE id = ?", uState.Phone, userID)
							stateMu.Lock()
							delete(userStates, userID)
							stateMu.Unlock()

							startUserbot(userID, cfg, bot)

							successText := "🎉 <b>تبریک! رمز دو مرحله‌ای تایید شد و سلف متصل گردید!</b> 🐺\n\n" +
								"✅ <i>وضعیت اکانت شما به حالت روشن تغییر یافت.</i>"
							return c.Send(successText, getKeyboard(userID), tele.ModeHTML)
						} else {
							stateMu.Lock()
							if uState.Cancel != nil {
								uState.Cancel()
							}
							delete(userStates, userID)
							stateMu.Unlock()
							return c.Send(fmt.Sprintf("❌ <b>رمز عبور اشتباه است:</b> %v\n\nلطفاً دوباره مراحل فعالسازی را از سر بگیرید.", res.Error), getKeyboard(userID), tele.ModeHTML)
						}
					case <-time.After(35 * time.Second):
						stateMu.Lock()
						if uState.Cancel != nil {
							uState.Cancel()
						}
						delete(userStates, userID)
						stateMu.Unlock()
						return c.Send("⏱️ <b>زمان پاسخ تلگرام به پایان رسید.</b>\nلطفاً دوباره تلاش کنید.", getKeyboard(userID), tele.ModeHTML)
					}
				default:
					return c.Send("⏳ در حال بررسی رمز عبور...")
				}
			}
		}

		if !cfg.IsAdmin(userID) {
			return nil
		}

		stateMu.RLock()
		state, adminHasState := adminStates[userID]
		stateMu.RUnlock()

		if !adminHasState {
			return nil
		}

		switch state.Action {
		case "set_card_num":
			SetSetting("card_number", text)
			_ = c.Send("✅ <b>شماره کارت با موفقیت به‌روزرسانی شد.</b>", tele.ModeHTML)
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()

		case "set_card_name":
			SetSetting("card_name", text)
			_ = c.Send("✅ <b>نام صاحب حساب با موفقیت به‌روزرسانی شد.</b>", tele.ModeHTML)
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()

		case "set_card_bank":
			SetSetting("card_bank", text)
			_ = c.Send("✅ <b>نام بانک با موفقیت به‌روزرسانی شد.</b>", tele.ModeHTML)
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()

		case "set_support_text":
			SetSetting("support_text", text)
			_ = c.Send("✅ <b>متن پشتیبانی با موفقیت به‌روزرسانی شد.</b>", tele.ModeHTML)
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()

		case "set_support_id":
			SetSetting("support_id", text)
			_ = c.Send("✅ <b>آیدی پشتیبانی با موفقیت به‌روزرسانی شد.</b>", tele.ModeHTML)
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()

		case "set_key_price":
			price, err := strconv.Atoi(text)
			if err != nil || price <= 0 {
				_ = c.Send("❌ <b>مبلغ نامعتبر است.</b>\nلطفاً فقط یک عدد صحیح (بدون کاما، حرف یا ریال) وارد کنید.", tele.ModeHTML)
				return nil
			}
			SetSetting("key_price", strconv.Itoa(price))
			_ = c.Send(fmt.Sprintf("✅ <b>نرخ کلید با موفقیت به %s تومان تغییر یافت.</b>\n\nاز این پس تمامی محاسبات بر اساس نرخ جدید انجام خواهد شد.", formatMoney(price)), tele.ModeHTML)
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()

		case "msg":
			_, err := bot.Send(&tele.User{ID: state.TargetID}, fmt.Sprintf("💬 <b>پیام از طرف مدیریت:</b>\n\n%s", html.EscapeString(text)), tele.ModeHTML)
			if err == nil {
				_ = c.Send("✅ پیام با موفقیت به کاربر ارسال شد.")
			} else {
				_ = c.Send("❌ خطا در ارسال پیام به کاربر.")
			}
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()

		case "manual_add":
			amount, err := strconv.Atoi(text)
			if err != nil || amount <= 0 {
				_ = c.Send("❌ مبلغ نامعتبر است. لطفاً فقط یک عدد صحیح وارد کنید.")
				return nil
			}
			SafeAddUserBalance(state.TargetID, amount)
			_, _ = bot.Send(&tele.User{ID: state.TargetID}, fmt.Sprintf("💰 <b>موجودی کیف پول شما به صورت دستی شارژ شد:</b>\n\nمبلغ: <code>%s تومان</code>", formatMoney(amount)), tele.ModeHTML)
			_ = c.Send(fmt.Sprintf("✅ مبلغ %s تومان با موفقیت به کیف پول کاربر اضافه شد.", formatMoney(amount)))
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()

		case "block_reason":
			_, _ = db.Exec("UPDATE users SET is_blocked = TRUE WHERE id = ?", state.TargetID)
			_, err := bot.Send(&tele.User{ID: state.TargetID}, fmt.Sprintf("❌ <b>حساب کاربری شما مسدود شد.</b>\n\n<b>دلیل مسدودی: %s</b>", html.EscapeString(text)), tele.ModeHTML)
			if err == nil {
				_ = c.Send("✅ کاربر مسدود شد و دلیل به صورت بولد برایش ارسال گردید.")
			} else {
				_ = c.Send("❌ خطا در ارسال پیام به کاربر.")
			}
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()
		}
		return nil
	})

	bot.Handle(&btnSupport, func(c tele.Context) error {
		if IsUserBlocked(c.Sender().ID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}
		sText := GetSetting("support_text")
		sID := GetSetting("support_id")
		text := fmt.Sprintf("%s\n\n🆔 <b>آیدی ارتباط:</b> %s", sText, sID)
		return c.Send(text, tele.ModeHTML)
	})

	startBillingWorker(bot)
	startClockWorker()
	startEmojiWorker()
	startBioWorker()

	rows, err := db.Query("SELECT id FROM users WHERE self_status = 'روشن'")
	if err == nil {
		for rows.Next() {
			var uid int64
			if err := rows.Scan(&uid); err == nil {
				startUserbot(uid, cfg, bot)
			}
		}
		rows.Close()
	}

	log.Println("⚡ ربات ولف سلف با بالاترین امنیت و موتور استاندارد آماده و روشن شد!")
	bot.Start()
}
