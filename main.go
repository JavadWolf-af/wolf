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

var (
	stateMu        sync.RWMutex
	adminStates    = make(map[int64]AdminAction)
	userStates     = make(map[int64]*UserState)
	userWalletTemp = make(map[int64]int)

	activeUserbotsMu sync.RWMutex
	activeUserbots   = make(map[int64]*UserbotSession)
)

var randomEmojiPool = []string{
	"🐺", "👑", "⚡", "🔥", "💎", "✨", "🚀", "🪐", "🌪", "🦁",
	"🦅", "🎯", "🎲", "🖤", "🤍", "❄️", "🌙", "⭐", "💫", "🗡",
	"🛡", "🧿", "🔮", "🎭", "🌊", "🩸", "🕊", "☘️", "🥀", "🌹",
	"🍒", "☕", "🛸", "⚓", "⏳", "🗝", "⚔️", "🏎", "🐉", "🐾",
}

func getRandomEmoji() string {
	return randomEmojiPool[rand.Intn(len(randomEmojiPool))]
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
		is_timer_media_enabled BOOLEAN DEFAULT FALSE
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;`)

	_, _ = db.Exec("ALTER TABLE users ADD COLUMN last_billed_at DATETIME DEFAULT CURRENT_TIMESTAMP")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_clock_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN original_last_name VARCHAR(255) DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_emoji_enabled BOOLEAN DEFAULT FALSE")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN original_first_name VARCHAR(255) DEFAULT ''")
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN is_timer_media_enabled BOOLEAN DEFAULT FALSE")

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
			origFirst = self.FirstName
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
		if ch, ok := e.Channels[p.ChannelID]; ok {
			return &tg.InputPeerChannel{
				ChannelID:  ch.ID,
				AccessHash: ch.AccessHash,
			}
		}
		return &tg.InputPeerChannel{ChannelID: p.ChannelID}
	}
	return nil
}

func deleteMsg(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, msgID int) {
	if ch, ok := inputPeer.(*tg.InputPeerChannel); ok {
		_, _ = client.API().ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{
				ChannelID:  ch.ChannelID,
				AccessHash: ch.AccessHash,
			},
			ID: []int{msgID},
		})
		return
	}
	_, _ = client.API().MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
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
	_, err := client.API().MessagesEditMessage(ctx, editReq)
	if err != nil {
		sendReq := &tg.MessagesSendMessageRequest{
			Peer:     inputPeer,
			Message:  text,
			RandomID: time.Now().UnixNano(),
			Entities: []tg.MessageEntityClass{
				&tg.MessageEntityBold{
					Offset: 0,
					Length: len([]rune(text)),
				},
			},
		}
		res, sendErr := client.API().MessagesSendMessage(ctx, sendReq)
		if sendErr == nil {
			time.Sleep(500 * time.Millisecond)
			if updates, ok := res.(*tg.Updates); ok {
				for _, u := range updates.Updates {
					if nu, ok := u.(*tg.UpdateNewMessage); ok {
						if m, ok := nu.Message.(*tg.Message); ok {
							deleteMsg(ctx, client, inputPeer, m.ID)
						}
					} else if ncu, ok := u.(*tg.UpdateNewChannelMessage); ok {
						if m, ok := ncu.Message.(*tg.Message); ok {
							deleteMsg(ctx, client, inputPeer, m.ID)
						}
					}
				}
			}
			deleteMsg(ctx, client, inputPeer, msgID)
			return
		}
	}

	time.Sleep(500 * time.Millisecond)
	deleteMsg(ctx, client, inputPeer, msgID)
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

func startUserbot(userID int64, cfg Config) {
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

	handleMsg := func(ctx context.Context, e tg.Entities, message tg.MessageClass) {
		msg, ok := message.(*tg.Message)
		if !ok {
			return
		}

		// بررسی عکس‌ها و ویدیوهای تایم‌دار ورودی در پیوی
		if !msg.Out {
			if _, isUser := msg.PeerID.(*tg.PeerUser); isUser && msg.Media != nil {
				var timerEnabled bool
				_ = db.QueryRow("SELECT is_timer_media_enabled FROM users WHERE id = ?", userID).Scan(&timerEnabled)
				if timerEnabled {
					var inputPeer tg.InputPeerClass = getInputPeer(msg.PeerID, e, userID)
					go func() {
						fwdReq := &tg.MessagesForwardMessagesRequest{
							DropAuthor: true,
							FromPeer:   inputPeer,
							ID:         []int{msg.ID},
							RandomID:   []int64{rand.Int63()},
							ToPeer:     &tg.InputPeerSelf{},
						}
						_, _ = client.API().MessagesForwardMessages(ctx, fwdReq)
					}()
				}
			}
			return
		}

		text := strings.TrimSpace(msg.Message)

		var inputPeer tg.InputPeerClass
		self, err := client.Self(ctx)
		selfID := int64(0)
		if err == nil {
			selfID = self.ID
		}
		inputPeer = getInputPeer(msg.PeerID, e, selfID)

		if text == "ساعت روشن شو" || text == "ساعت روشن" {
			handleClockOn(ctx, userID, client)
			if inputPeer != nil {
				go func() {
					dCtx, dCancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer dCancel()
					notifyAndSelfDestruct(dCtx, client, inputPeer, msg.ID, "ساعت روشن شد")
				}()
			}
		} else if text == "ساعت خاموش شو" || text == "ساعت خاموش" {
			handleClockOff(ctx, userID, client)
			if inputPeer != nil {
				go func() {
					dCtx, dCancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer dCancel()
					notifyAndSelfDestruct(dCtx, client, inputPeer, msg.ID, "ساعت خاموش شد")
				}()
			}
		} else if text == "اموجی روشن شو" || text == "اموجی روشن" {
			handleEmojiOn(ctx, userID, client)
			if inputPeer != nil {
				go func() {
					dCtx, dCancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer dCancel()
					notifyAndSelfDestruct(dCtx, client, inputPeer, msg.ID, "اموجی روشن شد")
				}()
			}
		} else if text == "اموجی خاموش شو" || text == "اموجی خاموش" {
			handleEmojiOff(ctx, userID, client)
			if inputPeer != nil {
				go func() {
					dCtx, dCancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer dCancel()
					notifyAndSelfDestruct(dCtx, client, inputPeer, msg.ID, "اموجی خاموش شد")
				}()
			}
		} else if text == "بفرست پیوی همه" {
			go func() {
				bCtx, bCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer bCancel()
				handleForwardToAllPV(bCtx, client, inputPeer, msg, true)
			}()
		} else if text == "پیوی همه" {
			go func() {
				bCtx, bCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer bCancel()
				handleForwardToAllPV(bCtx, client, inputPeer, msg, false)
			}()
		} else if text == "بفرست گروه همه" {
			go func() {
				gCtx, gCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer gCancel()
				handleForwardToAllGroups(gCtx, client, inputPeer, msg, true)
			}()
		} else if text == "گروه همه" {
			go func() {
				gCtx, gCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer gCancel()
				handleForwardToAllGroups(gCtx, client, inputPeer, msg, false)
			}()
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
	ticker := time.NewTicker(10 * time.Minute)
	go func() {
		for range ticker.C {
			if db == nil {
				continue
			}
			rows, err := db.Query("SELECT id, original_first_name FROM users WHERE self_status = 'روشن' AND is_emoji_enabled = TRUE")
			if err != nil {
				continue
			}

			type userEmojiInfo struct {
				id        int64
				origFirst string
			}
			var usersList []userEmojiInfo
			for rows.Next() {
				var u userEmojiInfo
				if err := rows.Scan(&u.id, &u.origFirst); err == nil && u.origFirst != "" {
					usersList = append(usersList, u)
				}
			}
			rows.Close()

			if len(usersList) == 0 {
				continue
			}

			activeUserbotsMu.RLock()
			for _, u := range usersList {
				if ub, ok := activeUserbots[u.id]; ok && ub.Client != nil {
					go func(cl *telegram.Client, orig string) {
						cTimeout, cancel := context.WithTimeout(context.Background(), 15*time.Second)
						defer cancel()
						req := &tg.AccountUpdateProfileRequest{}
						req.SetFirstName(fmt.Sprintf("%s %s", orig, getRandomEmoji()))
						_, _ = cl.API().AccountUpdateProfile(cTimeout, req)
					}(ub.Client, u.origFirst)
				}
			}
			activeUserbotsMu.RUnlock()
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

	userMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	adminMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	adminPanelMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	accountConfigMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	supportConfigMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	profileMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	confirmSelfMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	walletReplyMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	confidentialMenu := &tele.ReplyMarkup{ResizeKeyboard: true}
	timerMediaMenu := &tele.ReplyMarkup{ResizeKeyboard: true}

	btnBuy := userMenu.Text("🛍️ خرید سلف")
	btnProfile := userMenu.Text("👤 حساب کاربری")
	btnWallet := userMenu.Text("👛 کیف پول 💳")
	btnConfidential := userMenu.Text("🔐 محرمانه ها")
	btnSupport := userMenu.Text("🎧 پشتیبانی")
	btnGuide := userMenu.Text("📚 راهنما")
	btnAdminPanel := adminMenu.Text("⚙️ مدیریت")
	btnBack := adminMenu.Text("🔙 بازگشت")

	userMenu.Reply(
		userMenu.Row(btnBuy, btnProfile),
		userMenu.Row(btnWallet, btnConfidential),
		userMenu.Row(btnSupport, btnGuide),
	)

	adminMenu.Reply(
		adminMenu.Row(btnBuy, btnProfile),
		adminMenu.Row(btnWallet, btnConfidential),
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

	btnTimerMedia := confidentialMenu.Text("📸 رسانه تایمردار")
	confidentialMenu.Reply(
		confidentialMenu.Row(btnTimerMedia),
		confidentialMenu.Row(btnBack),
	)

	btnTurnOnTimer := timerMediaMenu.Text("🟢 روشن کردن رسانه تایمردار")
	btnTurnOffTimer := timerMediaMenu.Text("🔴 خاموش کردن رسانه تایمردار")
	timerMediaMenu.Reply(
		timerMediaMenu.Row(btnTurnOnTimer, btnTurnOffTimer),
		timerMediaMenu.Row(btnBack),
	)

	getKeyboard := func(userID int64) *tele.ReplyMarkup {
		if cfg.IsAdmin(userID) {
			return adminMenu
		}
		return userMenu
	}

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
		delete(userWalletTemp, userID)
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

	// هندلر منوی محرمانه ها (با بررسی خرید سلف)
	bot.Handle(&btnConfidential, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Send("❌ <b>دسترسی محدود!</b>\n\nبخش محرمانه ها فقط برای کاربرانی که اشتراک سلف را خریداری کرده‌اند فعال می‌باشد.", getKeyboard(userID), tele.ModeHTML)
		}

		return c.Send("🔐 <b>به بخش محرمانه ها خوش آمدید!</b>\n\nامکانات امنیتی و ویژه سلف در این بخش قرار دارد:", confidentialMenu, tele.ModeHTML)
	})

	bot.Handle(&btnTimerMedia, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		var isTimerEnabled bool
		_ = db.QueryRow("SELECT is_timer_media_enabled FROM users WHERE id = ?", userID).Scan(&isTimerEnabled)

		statusStr := "🔴 خاموش"
		if isTimerEnabled {
			statusStr = "🟢 روشن"
		}

		text := fmt.Sprintf("📸 <b>مدیریت رسانه های تایمردار (View-Once)</b>\n\n"+
			"با فعالسازی این قابلیت، به محض دریافت عکس یا ویدیوی تایم‌دار در پیوی، یک نسخه پشتیبان از آن به صورت خودکار در <b>پیام‌های ذخیره شده (Saved Messages)</b> شما ذخیره می‌شود تا پیش از باز کردن یا انقضای تایمر آن را از دست ندهید.\n\n"+
			"📌 <b>وضعیت فعلی شما:</b> %s", statusStr)

		return c.Send(text, timerMediaMenu, tele.ModeHTML)
	})

	bot.Handle(&btnTurnOnTimer, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_timer_media_enabled = TRUE WHERE id = ?", userID)
		return c.Send("🟢 <b>قابلیت ذخیره رسانه تایمردار با موفقیت روشن شد.</b>", timerMediaMenu, tele.ModeHTML)
	})

	bot.Handle(&btnTurnOffTimer, func(c tele.Context) error {
		userID := c.Sender().ID
		_, _ = db.Exec("UPDATE users SET is_timer_media_enabled = FALSE WHERE id = ?", userID)
		return c.Send("🔴 <b>قابلیت ذخیره رسانه تایمردار خاموش شد.</b>", timerMediaMenu, tele.ModeHTML)
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
			"🧾 <b>فاکتور شارژ کیف پول</b>\n\n💰 <b>مبلغ قابل پرداخت:</b> <code>%s تومان</code>\n🔑 <b>تعداد کلید دریافتی:</b> <code>%.2f کلید</code>\n(نرخ هر کلید: %s تومان)\n\n💳 لطفاً مبلغ فوق را به کارت زیر واریز کرده و سپس <b>تصویر رسید (فیش) واریزی</b> را همینجا برای ربات ارسال کنید:\n\n🏦 <b>%s</b>\n💳 <code>%s</code>\n👤 به نام: <b>%s</b>",
			formatMoney(amount), keys, formatMoney(price), cBank, cNum, cName,
		)

		return c.Send(text, tele.ModeHTML)
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

		return c.Send("✅ <b>فیش واریزی شما با موفقیت برای ادمین ارسال شد.</b>\n\nپس از بررسی و تایید، موجودی کیف پول شما به‌روزرسانی خواهد شد.", tele.ModeHTML, getKeyboard(user.ID))
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
			startUserbot(userID, cfg)

			go func(uid int64) {
				time.Sleep(2 * time.Second)
				activeUserbotsMu.RLock()
				ub, ok := activeUserbots[uid]
				activeUserbotsMu.RUnlock()
				if !ok || ub.Client == nil {
					return
				}

				var isClock, isEmoji bool
				var origFirst string
				_ = db.QueryRow("SELECT is_clock_enabled, is_emoji_enabled, original_first_name FROM users WHERE id = ?", uid).Scan(&isClock, &isEmoji, &origFirst)

				cTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				req := &tg.AccountUpdateProfileRequest{}
				needUpdate := false
				if isClock {
					req.SetLastName(getTehranBoldTime())
					needUpdate = true
				}
				if isEmoji && origFirst != "" {
					req.SetFirstName(fmt.Sprintf("%s %s", origFirst, getRandomEmoji()))
					needUpdate = true
				}
				if needUpdate {
					_, _ = ub.Client.API().AccountUpdateProfile(cTimeout, req)
				}
			}(userID)

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

		var isClock, isEmoji bool
		var origLast, origFirst string
		_ = db.QueryRow("SELECT is_clock_enabled, original_last_name, is_emoji_enabled, original_first_name FROM users WHERE id = ?", userID).Scan(&isClock, &origLast, &isEmoji, &origFirst)

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

		_, _ = db.Exec("UPDATE users SET self_status = 'خروج', phone = 'ثبت نشده', is_clock_enabled = FALSE, is_emoji_enabled = FALSE, is_timer_media_enabled = FALSE WHERE id = ?", userID)

		if c.Message() != nil {
			_ = bot.Delete(c.Message())
		}

		return c.Send("🛑 <b>شما با موفقیت از سیستم سلف خارج شدید و اتصال اکانت شما به طور کامل قطع گردید.</b>", getKeyboard(userID), tele.ModeHTML)
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

							startUserbot(userID, cfg)

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

							startUserbot(userID, cfg)

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

	getGuideInlineKeyboard := func() *tele.ReplyMarkup {
		guideMenu := &tele.ReplyMarkup{}
		btnGuideClock := guideMenu.Data("⏱ ساعت", "guide_clock")
		btnGuideEmoji := guideMenu.Data("🎭 اموجی", "guide_emoji")
		btnGuidePV := guideMenu.Data("📩 پیوی همه", "guide_pv")
		btnGuideGroup := guideMenu.Data("👥 گروه همه", "guide_group")
		guideMenu.Inline(
			guideMenu.Row(btnGuideClock, btnGuideEmoji),
			guideMenu.Row(btnGuidePV, btnGuideGroup),
		)
		return guideMenu
	}

	bot.Handle(&btnGuide, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) {
			return c.Send("❌ حساب کاربری شما مسدود شده است.")
		}

		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Send("❌ <b>دسترسی محدود!</b>\n\nبخش راهنما فقط برای کاربرانی که اشتراک سلف را خریداری کرده‌اند فعال می‌باشد.", getKeyboard(userID), tele.ModeHTML)
		}

		text := "📚 <b>بخش راهنما و آموزش امکانات سلف</b>\n\nجهت مشاهده راهنمای هر قابلیت، روی دکمه مربوط به آن کلیک کنید:"
		return c.Send(text, getGuideInlineKeyboard(), tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "guide_clock"}, func(c tele.Context) error {
		userID := c.Sender().ID
		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید.", ShowAlert: true})
		}

		backMenu := &tele.ReplyMarkup{}
		btnBackGuide := backMenu.Data("🔙 بازگشت", "guide_back")
		backMenu.Inline(backMenu.Row(btnBackGuide))

		text := "⏱ <b>راهنمای فعال‌سازی ساعت زنده روی پروفایل</b>\n\n" +
			"با استفاده از این قابلیت، ساعت رسمی تهران به صورت زنده و با فونت بولد روی نام خانوادگی (Last Name) اکانت شما قرار می‌گیرد و هر دقیقه تغییر می‌کند.\n\n" +
			"🟢 <b>روشن کردن ساعت:</b>\n" +
			"کافیست در هر چتی عبارت زیر را بفرستید:\n" +
			"<code>ساعت روشن شو</code>\n\n" +
			"🔴 <b>خاموش کردن ساعت:</b>\n" +
			"برای خاموش کردن ساعت و بازگرداندن نام خانوادگی قبلی‌تان، در هر چتی عبارت زیر را بفرستید:\n" +
			"<code>ساعت خاموش شو</code>"

		if c.Message() != nil {
			_ = c.Edit(text, backMenu, tele.ModeHTML)
		} else {
			_ = c.Send(text, backMenu, tele.ModeHTML)
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "guide_emoji"}, func(c tele.Context) error {
		userID := c.Sender().ID
		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید.", ShowAlert: true})
		}

		backMenu := &tele.ReplyMarkup{}
		btnBackGuide := backMenu.Data("🔙 بازگشت", "guide_back")
		backMenu.Inline(backMenu.Row(btnBackGuide))

		text := "🎭 <b>راهنمای فعال‌سازی اموجی رندوم کنار اسم</b>\n\n" +
			"با استفاده از این قابلیت، یک اموجی رندوم و جذاب کنار نام شما (First Name) قرار می‌گیرد و هر ۱۰ دقیقه به صورت خودکار تغییر می‌کند.\n\n" +
			"🟢 <b>روشن کردن اموجی:</b>\n" +
			"کافیست در هر چتی عبارت زیر را بفرستید:\n" +
			"<code>اموجی روشن شو</code>\n\n" +
			"🔴 <b>خاموش کردن اموجی:</b>\n" +
			"برای خاموش کردن و بازگرداندن اسم اصلی‌تان، در هر چتی عبارت زیر را بفرستید:\n" +
			"<code>اموجی خاموش شو</code>"

		if c.Message() != nil {
			_ = c.Edit(text, backMenu, tele.ModeHTML)
		} else {
			_ = c.Send(text, backMenu, tele.ModeHTML)
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "guide_pv"}, func(c tele.Context) error {
		userID := c.Sender().ID
		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید.", ShowAlert: true})
		}

		backMenu := &tele.ReplyMarkup{}
		btnBackGuide := backMenu.Data("🔙 بازگشت", "guide_back")
		backMenu.Inline(backMenu.Row(btnBackGuide))

		text := "📩 <b>راهنمای فوروارد به تمام پیوی‌ها</b>\n\n" +
			"روی پیام مورد نظر ریپلای کنید و بنویسید:\n" +
			"🔸 <code>بفرست پیوی همه</code> (بدون نام فرستنده)\n" +
			"🔸 <code>پیوی همه</code> (با درج نام فرستنده)"

		if c.Message() != nil {
			_ = c.Edit(text, backMenu, tele.ModeHTML)
		} else {
			_ = c.Send(text, backMenu, tele.ModeHTML)
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "guide_group"}, func(c tele.Context) error {
		userID := c.Sender().ID
		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Respond(&tele.CallbackResponse{Text: "❌ شما دسترسی ندارید.", ShowAlert: true})
		}

		backMenu := &tele.ReplyMarkup{}
		btnBackGuide := backMenu.Data("🔙 بازگشت", "guide_back")
		backMenu.Inline(backMenu.Row(btnBackGuide))

		text := "👥 <b>راهنمای فوروارد به تمام گروه‌ها</b>\n\n" +
			"روی پیام مورد نظر ریپلای کنید و بنویسید:\n" +
			"🔸 <code>بفرست گروه همه</code> (بدون نام فرستنده)\n" +
			"🔸 <code>گروه همه</code> (با درج نام فرستنده)"

		if c.Message() != nil {
			_ = c.Edit(text, backMenu, tele.ModeHTML)
		} else {
			_ = c.Send(text, backMenu, tele.ModeHTML)
		}
		return c.Respond()
	})

	bot.Handle(&tele.Btn{Unique: "guide_back"}, func(c tele.Context) error {
		text := "📚 <b>بخش راهنما و آموزش امکانات سلف</b>\n\nجهت مشاهده راهنمای هر قابلیت، روی دکمه مربوط به آن کلیک کنید:"
		if c.Message() != nil {
			_ = c.Edit(text, getGuideInlineKeyboard(), tele.ModeHTML)
		} else {
			_ = c.Send(text, getGuideInlineKeyboard(), tele.ModeHTML)
		}
		return c.Respond()
	})

	startBillingWorker(bot)
	startClockWorker()
	startEmojiWorker()

	rows, err := db.Query("SELECT id FROM users WHERE self_status = 'روشن'")
	if err == nil {
		for rows.Next() {
			var uid int64
			if err := rows.Scan(&uid); err == nil {
				startUserbot(uid, cfg)
			}
		}
		rows.Close()
	}

	log.Println("⚡ ربات ولف سلف با بالاترین امنیت و موتور استاندارد آماده و روشن شد!")
	bot.Start()
}
