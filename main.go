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
	"unicode/utf16"

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

var (
	db              *sql.DB
	controllerBotID int64
	cfg             Config

	userMenu       = &tele.ReplyMarkup{ResizeKeyboard: true}
	adminMenu      = &tele.ReplyMarkup{ResizeKeyboard: true}
	adminPanelMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	profileMenu    = &tele.ReplyMarkup{ResizeKeyboard: true}

	accountConfigMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	supportConfigMenu = &tele.ReplyMarkup{ResizeKeyboard: true}

	btnBuy        = userMenu.Text("🛍️ خرید سلف")
	btnProfile    = userMenu.Text("👤 حساب کاربری")
	btnWallet     = userMenu.Text("👛 کیف پول 💳")
	btnWolfPlus   = userMenu.Text("🐺 ولف +")
	btnSupport    = userMenu.Text("🎧 پشتیبانی")
	btnGuide      = userMenu.Text("📚 راهنما")
	btnAdminPanel = adminMenu.Text("⚙️ مدیریت")
	btnBack       = adminMenu.Text("🔙 بازگشت")

	btnTurnOnSelf  = profileMenu.Text("🟢 روشن کردن سلف")
	btnTurnOffSelf = profileMenu.Text("🔴 خاموش کردن سلف")
	btnExitSelf    = profileMenu.Text("🛑 خروج سلف")

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

	stateMu        sync.RWMutex
	adminStates    = make(map[int64]AdminAction)
	userStates     = make(map[int64]*UserState)
	userWalletTemp = make(map[int64]int)

	activeUserbotsMu sync.RWMutex
	activeUserbots   = make(map[int64]*UserbotSession)

	friendsCacheMu sync.RWMutex
	friendsCache   = make(map[int64]map[int64]bool)

	enemiesCacheMu sync.RWMutex
	enemiesCache   = make(map[int64]map[int64]bool)

	autoReactsCacheMu sync.RWMutex
	autoReactsCache   = make(map[int64]map[int64]string)

	fontSettingsMu sync.RWMutex
	fontSettings   = make(map[int64]struct {
		Enabled bool
		Mode    string
	})

	activeActionsMu sync.Mutex
	activeActions   = make(map[string]context.CancelFunc)

	channelAccessHashesMu sync.RWMutex
	channelAccessHashes   = make(map[int64]int64)
)

var randomEmojiPool = []string{"🐺", "👑", "⚡", "🔥", "💎", "✨", "🚀", "🪐", "🌪", "🦁"}
var randomBioPool = []string{"🐺 در سکوت شب، زوزه‌ی گرگ شنیدنی‌تر است.", "⚡️ قوی بمان، قصه‌ی تو پایان درخشانی دارد."}

func getRandomEmoji() string { return randomEmojiPool[rand.Intn(len(randomEmojiPool))] }
func getRandomBio() string   { return randomBioPool[rand.Intn(len(randomBioPool))] }

func cleanName(name string) string {
	name = strings.TrimSpace(name)
	for _, em := range randomEmojiPool {
		name = strings.TrimSpace(strings.TrimSuffix(name, em))
	}
	return name
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

func (c *Config) IsAdmin(userID int64) bool {
	for _, id := range c.AdminIDs {
		if id == userID {
			return true
		}
	}
	return false
}

func loadConfig() Config {
	_ = godotenv.Load()
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("❌ مقدار BOT_TOKEN در فایل .env یافت نشد.")
	}
	apiID, _ := strconv.Atoi(os.Getenv("API_ID"))
	apiHash := os.Getenv("API_HASH")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")

	var adminIDs []int64
	for _, idStr := range strings.Split(os.Getenv("ADMIN_ID"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
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

func InitDB(cfg Config) {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s?parseTime=true&charset=utf8mb4", cfg.DBUser, cfg.DBPass, cfg.DBName)
	db, err = sql.Open("mysql", dsn)
	if err != nil || db.Ping() != nil {
		log.Fatalf("❌ خطا در اتصال به دیتابیس: %v", err)
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
		font_mode VARCHAR(30) DEFAULT 'bold'
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_friends (
		owner_id BIGINT,
		friend_id BIGINT,
		friend_name VARCHAR(255) DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, friend_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec(`
	CREATE TABLE IF NOT EXISTS wolf_enemies (
		owner_id BIGINT,
		enemy_id BIGINT,
		enemy_name VARCHAR(255) DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (owner_id, enemy_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

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
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('support_text', '🎧 پشتیبانی ولف سلف')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('support_id', '@JavadWolf')`)
	db.Exec(`INSERT IGNORE INTO settings (setting_key, setting_value) VALUES ('key_price', '3333')`)

	loadAllFriendsToCache()
	loadAllEnemiesToCache()
	loadAllAutoReactsToCache()
	loadAllFontSettingsToCache()
}

func loadAllAutoReactsToCache() {
	if db == nil { return }
	rows, err := db.Query("SELECT owner_id, target_id, emoji FROM wolf_auto_reacts")
	if err != nil { return }
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
	if uSet, exists := autoReactsCache[ownerID]; exists {
		em, found := uSet[targetID]
		return em, found
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
	if uSet, exists := autoReactsCache[ownerID]; exists {
		delete(uSet, targetID)
	}
}

func clearAutoReactsCache(ownerID int64) {
	autoReactsCacheMu.Lock()
	defer autoReactsCacheMu.Unlock()
	delete(autoReactsCache, ownerID)
}

func loadAllFriendsToCache() {
	if db == nil { return }
	rows, err := db.Query("SELECT owner_id, friend_id FROM wolf_friends")
	if err != nil { return }
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
	if uSet, exists := friendsCache[ownerID]; exists {
		return uSet[targetID]
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
	if uSet, exists := friendsCache[ownerID]; exists {
		delete(uSet, friendID)
	}
}

func clearFriendsCache(ownerID int64) {
	friendsCacheMu.Lock()
	defer friendsCacheMu.Unlock()
	delete(friendsCache, ownerID)
}

func loadAllEnemiesToCache() {
	if db == nil { return }
	rows, err := db.Query("SELECT owner_id, enemy_id FROM wolf_enemies")
	if err != nil { return }
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
	if uSet, exists := enemiesCache[ownerID]; exists {
		return uSet[targetID]
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
	if uSet, exists := enemiesCache[ownerID]; exists {
		delete(uSet, enemyID)
	}
}

func clearEnemiesCache(ownerID int64) {
	enemiesCacheMu.Lock()
	defer enemiesCacheMu.Unlock()
	delete(enemiesCache, ownerID)
}

func loadAllFontSettingsToCache() {
	if db == nil { return }
	rows, err := db.Query("SELECT id, is_font_enabled, font_mode FROM users")
	if err != nil { return }
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
				mode = "bold"
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
	return false, "bold"
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
	if durationSec <= 0 { durationSec = 20 }
	if durationSec > 300 { durationSec = 300 }

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

		_, _ = client.API().MessagesSetTyping(actCtx, &tg.MessagesSetTypingRequest{Peer: inputPeer, Action: action})
		for {
			select {
			case <-actCtx.Done():
				cCtx, cCancel := context.WithTimeout(context.Background(), 3*time.Second)
				_, _ = client.API().MessagesSetTyping(cCtx, &tg.MessagesSetTypingRequest{Peer: inputPeer, Action: &tg.SendMessageCancelAction{}})
				cCancel()
				return
			case <-ticker.C:
				if _, err := client.API().MessagesSetTyping(actCtx, &tg.MessagesSetTypingRequest{Peer: inputPeer, Action: action}); err != nil {
					return
				}
			}
		}
	}()
}

func deleteMsg(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msgID int) {
	dCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	if ch, ok := inputPeer.(*tg.InputPeerChannel); ok {
		_, _ = client.API().ChannelsDeleteMessages(dCtx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash},
			ID:      []int{msgID},
		})
		return
	}
	_, _ = client.API().MessagesDeleteMessages(dCtx, &tg.MessagesDeleteMessagesRequest{Revoke: true, ID: []int{msgID}})
}

func deleteMessageBatch(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, ids []int) {
	if len(ids) == 0 { return }
	dCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if ch, ok := inputPeer.(*tg.InputPeerChannel); ok {
		_, _ = client.API().ChannelsDeleteMessages(dCtx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash},
			ID:      ids,
		})
		return
	}
	_, _ = client.API().MessagesDeleteMessages(dCtx, &tg.MessagesDeleteMessagesRequest{Revoke: true, ID: ids})
}

func getUTF16Length(text string) int {
	return len(utf16.Encode([]rune(text)))
}

func sendTemporaryNotice(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, text string, duration time.Duration) {
	sendReq := &tg.MessagesSendMessageRequest{
		Peer:     inputPeer,
		Message:  text,
		RandomID: rand.Int63(),
		Entities: []tg.MessageEntityClass{&tg.MessageEntityBold{Offset: 0, Length: getUTF16Length(text)}},
	}
	res, err := client.API().MessagesSendMessage(ctx, sendReq)
	if err != nil { return }

	msgID := 0
	if updates, ok := res.(*tg.Updates); ok {
		for _, u := range updates.Updates {
			if nu, ok := u.(*tg.UpdateNewMessage); ok {
				if m, ok := nu.Message.(*tg.Message); ok {
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

func notifyAndSelfDestruct(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msgID int, text string) {
	editReq := &tg.MessagesEditMessageRequest{
		Peer:    inputPeer,
		ID:      msgID,
		Message: text,
		Entities: []tg.MessageEntityClass{
			&tg.MessageEntityBold{Offset: 0, Length: getUTF16Length(text)},
		},
	}
	_, _ = client.API().MessagesEditMessage(ctx, editReq)
	time.Sleep(100 * time.Millisecond)
	deleteMsg(context.Background(), client, inputPeer, msgID)
}

func getEntitiesForFont(text string, mode string) []tg.MessageEntityClass {
	length := getUTF16Length(text)
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
		return []tg.MessageEntityClass{&tg.MessageEntityBold{Offset: 0, Length: length}}
	}
}

func handleForwardToAllPV(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, dropAuthor bool) {
	if msg.ReplyTo == nil {
		if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است") }
		return
	}
	header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || header.ReplyToMsgID == 0 {
		if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است") }
		return
	}
	replyMsgID := header.ReplyToMsgID

	dialogsReq := &tg.MessagesGetDialogsRequest{OffsetPeer: &tg.InputPeerEmpty{}, Limit: 100}
	res, err := client.API().MessagesGetDialogs(ctx, dialogsReq)
	if err != nil { return }

	var users []tg.UserClass
	var dialogs []tg.DialogClass
	if d, ok := res.(*tg.MessagesDialogs); ok {
		users, dialogs = d.Users, d.Dialogs
	} else if ds, ok := res.(*tg.MessagesDialogsSlice); ok {
		users, dialogs = ds.Users, ds.Dialogs
	}

	userMap := make(map[int64]*tg.User)
	for _, uClass := range users {
		if u, ok := uClass.(*tg.User); ok { userMap[u.ID] = u }
	}

	for _, dlg := range dialogs {
		d, ok := dlg.(*tg.Dialog)
		if !ok { continue }
		peerUser, ok := d.Peer.(*tg.PeerUser)
		if !ok { continue }
		u, exists := userMap[peerUser.UserID]
		if !exists || u.Bot || u.Self || u.Deleted { continue }

		fwdReq := &tg.MessagesForwardMessagesRequest{
			DropAuthor: dropAuthor,
			FromPeer:   inputPeer,
			ID:         []int{replyMsgID},
			RandomID:   []int64{rand.Int63()},
			ToPeer:     &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash},
		}
		_, _ = client.API().MessagesForwardMessages(ctx, fwdReq)
		time.Sleep(80 * time.Millisecond)
	}
	if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "انجام شد") }
}

func handleForwardToAllGroups(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msg *tg.Message, dropAuthor bool) {
	if msg.ReplyTo == nil {
		if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است") }
		return
	}
	header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || header.ReplyToMsgID == 0 {
		if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است") }
		return
	}
	replyMsgID := header.ReplyToMsgID

	dialogsReq := &tg.MessagesGetDialogsRequest{OffsetPeer: &tg.InputPeerEmpty{}, Limit: 100}
	res, err := client.API().MessagesGetDialogs(ctx, dialogsReq)
	if err != nil { return }

	var chats []tg.ChatClass
	var dialogs []tg.DialogClass
	if d, ok := res.(*tg.MessagesDialogs); ok {
		chats, dialogs = d.Chats, d.Dialogs
	} else if ds, ok := res.(*tg.MessagesDialogsSlice); ok {
		chats, dialogs = ds.Chats, ds.Dialogs
	}

	chatMap := make(map[int64]tg.ChatClass)
	for _, cClass := range chats {
		switch ch := cClass.(type) {
		case *tg.Chat: chatMap[ch.ID] = ch
		case *tg.Channel: chatMap[ch.ID] = ch
		}
	}

	for _, dlg := range dialogs {
		d, ok := dlg.(*tg.Dialog)
		if !ok { continue }
		var targetPeer tg.InputPeerClass
		switch p := d.Peer.(type) {
		case *tg.PeerChat: targetPeer = &tg.InputPeerChat{ChatID: p.ChatID}
		case *tg.PeerChannel:
			if chObj, exists := chatMap[p.ChannelID]; exists {
				if ch, ok := chObj.(*tg.Channel); ok && !ch.Broadcast {
					targetPeer = &tg.InputPeerChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash}
				}
			}
		}
		if targetPeer == nil { continue }
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
	if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "انجام شد") }
}

func handlePurgeAction(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, cmdMsgID int, fromReplyID int, countLimit int) {
	pCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var toDelete []int
	offsetID := cmdMsgID

	for {
		res, err := client.API().MessagesGetHistory(pCtx, &tg.MessagesGetHistoryRequest{Peer: inputPeer, OffsetID: offsetID, Limit: 100})
		if err != nil { break }

		var messages []tg.MessageClass
		switch h := res.(type) {
		case *tg.MessagesMessages: messages = h.Messages
		case *tg.MessagesMessagesSlice: messages = h.Messages
		case *tg.MessagesChannelMessages: messages = h.Messages
		}
		if len(messages) == 0 { break }

		stopSearch := false
		for _, mClass := range messages {
			m, ok := mClass.(*tg.Message)
			if !ok { continue }
			if fromReplyID > 0 {
				if m.ID < fromReplyID { stopSearch = true; break }
				if m.Out { toDelete = append(toDelete, m.ID) }
				if m.ID == fromReplyID { stopSearch = true; break }
			} else {
				if m.Out {
					toDelete = append(toDelete, m.ID)
					if len(toDelete) >= countLimit { stopSearch = true; break }
				}
			}
		}
		if stopSearch || len(messages) < 100 { break }
		if lastMsg, ok := messages[len(messages)-1].(*tg.Message); ok {
			offsetID = lastMsg.ID
		} else { break }
	}

	for i := 0; i < len(toDelete); i += 100 {
		end := i + 100
		if end > len(toDelete) { end = len(toDelete) }
		deleteMessageBatch(pCtx, client, inputPeer, toDelete[i:end])
		time.Sleep(100 * time.Millisecond)
	}

	reportText := fmt.Sprintf("🗑 %d پیام شما پاکسازی شد", len(toDelete))
	if len(toDelete) == 0 { reportText = "⚠️ پیامی برای پاکسازی یافت نشد" }
	sendTemporaryNotice(pCtx, client, inputPeer, reportText, 1500*time.Millisecond)
}

func toBoldDigits(t string) string {
	boldDigits := map[rune]string{'0': "𝟎", '1': "𝟏", '2': "𝟐", '3': "𝟑", '4': "𝟒", '5': "𝟓", '6': "𝟔", '7': "𝟕", '8': "𝟖", '9': "𝟗", ':': ":"}
	var sb strings.Builder
	for _, r := range t {
		if b, ok := boldDigits[r]; ok { sb.WriteString(b) } else { sb.WriteRune(r) }
	}
	return sb.String()
}

func getTehranBoldTime() string {
	loc := getTehranLocation()
	return toBoldDigits(time.Now().In(loc).Format("15:04"))
}

func handleClockOn(ctx context.Context, userID int64, client *telegram.Client) {
	var isEnabled bool
	var origLast string
	_ = db.QueryRow("SELECT is_clock_enabled, original_last_name FROM users WHERE id = ?", userID).Scan(&isEnabled, &origLast)
	if !isEnabled || origLast == "" {
		if self, err := client.Self(ctx); err == nil {
			origLast = self.LastName
			_, _ = db.Exec("UPDATE users SET original_last_name = ? WHERE id = ?", origLast, userID)
		}
	}
	req := &tg.AccountUpdateProfileRequest{}
	req.SetLastName(getTehranBoldTime())
	if _, err := client.API().AccountUpdateProfile(ctx, req); err == nil {
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
		if self, err := client.Self(ctx); err == nil {
			origFirst = cleanName(self.FirstName)
			_, _ = db.Exec("UPDATE users SET original_first_name = ? WHERE id = ?", origFirst, userID)
		}
	}
	req := &tg.AccountUpdateProfileRequest{}
	req.SetFirstName(fmt.Sprintf("%s %s", origFirst, getRandomEmoji()))
	if _, err := client.API().AccountUpdateProfile(ctx, req); err == nil {
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
		if full, err := client.API().UsersGetFullUser(ctx, &tg.InputUserSelf{}); err == nil {
			origBio = full.FullUser.About
			_, _ = db.Exec("UPDATE users SET original_bio = ? WHERE id = ?", origBio, userID)
		}
	}
	req := &tg.AccountUpdateProfileRequest{}
	req.SetAbout(getRandomBio())
	if _, err := client.API().AccountUpdateProfile(ctx, req); err == nil {
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
	req := &tg.AccountUpdateProfileRequest{}
	req.SetAbout(getRandomBio())
	if _, err := client.API().AccountUpdateProfile(ctx, req); err == nil {
		_, _ = db.Exec("UPDATE users SET is_bio_enabled = TRUE, bio_mode = 'random' WHERE id = ?", userID)
	}
}

func handleBioCustom(ctx context.Context, userID int64, client *telegram.Client, customBio string) {
	var origBio string
	_ = db.QueryRow("SELECT original_bio FROM users WHERE id = ?", userID).Scan(&origBio)
	if origBio == "" {
		if full, err := client.API().UsersGetFullUser(ctx, &tg.InputUserSelf{}); err == nil {
			origBio = full.FullUser.About
			_, _ = db.Exec("UPDATE users SET original_bio = ? WHERE id = ?", origBio, userID)
		}
	}
	runes := []rune(customBio)
	if len(runes) > 70 { customBio = string(runes[:70]) }
	req := &tg.AccountUpdateProfileRequest{}
	req.SetAbout(customBio)
	if _, err := client.API().AccountUpdateProfile(ctx, req); err == nil {
		_, _ = db.Exec("UPDATE users SET is_bio_enabled = TRUE, bio_mode = 'custom', custom_bio = ? WHERE id = ?", customBio, userID)
	}
}

func getInputPeer(peer tg.PeerClass, e tg.Entities, selfID int64) tg.InputPeerClass {
	if peer == nil { return nil }
	switch p := peer.(type) {
	case *tg.PeerUser:
		if p.UserID == selfID { return &tg.InputPeerSelf{} }
		if u, ok := e.Users[p.UserID]; ok {
			return &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash}
		}
		return &tg.InputPeerUser{UserID: p.UserID}
	case *tg.PeerChat: return &tg.InputPeerChat{ChatID: p.ChatID}
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
		return &tg.InputPeerChannel{ChannelID: p.ChannelID, AccessHash: aHash}
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
	if err != nil { return nil, nil, err }

	var msgs []tg.MessageClass
	var users []tg.UserClass
	switch m := res.(type) {
	case *tg.MessagesChannelMessages: msgs, users = m.Messages, m.Users
	case *tg.MessagesMessages: msgs, users = m.Messages, m.Users
	case *tg.MessagesMessagesSlice: msgs, users = m.Messages, m.Users
	}
	if len(msgs) > 0 {
		if msg, ok := msgs[0].(*tg.Message); ok { return msg, users, nil }
	}
	return nil, nil, errors.New("not found")
}

func GetSetting(key string) string {
	var val string
	_ = db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = ?", key).Scan(&val)
	return val
}

func SetSetting(key, val string) {
	_, _ = db.Exec("INSERT INTO settings (setting_key, setting_value) VALUES (?, ?) ON DUPLICATE KEY UPDATE setting_value = ?", key, val, val)
}

func SaveUser(userID int64, firstName, username string) {
	if db == nil { return }
	_, _ = db.Exec(`INSERT INTO users (id, first_name, username) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE first_name=?, username=?`, userID, firstName, username, firstName, username)
	_, _ = db.Exec(`INSERT IGNORE INTO wallets (user_id, balance) VALUES (?, 0)`, userID)
}

func GetUserBalance(userID int64) int {
	var balance int
	_ = db.QueryRow("SELECT balance FROM wallets WHERE user_id = ?", userID).Scan(&balance)
	return balance
}

func IsUserBlocked(userID int64) bool {
	var blocked bool
	_ = db.QueryRow("SELECT is_blocked FROM users WHERE id = ?", userID).Scan(&blocked)
	return blocked
}

func GetUserSelfStatus(userID int64) string {
	var status string
	_ = db.QueryRow("SELECT self_status FROM users WHERE id = ?", userID).Scan(&status)
	if status == "" { return "خرید نداشته" }
	return status
}

func SafeAddUserBalance(userID int64, amount int) error {
	tx, err := db.Begin()
	if err != nil { return err }
	defer tx.Rollback()
	_, _ = tx.Exec(`INSERT IGNORE INTO users (id, first_name, username) VALUES (?, 'کاربر', 'ثبت_نشده')`, userID)
	_, _ = tx.Exec(`INSERT INTO wallets (user_id, balance) VALUES (?, ?) ON DUPLICATE KEY UPDATE balance = balance + ?`, userID, amount, amount)
	_, _ = tx.Exec(`UPDATE users SET purchases_count = purchases_count + 1 WHERE id = ?`, userID)
	return tx.Commit()
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit { return fmt.Sprintf("%d B", b) }
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func getAdminDashboard() string {
	var totalUsers, activeUsers, blockedUsers int
	_ = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE is_blocked = TRUE").Scan(&blockedUsers)
	_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE self_status = 'روشن'").Scan(&activeUsers)
	inactiveUsers := totalUsers - activeUsers
	adminCount := len(cfg.AdminIDs)

	var cpuUsage float64
	if c, err := cpu.Percent(0, false); err == nil && len(c) > 0 {
		cpuUsage = c[0]
	}

	var ramUsed, ramTotal uint64
	var ramPercent float64
	if v, err := mem.VirtualMemory(); err == nil {
		ramUsed = v.Used
		ramTotal = v.Total
		ramPercent = v.UsedPercent
	}

	var swapUsed, swapTotal uint64
	var swapPercent float64
	if s, err := mem.SwapMemory(); err == nil {
		swapUsed = s.Used
		swapTotal = s.Total
		swapPercent = s.UsedPercent
	}

	var diskUsed, diskTotal uint64
	var diskPercent float64
	if d, err := disk.Usage("/"); err == nil {
		diskUsed = d.Used
		diskTotal = d.Total
		diskPercent = d.UsedPercent
	}

	var totalUp, totalDown uint64
	if nv, err := net.IOCounters(false); err == nil && len(nv) > 0 {
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
	})

	activeUserbots[userID] = &UserbotSession{UserID: userID, Client: client, Cancel: cancel}
	activeUserbotsMu.Unlock()

	RegisterWolfPlusDispatcher(&dispatcher, client, userID)

	handleMsg := func(ctx context.Context, e tg.Entities, message tg.MessageClass) {
		msg, ok := message.(*tg.Message)
		if !ok { return }

		for cid, ch := range e.Channels {
			channelAccessHashesMu.Lock()
			channelAccessHashes[cid] = ch.AccessHash
			channelAccessHashesMu.Unlock()
		}

		WolfPlusHandleIncoming(ctx, client, bot, userID, msg, e)

		selfID := int64(0)
		if self, err := client.Self(ctx); err == nil { selfID = self.ID }
		inputPeer := getInputPeer(msg.PeerID, e, selfID)

		peerKey := "chat"
		switch p := msg.PeerID.(type) {
		case *tg.PeerUser: peerKey = fmt.Sprintf("user_%d", p.UserID)
		case *tg.PeerChat: peerKey = fmt.Sprintf("chat_%d", p.ChatID)
		case *tg.PeerChannel: peerKey = fmt.Sprintf("channel_%d", p.ChannelID)
		}

		// پردازش قفل پیوی (از wolfplus.go)
		if ProcessPVLockIncoming(ctx, client, userID, msg, e) { return }

		// واکنش به پیام‌های دیگران
		if !msg.Out {
			senderID := int64(0)
			if fromUser, ok := msg.FromID.(*tg.PeerUser); ok { senderID = fromUser.UserID }
			if senderID != 0 && senderID != selfID && senderID != 777000 {
				if emoji, exists := getAutoReact(userID, senderID); exists && inputPeer != nil {
					go func(mID int, p tg.InputPeerClass, em string) {
						time.Sleep(200 * time.Millisecond)
						rCtx, rCancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer rCancel()
						_, _ = client.API().MessagesSendReaction(rCtx, &tg.MessagesSendReactionRequest{Peer: p, MsgID: mID, Reaction: []tg.ReactionClass{&tg.ReactionEmoji{Emoticon: em}}})
					}(msg.ID, inputPeer, emoji)
				}
				if isUserFriend(userID, senderID) {
					go func(mID int, p tg.InputPeerClass) {
						time.Sleep(150 * time.Millisecond)
						rCtx, rCancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer rCancel()
						req := &tg.MessagesSendMessageRequest{Peer: p, Message: GetRandomFriendMessage(), RandomID: rand.Int63()}
						req.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: mID})
						_, _ = client.API().MessagesSendMessage(rCtx, req)
					}(msg.ID, inputPeer)
				}
				if isUserEnemy(userID, senderID) {
					go func(mID int, p tg.InputPeerClass) {
						time.Sleep(150 * time.Millisecond)
						rCtx, rCancel := context.WithTimeout(context.Background(), 10*time.Second)
						defer rCancel()
						req := &tg.MessagesSendMessageRequest{Peer: p, Message: GetRandomEnemyMessage(), RandomID: rand.Int63()}
						req.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: mID})
						_, _ = client.API().MessagesSendMessage(rCtx, req)
					}(msg.ID, inputPeer)
				}
			}
			return
		}

		text := strings.TrimSpace(msg.Message)

		// پردازش مترجم (از guide.go) و قفل پیوی (از wolfplus.go)
		if ProcessLiveTranslator(ctx, client, inputPeer, msg, text) { return }
		if ProcessPVLockCommand(ctx, client, inputPeer, msg, text, userID) { return }

		// دستورات ری اکشن
		if text == "ری اکشن" || strings.HasPrefix(text, "ری اکشن ") {
			if msg.ReplyTo == nil {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ روی پیام فرد ریپلای کنید!") }
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 { return }
			emoji := "❤️"
			if strings.HasPrefix(text, "ری اکشن ") {
				if em := strings.TrimSpace(strings.TrimPrefix(text, "ری اکشن ")); em != "" { emoji = em }
			}
			go func(repID int, p tg.InputPeerClass, mID int, em string) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()
				repMsg, usersList, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil { return }
				var tUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok { tUID = f.UserID }
				if tUID == 0 || tUID == selfID { return }

				tUName := fmt.Sprintf("کاربر (%d)", tUID)
				for _, uClass := range usersList {
					if u, ok := uClass.(*tg.User); ok && u.ID == tUID { tUName = formatTelegramUser(u); break }
				}
				_, _ = db.Exec(`INSERT INTO wolf_auto_reacts (owner_id, target_id, target_name, emoji) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE target_name = VALUES(target_name), emoji = VALUES(emoji)`, userID, tUID, tUName, em)
				setAutoReactToCache(userID, tUID, em)
				go notifyAndSelfDestruct(dCtx, client, p, mID, fmt.Sprintf("✅ ری اکشن %s فعال شد", em))
			}(header.ReplyToMsgID, inputPeer, msg.ID, emoji)
			return
		} else if text == "حذف ری اکشن" {
			if msg.ReplyTo == nil { return }
			if header, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && header.ReplyToMsgID != 0 {
				go func(repID int, p tg.InputPeerClass, mID int) {
					dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer dCancel()
					if repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID); err == nil && repMsg != nil {
						if f, ok := repMsg.FromID.(*tg.PeerUser); ok {
							_, _ = db.Exec("DELETE FROM wolf_auto_reacts WHERE owner_id = ? AND target_id = ?", userID, f.UserID)
							removeAutoReactFromCache(userID, f.UserID)
							go notifyAndSelfDestruct(dCtx, client, p, mID, "✅ ری اکشن حذف شد")
						}
					}
				}(header.ReplyToMsgID, inputPeer, msg.ID)
			}
			return
		} else if text == "پاکسازی ری اکشن" {
			_, _ = db.Exec("DELETE FROM wolf_auto_reacts WHERE owner_id = ?", userID)
			clearAutoReactsCache(userID)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🗑 لیست ری اکشن پاکسازی شد") }
			return
		}

		// تایمر
		if strings.HasPrefix(text, "تایمر ") || strings.HasPrefix(text, "شمارش ") {
			pfx := "تایمر "
			if strings.HasPrefix(text, "شمارش ") { pfx = "شمارش " }
			count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(text, pfx)))
			if err == nil && count > 0 && count <= 60 {
				go func(p tg.InputPeerClass, mID int, cnt int) {
					for i := cnt; i > 0; i-- {
						eCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						msgText := fmt.Sprintf("⏳ %d", i)
						_, _ = client.API().MessagesEditMessage(eCtx, &tg.MessagesEditMessageRequest{
							Peer: p, ID: mID, Message: msgText, Entities: []tg.MessageEntityClass{&tg.MessageEntityBold{Offset: 0, Length: getUTF16Length(msgText)}},
						})
						cancel()
						time.Sleep(1 * time.Second)
					}
					eCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					_, _ = client.API().MessagesEditMessage(eCtx, &tg.MessagesEditMessageRequest{Peer: p, ID: mID, Message: "💥"})
					cancel()
				}(inputPeer, msg.ID, count)
				return
			}
		}

		// پاکشو
		if text == "پاکشو" || strings.HasPrefix(text, "پاکشو ") {
			if msg.ReplyTo != nil {
				if header, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && header.ReplyToMsgID != 0 {
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🗑 در حال پاکسازی...")
					go handlePurgeAction(ctx, client, inputPeer, msg.ID, header.ReplyToMsgID, 0)
					return
				}
			}
			numStr := strings.TrimSpace(strings.TrimPrefix(text, "پاکشو"))
			if count, err := strconv.Atoi(numStr); err == nil && count > 0 {
				if count > 100 { count = 100 }
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, fmt.Sprintf("🗑 حذف %d پیام...", count))
				go handlePurgeAction(ctx, client, inputPeer, msg.ID, 0, count)
				return
			}
		}

		// اکشن‌ها
		if text == "لغو اکشن" || text == "توقف اکشن" {
			stopActiveAction(fmt.Sprintf("%d_%s", userID, peerKey))
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🛑 اکشن متوقف شد") }
			return
		}

		// دوست و دشمن
		if text == "تنظیم دوست" && msg.ReplyTo != nil {
			if header, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && header.ReplyToMsgID != 0 {
				go func(repID int, p tg.InputPeerClass, mID int) {
					dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer dCancel()
					if repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID); err == nil && repMsg != nil {
						if f, ok := repMsg.FromID.(*tg.PeerUser); ok && f.UserID != selfID {
							_, _ = db.Exec(`INSERT INTO wolf_friends (owner_id, friend_id) VALUES (?, ?) ON DUPLICATE KEY UPDATE friend_id=friend_id`, userID, f.UserID)
							addFriendToCache(userID, f.UserID)
							go notifyAndSelfDestruct(dCtx, client, p, mID, "✅ دوست تنظیم شد")
						}
					}
				}(header.ReplyToMsgID, inputPeer, msg.ID)
			}
			return
		} else if text == "تنظیم دشمن" && msg.ReplyTo != nil {
			if header, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && header.ReplyToMsgID != 0 {
				go func(repID int, p tg.InputPeerClass, mID int) {
					dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer dCancel()
					if repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID); err == nil && repMsg != nil {
						if f, ok := repMsg.FromID.(*tg.PeerUser); ok && f.UserID != selfID {
							_, _ = db.Exec(`INSERT INTO wolf_enemies (owner_id, enemy_id) VALUES (?, ?) ON DUPLICATE KEY UPDATE enemy_id=enemy_id`, userID, f.UserID)
							addEnemyToCache(userID, f.UserID)
							go notifyAndSelfDestruct(dCtx, client, p, mID, "⚔️ دشمن تنظیم شد")
						}
					}
				}(header.ReplyToMsgID, inputPeer, msg.ID)
			}
			return
		}

		// خوشنویسی چت
		if text == "خوشنویسی روشن" {
			_, _ = db.Exec("UPDATE users SET is_font_enabled = TRUE WHERE id = ?", userID)
			_, m := getFontSetting(userID)
			updateFontCache(userID, true, m)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🟢 خوشنویسی روشن شد") }
			return
		} else if text == "خوشنویسی خاموش" {
			_, _ = db.Exec("UPDATE users SET is_font_enabled = FALSE WHERE id = ?", userID)
			_, m := getFontSetting(userID)
			updateFontCache(userID, false, m)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🔴 خوشنویسی خاموش شد") }
			return
		}

		// ساعت و اموجی و بیو
		if text == "ساعت روشن شو" || text == "ساعت روشن" {
			handleClockOn(ctx, userID, client)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "ساعت روشن شد") }
			return
		} else if text == "ساعت خاموش شو" || text == "ساعت خاموش" {
			handleClockOff(ctx, userID, client)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "ساعت خاموش شد") }
			return
		} else if text == "اموجی روشن شو" || text == "اموجی روشن" {
			handleEmojiOn(ctx, userID, client)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "اموجی روشن شد") }
			return
		} else if text == "اموجی خاموش شو" || text == "اموجی خاموش" {
			handleEmojiOff(ctx, userID, client)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "اموجی خاموش شد") }
			return
		}

		// فوروارد همگانی
		if text == "بفرست پیوی همه" {
			go handleForwardToAllPV(context.Background(), client, inputPeer, msg, true)
			return
		} else if text == "بفرست گروه همه" {
			go handleForwardToAllGroups(context.Background(), client, inputPeer, msg, true)
			return
		}

		// اعمال فونت خروجی
		fontEnabled, fontMode := getFontSetting(userID)
		if fontEnabled && text != "" && msg.Media == nil {
			go func(p tg.InputPeerClass, mID int, origText string, fMode string) {
				eCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
				defer c()
				_, _ = client.API().MessagesEditMessage(eCtx, &tg.MessagesEditMessageRequest{
					Peer: p, ID: mID, Message: origText, Entities: getEntitiesForFont(origText, fMode),
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
		_ = client.Run(ctx, func(ctx context.Context) error {
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
		activeUserbotsMu.Lock()
		delete(activeUserbots, userID)
		activeUserbotsMu.Unlock()
	}()
}

func stopUserbot(userID int64) {
	activeUserbotsMu.Lock()
	defer activeUserbotsMu.Unlock()
	if ub, exists := activeUserbots[userID]; exists {
		if ub.Cancel != nil { ub.Cancel() }
		delete(activeUserbots, userID)
	}
}

func updateClocks() {
	if db == nil { return }
	rows, err := db.Query("SELECT id FROM users WHERE self_status = 'روشن' AND is_clock_enabled = TRUE")
	if err != nil { return }
	var uids []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err == nil { uids = append(uids, uid) }
	}
	rows.Close()
	t := getTehranBoldTime()
	activeUserbotsMu.RLock()
	for _, uid := range uids {
		if ub, ok := activeUserbots[uid]; ok && ub.Client != nil {
			go func(cl *telegram.Client) {
				cTimeout, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				req := &tg.AccountUpdateProfileRequest{}
				req.SetLastName(t)
				_, _ = cl.API().AccountUpdateProfile(cTimeout, req)
			}(ub.Client)
		}
	}
	activeUserbotsMu.RUnlock()
}

func startClockWorker() {
	go func() {
		time.Sleep(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)))
		updateClocks()
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C { updateClocks() }
	}()
}

func startBillingWorker(bot *tele.Bot) {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		for range ticker.C {
			if db == nil { continue }
			rows, err := db.Query(`SELECT id FROM users WHERE self_status = 'روشن' AND (last_billed_at IS NULL OR last_billed_at <= DATE_SUB(NOW(), INTERVAL 24 HOUR))`)
			if err != nil { continue }
			var userIDs []int64
			for rows.Next() {
				var uid int64
				if err := rows.Scan(&uid); err == nil { userIDs = append(userIDs, uid) }
			}
			rows.Close()
			keyPrice := getKeyPrice()
			for _, uid := range userIDs {
				bal := GetUserBalance(uid)
				if bal < keyPrice {
					_, _ = db.Exec("UPDATE users SET self_status = 'خاموش' WHERE id = ?", uid)
					stopUserbot(uid)
					_, _ = bot.Send(&tele.User{ID: uid}, "⚠️ شارژ کلیدهای شما به پایان رسید و سلف خاموش شد.")
				} else {
					if _, err := db.Exec(`UPDATE wallets SET balance = balance - ? WHERE user_id = ? AND balance >= ?`, keyPrice, uid, keyPrice); err == nil {
						_, _ = db.Exec("UPDATE users SET last_billed_at = NOW() WHERE id = ?", uid)
					}
				}
			}
		}
	}()
}

type botAuthenticator struct {
	phone        string
	codeChan     chan string
	passwordChan chan string
	resultChan   chan AuthResult
}

func (b *botAuthenticator) Phone(ctx context.Context) (string, error) { return b.phone, nil }
func (b *botAuthenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	select {
	case code, ok := <-b.codeChan:
		if !ok { return "", errors.New("canceled") }
		return code, nil
	case <-ctx.Done(): return "", ctx.Err()
	}
}
func (b *botAuthenticator) Password(ctx context.Context) (string, error) {
	b.resultChan <- AuthResult{Type: AuthResultNeeds2FA}
	select {
	case pwd, ok := <-b.passwordChan:
		if !ok { return "", errors.New("canceled") }
		return pwd, nil
	case <-ctx.Done(): return "", ctx.Err()
	}
}
func (b *botAuthenticator) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error { return nil }
func (b *botAuthenticator) SignUp(ctx context.Context) (auth.UserInfo, error) { return auth.UserInfo{}, errors.New("unsupported") }

func startTelegramLogin(ctx context.Context, userID int64, cfg Config, authHandler *botAuthenticator) {
	sessionDir := "/opt/wolf/sessions"
	_ = os.MkdirAll(sessionDir, 0700)
	sessionPath := filepath.Join(sessionDir, fmt.Sprintf("user_%d.json", userID))
	loader := &session.FileStorage{Path: sessionPath}
	client := telegram.NewClient(cfg.APIID, cfg.APIHash, telegram.Options{SessionStorage: loader})
	flow := auth.NewFlow(authHandler, auth.SendCodeOptions{})

	err := client.Run(ctx, func(ctx context.Context) error { return client.Auth().IfNecessary(ctx, flow) })
	if err != nil {
		authHandler.resultChan <- AuthResult{Type: AuthResultFailed, Error: err}
	} else {
		authHandler.resultChan <- AuthResult{Type: AuthResultSuccess}
	}
}

func toPersianDigits(s string) string {
	persianDigits := []string{"۰", "۱", "۲", "۳", "۴", "۵", "۶", "۷", "۸", "۹"}
	for i, d := range persianDigits { s = strings.ReplaceAll(s, strconv.Itoa(i), d) }
	return s
}

func extractDigits(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) { sb.WriteRune(r) }
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

func getKeyPrice() int {
	price, err := strconv.Atoi(GetSetting("key_price"))
	if err != nil || price <= 0 { return 3333 }
	return price
}

func getTehranLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil { return time.FixedZone("Asia/Tehran", 12600) }
	return loc
}

func getMainKeyboard(userID int64) *tele.ReplyMarkup {
	if cfg.IsAdmin(userID) { return adminMenu }
	return userMenu
}

func getKeyboard(userID int64) *tele.ReplyMarkup {
	return getMainKeyboard(userID)
}

func main() {
	cfg = loadConfig()
	if parts := strings.Split(cfg.BotToken, ":"); len(parts) > 0 {
		controllerBotID, _ = strconv.ParseInt(parts[0], 10, 64)
	}

	InitDB(cfg)
	defer db.Close()
	InitWolfPlusDB()

	bot, err := tele.NewBot(tele.Settings{
		Token:  cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Fatalf("❌ خطا در راه‌اندازی ربات: %v", err)
	}

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

	profileMenu.Reply(
		profileMenu.Row(btnTurnOnSelf, btnTurnOffSelf),
		profileMenu.Row(btnExitSelf),
		profileMenu.Row(btnBack),
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

	// اتصال ماژول‌ها
	RegisterWalletHandlers(bot)
	RegisterGuideHandlers(bot)
	RegisterWolfPlusHandlers(bot)

	// منوهای اصلی
	bot.Handle("/start", func(c tele.Context) error {
		SaveUser(c.Sender().ID, c.Sender().FirstName, c.Sender().Username)
		return c.Send(fmt.Sprintf("👑 <b>به ربات ولف سلف خوش آمدید!</b>\n\n👤 %s\n🆔 <code>%d</code>", html.EscapeString(c.Sender().FirstName), c.Sender().ID), getKeyboard(c.Sender().ID), tele.ModeHTML)
	})

	bot.Handle(&btnBack, func(c tele.Context) error {
		return c.Send("🔙 <b>منوی اصلی:</b>", getKeyboard(c.Sender().ID), tele.ModeHTML)
	})

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
		if daysActive < 1 { daysActive = 1 }

		statusIcon := "❌"
		if selfStatus == "روشن" {
			statusIcon = "✅"
		} else if selfStatus == "خاموش" {
			statusIcon = "⏸️"
		}

		text := fmt.Sprintf("💙 تاریخ امروز: %s\n\n⏰ ساعت: %s\n\n🔒 اطلاعات حساب کاربری\n\n⭐ آیدی عددی: <code>%d</code>\n📅 تاریخ عضویت در ربات: %s\n👀 فعالیت در ربات: %d روز\n💰 موجودی: %s تومان\n🔥 وضعیت سلف: %s %s",
			toPersianDigits(tNow.Format("yyyy/MM/dd")), toPersianDigits(tNow.Format("HH:mm:ss")), user.ID, toPersianDigits(tJoined.Format("yyyy/MM/dd")), daysActive, formatMoney(GetUserBalance(user.ID)), statusIcon, selfStatus)

		return c.Send(text, profileMenu, tele.ModeHTML)
	})

	bot.Handle(&btnWolfPlus, func(c tele.Context) error {
		if GetUserSelfStatus(c.Sender().ID) == "خرید نداشته" {
			return c.Send("❌ اشتراک سلف فعال نیست.")
		}
		return c.Send(buildWolfPlusDashboardText(c.Sender().ID), wolfPlusMenu, tele.ModeHTML)
	})

	bot.Handle(&btnSupport, func(c tele.Context) error {
		return c.Send(fmt.Sprintf("%s\n\n🆔 %s", GetSetting("support_text"), GetSetting("support_id")), tele.ModeHTML)
	})

	// بخش مدیریت و تنظیمات ادمین
	bot.Handle(&btnAdminPanel, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) {
			return c.Send("❌ شما دسترسی ندارید.")
		}
		return c.Send(getAdminDashboard(), adminPanelMenu, tele.ModeHTML)
	})

	backToAdminHandler := func(c tele.Context) error {
		stateMu.Lock()
		delete(adminStates, c.Sender().ID)
		stateMu.Unlock()
		return c.Send(getAdminDashboard(), adminPanelMenu, tele.ModeHTML)
	}
	bot.Handle(&btnBackToAdminAcc, backToAdminHandler)
	bot.Handle(&btnBackToAdminSup, backToAdminHandler)

	bot.Handle(&btnConfigAccount, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		return c.Send("💳 تنظیمات اطلاعات بانکی:", accountConfigMenu, tele.ModeHTML)
	})

	bot.Handle(&btnConfigCardNum, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_card_num"}
		stateMu.Unlock()
		return c.Send("✏️ شماره کارت جدید را ارسال کنید:", tele.ModeHTML)
	})

	bot.Handle(&btnConfigCardName, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_card_name"}
		stateMu.Unlock()
		return c.Send("✏️ نام دارنده حساب جدید را ارسال کنید:", tele.ModeHTML)
	})

	bot.Handle(&btnConfigCardBank, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_card_bank"}
		stateMu.Unlock()
		return c.Send("✏️ نام بانک جدید را ارسال کنید:", tele.ModeHTML)
	})

	bot.Handle(&btnConfigSupport, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		return c.Send("📞 تنظیمات پشتیبانی:", supportConfigMenu, tele.ModeHTML)
	})

	bot.Handle(&btnConfigSupportText, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_support_text"}
		stateMu.Unlock()
		return c.Send("✏️ متن جدید پشتیبانی را ارسال کنید:", tele.ModeHTML)
	})

	bot.Handle(&btnConfigSupportID, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_support_id"}
		stateMu.Unlock()
		return c.Send("✏️ آیدی جدید پشتیبانی (مثال: @YourID) را ارسال کنید:", tele.ModeHTML)
	})

	bot.Handle(&btnConfigKeyPrice, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_key_price"}
		stateMu.Unlock()
		return c.Send("✏️ مبلغ جدید هر کلید را به تومان ارسال کنید:", tele.ModeHTML)
	})

	// کنترل وضعیت روشن/خاموش سلف
	bot.Handle(&btnTurnOnSelf, func(c tele.Context) error {
		userID := c.Sender().ID
		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "روشن" {
			return c.Send("⚠️ سلف در حال حاضر روشن است.", getKeyboard(userID), tele.ModeHTML)
		}
		_, statErr := os.Stat(fmt.Sprintf("/opt/wolf/sessions/user_%d.json", userID))
		if statErr == nil {
			_, _ = db.Exec("UPDATE users SET self_status = 'روشن' WHERE id = ?", userID)
			startUserbot(userID, cfg, bot)
			return c.Send("🟢 سلف شما روشن شد.", getKeyboard(userID), tele.ModeHTML)
		}
		return c.Send("❌ سلف فعالی یافت نشد. از بخش خرید سلف اقدام کنید.", getKeyboard(userID), tele.ModeHTML)
	})

	bot.Handle(&btnTurnOffSelf, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET self_status = 'خاموش' WHERE id = ?", userID)
		stopUserbot(userID)
		return c.Send("🔴 سلف خاموش شد.", getKeyboard(userID), tele.ModeHTML)
	})

	bot.Handle(&btnExitSelf, func(c tele.Context) error {
		exitMenu := &tele.ReplyMarkup{}
		exitMenu.Inline(exitMenu.Row(exitMenu.Data("🛑 تایید خروج", "exit_confirm"), exitMenu.Data("❌ لغو", "exit_cancel")))
		return c.Send("⚠️ از خروج اطمینان دارید؟ نشست حذف خواهد شد.", exitMenu, tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "exit_confirm"}, func(c tele.Context) error {
		userID := c.Sender().ID
		stopUserbot(userID)
		_ = os.Remove(fmt.Sprintf("/opt/wolf/sessions/user_%d.json", userID))
		_, _ = db.Exec("UPDATE users SET self_status = 'خروج' WHERE id = ?", userID)
		if c.Message() != nil { _ = bot.Delete(c.Message()) }
		return c.Send("🛑 سلف شما حذف شد.", getKeyboard(userID), tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "exit_cancel"}, func(c tele.Context) error {
		if c.Message() != nil { _ = bot.Delete(c.Message()) }
		return c.Send("✅ خروج لغو شد.", getKeyboard(c.Sender().ID), tele.ModeHTML)
	})

	bot.Handle(&btnConfirmSelfAction, func(c tele.Context) error {
		userID := c.Sender().ID
		stateMu.Lock()
		userStates[userID] = &UserState{Action: "waiting_for_contact"}
		stateMu.Unlock()

		shareMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
		shareMenu.Reply(shareMenu.Row(shareMenu.Contact("📱 ارسال شماره اکانت (Share Contact)"), shareMenu.Text("🔙 بازگشت")))
		return c.Send("📞 شماره خود را از دکمه زیر ارسال کنید:", shareMenu, tele.ModeHTML)
	})

	// لاگین تلگرام
	bot.Handle(tele.OnContact, func(c tele.Context) error {
		userID := c.Sender().ID
		contact := c.Message().Contact
		if contact.UserID != userID { return c.Send("❌ فقط شماره خودتان را بفرستید.") }

		codeChan := make(chan string, 1)
		passwordChan := make(chan string, 1)
		resultChan := make(chan AuthResult, 1)
		ctx, cancel := context.WithCancel(context.Background())

		authHandler := &botAuthenticator{phone: contact.PhoneNumber, codeChan: codeChan, passwordChan: passwordChan, resultChan: resultChan}
		stateMu.Lock()
		userStates[userID] = &UserState{Action: "waiting_for_code", Phone: contact.PhoneNumber, CodeChan: codeChan, PasswordChan: passwordChan, ResultChan: resultChan, Cancel: cancel}
		stateMu.Unlock()

		go startTelegramLogin(ctx, userID, cfg, authHandler)
		return c.Send("📲 کد تایید تلگرام را با فاصله ارسال فرمایید (مثال: <code>1 2 3 4 5</code>):", tele.ModeHTML)
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		if HandleWolfPlusText(c) { return nil }
		userID := c.Sender().ID
		text := strings.TrimSpace(c.Text())

		stateMu.RLock()
		uState, hasState := userStates[userID]
		stateMu.RUnlock()

		if hasState && uState != nil {
			if uState.Action == "waiting_for_code" {
				cleanCode := extractDigits(text)
				if len(cleanCode) < 5 {
					return c.Send("❌ کد ۵ رقمی را با فاصله بفرستید:")
				}
				select {
				case uState.CodeChan <- cleanCode:
					select {
					case res := <-uState.ResultChan:
						if res.Type == AuthResultNeeds2FA {
							stateMu.Lock()
							uState.Action = "waiting_for_password"
							stateMu.Unlock()
							return c.Send("🔒 رمز تایید دو مرحله‌ای (2FA) را وارد فرمایید:")
						} else if res.Type == AuthResultSuccess {
							_, _ = db.Exec("UPDATE users SET self_status = 'روشن', phone = ?, last_billed_at = NOW() WHERE id = ?", uState.Phone, userID)
							stateMu.Lock()
							delete(userStates, userID)
							stateMu.Unlock()
							startUserbot(userID, cfg, bot)
							return c.Send("🎉 سلف شما روشن شد!", getKeyboard(userID))
						} else {
							stateMu.Lock()
							if uState.Cancel != nil { uState.Cancel() }
							delete(userStates, userID)
							stateMu.Unlock()
							return c.Send(fmt.Sprintf("❌ خطا: %v", res.Error), getKeyboard(userID))
						}
					case <-time.After(35 * time.Second):
						return c.Send("⏱ زمان تایید گذشت.")
					}
				default:
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
							return c.Send("🎉 سلف با موفقیت روشن شد!", getKeyboard(userID))
						} else {
							stateMu.Lock()
							if uState.Cancel != nil { uState.Cancel() }
							delete(userStates, userID)
							stateMu.Unlock()
							return c.Send(fmt.Sprintf("❌ رمز اشتباه: %v", res.Error), getKeyboard(userID))
						}
					case <-time.After(35 * time.Second):
						return c.Send("⏱ زمان تایید رمز گذشت.")
					}
				default:
				}
			}
		}

		if !cfg.IsAdmin(userID) { return nil }

		stateMu.RLock()
		state, adminHasState := adminStates[userID]
		stateMu.RUnlock()

		if !adminHasState { return nil }

		switch state.Action {
		case "set_card_num":
			SetSetting("card_number", text)
			_ = c.Send("✅ شماره کارت ذخیره شد.")
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()
		case "set_card_name":
			SetSetting("card_name", text)
			_ = c.Send("✅ نام دارنده حساب ذخیره شد.")
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()
		case "set_card_bank":
			SetSetting("card_bank", text)
			_ = c.Send("✅ نام بانک ذخیره شد.")
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()
		case "set_support_text":
			SetSetting("support_text", text)
			_ = c.Send("✅ متن پشتیبانی ذخیره شد.")
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()
		case "set_support_id":
			SetSetting("support_id", text)
			_ = c.Send("✅ آیدی پشتیبانی ذخیره شد.")
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()
		case "set_key_price":
			price, _ := strconv.Atoi(text)
			if price > 0 {
				SetSetting("key_price", strconv.Itoa(price))
				_ = c.Send(fmt.Sprintf("✅ نرخ کلید به %s تومان تغییر کرد.", formatMoney(price)))
			}
			stateMu.Lock()
			delete(adminStates, userID)
			stateMu.Unlock()
		}
		return nil
	})

	startBillingWorker(bot)
	startClockWorker()

	rows, err := db.Query("SELECT id FROM users WHERE self_status = 'روشن'")
	if err == nil {
		for rows.Next() {
			var uid int64
			if err := rows.Scan(&uid); err == nil { startUserbot(uid, cfg, bot) }
		}
		rows.Close()
	}

	log.Println("⚡ ربات ولف سلف آماده به کار شد!")
	bot.Start()
}
