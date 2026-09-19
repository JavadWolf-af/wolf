package main

import (
	"context"
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

var getMainKeyboard func(userID int64) *tele.ReplyMarkup
var controllerBotID int64

var (
	stateMu        sync.RWMutex
	adminStates    = make(map[int64]AdminAction)
	userStates     = make(map[int64]*UserState)
	userWalletTemp = make(map[int64]int)

	activeUserbotsMu sync.RWMutex
	activeUserbots   = make(map[int64]*UserbotSession)

	activeActionsMu sync.Mutex
	activeActions   = make(map[string]context.CancelFunc)

	channelAccessHashesMu sync.RWMutex
	channelAccessHashes   = make(map[int64]int64)
)

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

// =====================================
// توابع کنترل پروفایل و تنظیمات کاربر
// =====================================
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

// =====================================
// توابع کمکی ارتباط با API تلگرام
// =====================================
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
		if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است") }
		return
	}

	header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || header.ReplyToMsgID == 0 {
		if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است") }
		return
	}
	replyMsgID := header.ReplyToMsgID

	dialogsReq := &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
	}
	res, err := client.API().MessagesGetDialogs(ctx, dialogsReq)
	if err != nil { return }

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
		if !ok { continue }
		peerUser, ok := d.Peer.(*tg.PeerUser)
		if !ok { continue }
		u, exists := userMap[peerUser.UserID]
		if !exists || u.Bot || u.Self || u.Deleted { continue }

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
		if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است") }
		return
	}

	header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || header.ReplyToMsgID == 0 {
		if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "پیام نامعتبر است") }
		return
	}
	replyMsgID := header.ReplyToMsgID

	dialogsReq := &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
	}
	res, err := client.API().MessagesGetDialogs(ctx, dialogsReq)
	if err != nil { return }

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
		if !ok { continue }

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

	if inputPeer != nil {
		notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "انجام شد")
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
				if err != nil { return }
			}
		}
	}()
}

func deleteMessageBatch(ctx context.Context, client *telegram.Client, inputPeer tg.InputPeerClass, ids []int) {
	if len(ids) == 0 { return }
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
	if err != nil { return }

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
		if err != nil { break }

		var messages []tg.MessageClass
		switch h := res.(type) {
		case *tg.MessagesMessages:
			messages = h.Messages
		case *tg.MessagesMessagesSlice:
			messages = h.Messages
		case *tg.MessagesChannelMessages:
			messages = h.Messages
		}

		if len(messages) == 0 { break }

		stopSearch := false
		for _, mClass := range messages {
			m, ok := mClass.(*tg.Message)
			if !ok { continue }

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

		if stopSearch || len(messages) < 100 { break }

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

// =====================================
// حلقه اصلی سلف‌بات
// =====================================
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
		if !ok { return }

		for cid, ch := range e.Channels {
			channelAccessHashesMu.Lock()
			channelAccessHashes[cid] = ch.AccessHash
			channelAccessHashesMu.Unlock()
		}

		var inputPeer tg.InputPeerClass
		self, err := client.Self(ctx)
		selfID := int64(0)
		if err == nil {
			selfID = self.ID
		}
		inputPeer = getInputPeer(msg.PeerID, e, selfID)

		// استخراج شناسه فرستنده
		senderID := int64(0)
		if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
			senderID = fromUser.UserID
		} else if peerUser, ok := msg.PeerID.(*tg.PeerUser); ok {
			senderID = peerUser.UserID
		}

		// =======================================================================
		// 🔴 هسته اصلی قفل پیوی (اولین خط دفاعی برای نابودی تمام پیام‌های مزاحم)
		// =======================================================================
		if !msg.Out {
			if _, ok := msg.PeerID.(*tg.PeerUser); ok && senderID != 0 {
				
				// بررسی استثنائات (تلگرام رسمی و ربات‌ها)
				isBot := false
				if senderID == 777000 {
					isBot = true
				} else if usr, exists := e.Users[senderID]; exists && usr != nil {
					if usr.Bot {
						isBot = true
					}
				}

				if !isBot {
					var pvLockEnabled bool
					_ = db.QueryRow("SELECT is_pv_lock_enabled FROM users WHERE id = ?", userID).Scan(&pvLockEnabled)
					
					if pvLockEnabled {
						var isAllowed int
						_ = db.QueryRow("SELECT COUNT(*) FROM wolf_pv_allowed WHERE owner_id = ? AND allowed_id = ?", userID, senderID).Scan(&isAllowed)
						
						if isAllowed == 0 {
							go func(p tg.InputPeerClass) {
								dCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
								defer cancel()
								
								// فقط از تابع حذف تاریخچه استفاده می‌کنیم تا کل باکس چت محو شود
								// با قرار دادن JustClear: false، باکس چت از لیست دیالوگ‌ها به صورت دوطرفه پاک خواهد شد
								_, _ = client.API().MessagesDeleteHistory(dCtx, &tg.MessagesDeleteHistoryRequest{
									JustClear: false,
									Revoke:    true,
									Peer:      p,
									MaxID:     0,
								})
							}(inputPeer)
							return // ❌ خروج فوری از تابع (توقف ارسال به حالت روح یا لاگر)
						}
					}
				}
			}
		}

		// هدایت سایر پیام‌های مجاز به سمت سیستم‌های ولف پلاس (مثل حالت روح و ضدحذف)
		WolfPlusHandleIncoming(ctx, client, bot, userID, msg, e)

		peerKey := "chat"
		switch p := msg.PeerID.(type) {
		case *tg.PeerUser:
			peerKey = fmt.Sprintf("user_%d", p.UserID)
		case *tg.PeerChat:
			peerKey = fmt.Sprintf("chat_%d", p.ChatID)
		case *tg.PeerChannel:
			peerKey = fmt.Sprintf("channel_%d", p.ChannelID)
		}

		// پردازش سیستم دوست و دشمن و ری‌اکشن برای پیام‌های مجاز دریافتی
		if !msg.Out {
			if senderID != 0 {
				// ری‌اکشن خودکار
				if emoji, exists := getAutoReact(userID, senderID); exists && inputPeer != nil {
					go func(msgID int, p tg.InputPeerClass, em string) {
						time.Sleep(200 * time.Millisecond)
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

				// سیستم دوست
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
						req.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: msgID})
						_, _ = client.API().MessagesSendMessage(rCtx, req)
					}(msg.ID, inputPeer)
				}

				// سیستم دشمن
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
						req.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: msgID})
						_, _ = client.API().MessagesSendMessage(rCtx, req)
					}(msg.ID, inputPeer)
				}
			}
			return
		}

		text := strings.TrimSpace(msg.Message)

		// === تنظیمات قفل پیوی (دستورات کاربر) ===
		if text == "قفل پیوی روشن" {
			_, _ = db.Exec("UPDATE users SET is_pv_lock_enabled = TRUE WHERE id = ?", userID)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🔒 قفل پیوی روشن شد") }
			return
		} else if text == "قفل پیوی خاموش" {
			_, _ = db.Exec("UPDATE users SET is_pv_lock_enabled = FALSE WHERE id = ?", userID)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🔓 قفل پیوی خاموش شد") }
			return
		} else if text == "قفل پیوی باز" {
			if p, ok := msg.PeerID.(*tg.PeerUser); ok {
				_, _ = db.Exec("INSERT IGNORE INTO wolf_pv_allowed (owner_id, allowed_id) VALUES (?, ?)", userID, p.UserID)
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "✅ قفل پیوی برای این کاربر باز شد (استثنا)") }
			} else {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ این دستور فقط در چت خصوصی (پیوی) کار می‌کند!") }
			}
			return
		} else if text == "قفل پیوی بسته" {
			if p, ok := msg.PeerID.(*tg.PeerUser); ok {
				_, _ = db.Exec("DELETE FROM wolf_pv_allowed WHERE owner_id = ? AND allowed_id = ?", userID, p.UserID)
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "❌ قفل پیوی برای این کاربر بسته شد") }
			} else {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ این دستور فقط در چت خصوصی (پیوی) کار می‌کند!") }
			}
			return
		}

		// === ری‌اکشن خودکار ===
		isAutoReact := text == "ری‌اکشن" || strings.HasPrefix(text, "ری‌اکشن ") || text == "ری اکشن" || strings.HasPrefix(text, "ری اکشن ") || text == "ریاکشن" || strings.HasPrefix(text, "ریاکشن ")
		if isAutoReact {
			if msg.ReplyTo == nil {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!") }
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 { return }

			emoji := "❤️"
			parts := strings.Split(text, " ")
			if len(parts) >= 2 {
				emoji = strings.TrimSpace(parts[1])
			} else if strings.HasPrefix(text, "ری‌اکشن ") { 
				em := strings.TrimSpace(strings.TrimPrefix(text, "ری‌اکشن "))
				if em != "" { emoji = em }
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
				if targetUID == 0 || targetUID == selfID { return }

				targetUName := ""
				for _, uClass := range usersList {
					if u, ok := uClass.(*tg.User); ok && u.ID == targetUID {
						targetUName = formatTelegramUser(u)
						break
					}
				}
				if targetUName == "" { targetUName = fmt.Sprintf("کاربر (%d)", targetUID) }

				_, _ = db.Exec(`
					INSERT INTO wolf_auto_reacts (owner_id, target_id, target_name, emoji)
					VALUES (?, ?, ?, ?)
					ON DUPLICATE KEY UPDATE target_name = VALUES(target_name), emoji = VALUES(emoji)
				`, userID, targetUID, targetUName, selectedEmoji)

				setAutoReactToCache(userID, targetUID, selectedEmoji)
				go notifyAndSelfDestruct(dCtx, client, p, mID, fmt.Sprintf("✅ ری‌اکشن %s فعال شد", selectedEmoji))
			}(header.ReplyToMsgID, inputPeer, msg.ID, emoji)
			return

		} else if text == "حذف ری‌اکشن" || text == "حذف ری اکشن" || text == "حذف ریاکشن" {
			if msg.ReplyTo == nil {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!") }
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 { return }

			go func(repID int, p tg.InputPeerClass, mID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()

				repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil { return }

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok {
					targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok {
					targetUID = peerU.UserID
				}
				if targetUID == 0 { return }

				_, _ = db.Exec("DELETE FROM wolf_auto_reacts WHERE owner_id = ? AND target_id = ?", userID, targetUID)
				removeAutoReactFromCache(userID, targetUID)
				go notifyAndSelfDestruct(dCtx, client, p, mID, "✅ ری‌اکشن این فرد لغو شد")
			}(header.ReplyToMsgID, inputPeer, msg.ID)
			return

		} else if text == "لیست ری‌اکشن" || text == "لیست ری اکشن" || text == "لیست ریاکشن" {
			go func(mID int, p tg.InputPeerClass) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()
				go notifyAndSelfDestruct(dCtx, client, p, mID, "📋 لیست ری‌اکشن‌ها ارسال شد")
				rows, err := db.Query("SELECT target_id, target_name, emoji FROM wolf_auto_reacts WHERE owner_id = ?", userID)
				if err != nil { return }
				defer rows.Close()

				var list []string
				idx := 1
				for rows.Next() {
					var tid int64
					var tname, emj string
					if err := rows.Scan(&tid, &tname, &emj); err == nil {
						list = append(list, fmt.Sprintf("%d. %s (<code>%d</code>) - اموجی: %s", idx, tname, tid, emj))
						idx++
					}
				}
				msgText := "🔥 <b>لیست ری‌اکشن‌های خودکار شما:</b>\n\n" + strings.Join(list, "\n")
				if len(list) == 0 { msgText = "⚠️ <i>لیست ری‌اکشن‌های خودکار شما خالی است!</i>" }

				_, _ = client.API().MessagesSendMessage(dCtx, &tg.MessagesSendMessageRequest{
					Peer:     &tg.InputPeerSelf{},
					Message:  msgText,
					RandomID: rand.Int63(),
				})
			}(msg.ID, inputPeer)
			return

		} else if text == "پاکسازی ری‌اکشن" || text == "پاکسازی ری اکشن" || text == "پاکسازی ریاکشن" {
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

		// === شمارش معکوس ===
		if strings.HasPrefix(text, "تایمر ") || strings.HasPrefix(text, "شمارش ") {
			prefix := "تایمر "
			if strings.HasPrefix(text, "شمارش ") { prefix = "شمارش " }
			numStr := strings.TrimSpace(strings.TrimPrefix(text, prefix))
			count, err := strconv.Atoi(numStr)
			if err == nil && count > 0 {
				if count > 60 { count = 60 }
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

		// === پاکسازی سریع ===
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
					if count > 100 { count = 100 }
					go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, fmt.Sprintf("🗑 در حال حذف %d پیام اخیر...", count))
					go handlePurgeAction(ctx, client, inputPeer, msg.ID, 0, count)
					return
				}
			}
		}

		// === اکشن‌های جعلی ===
		if text == "لغو اکشن" || text == "توقف اکشن" {
			actionKey := fmt.Sprintf("%d_%s", userID, peerKey)
			stopActiveAction(actionKey)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🛑 اکشن متوقف شد") }
			return
		}

		parseActionDuration := func(cmdText, prefix string) int {
			rem := strings.TrimSpace(strings.TrimPrefix(cmdText, prefix))
			if rem == "" { return 20 }
			d, err := strconv.Atoi(rem)
			if err != nil || d <= 0 { return 20 }
			return d
		}

		var actionToRun tg.SendMessageActionClass
		actionDuration := 20
		isActionCmd := false

		if strings.HasPrefix(text, "اکشن تایپ") || strings.HasPrefix(text, "تایپینگ") {
			isActionCmd = true
			actionToRun = &tg.SendMessageTypingAction{}
			if strings.HasPrefix(text, "اکشن تایپ") { actionDuration = parseActionDuration(text, "اکشن تایپ")
			} else { actionDuration = parseActionDuration(text, "تایپینگ") }
		} else if strings.HasPrefix(text, "اکشن وویس") || strings.HasPrefix(text, "ضبط صدا") {
			isActionCmd = true
			actionToRun = &tg.SendMessageRecordAudioAction{}
			if strings.HasPrefix(text, "اکشن وویس") { actionDuration = parseActionDuration(text, "اکشن وویس")
			} else { actionDuration = parseActionDuration(text, "ضبط صدا") }
		} else if strings.HasPrefix(text, "اکشن ویدیوگرد") || strings.HasPrefix(text, "ویدیو گرد") {
			isActionCmd = true
			actionToRun = &tg.SendMessageRecordRoundAction{}
			if strings.HasPrefix(text, "اکشن ویدیوگرد") { actionDuration = parseActionDuration(text, "اکشن ویدیوگرد")
			} else { actionDuration = parseActionDuration(text, "ویدیو گرد") }
		} else if strings.HasPrefix(text, "اکشن ویدیو") || strings.HasPrefix(text, "ضبط ویدیو") {
			isActionCmd = true
			actionToRun = &tg.SendMessageRecordVideoAction{}
			if strings.HasPrefix(text, "اکشن ویدیو") { actionDuration = parseActionDuration(text, "اکشن ویدیو")
			} else { actionDuration = parseActionDuration(text, "ضبط ویدیو") }
		} else if strings.HasPrefix(text, "اکشن عکس") || strings.HasPrefix(text, "ارسال عکس") {
			isActionCmd = true
			actionToRun = &tg.SendMessageUploadPhotoAction{}
			if strings.HasPrefix(text, "اکشن عکس") { actionDuration = parseActionDuration(text, "اکشن عکس")
			} else { actionDuration = parseActionDuration(text, "ارسال عکس") }
		} else if strings.HasPrefix(text, "اکشن فایل") || strings.HasPrefix(text, "ارسال فایل") {
			isActionCmd = true
			actionToRun = &tg.SendMessageUploadDocumentAction{}
			if strings.HasPrefix(text, "اکشن فایل") { actionDuration = parseActionDuration(text, "اکشن فایل")
			} else { actionDuration = parseActionDuration(text, "ارسال فایل") }
		} else if strings.HasPrefix(text, "اکشن بازی") || strings.HasPrefix(text, "بازی") {
			isActionCmd = true
			actionToRun = &tg.SendMessageGamePlayAction{}
			if strings.HasPrefix(text, "اکشن بازی") { actionDuration = parseActionDuration(text, "اکشن بازی")
			} else { actionDuration = parseActionDuration(text, "بازی") }
		}

		if isActionCmd && actionToRun != nil {
			if inputPeer != nil {
				go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, fmt.Sprintf("✅ اکشن به مدت %d ثانیه فعال شد", actionDuration))
				go startFakeAction(ctx, client, userID, inputPeer, peerKey, actionToRun, actionDuration)
			}
			return
		}

		// === سیستم دوست ===
		if text == "تنظیم دوست" {
			if msg.ReplyTo == nil {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!") }
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 { return }
			
			go func(repID int, p tg.InputPeerClass, mID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()
				go notifyAndSelfDestruct(dCtx, client, p, mID, "✅ تنظیم دوست شد")

				repMsg, usersList, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil { return }

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok { targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok { targetUID = peerU.UserID }

				if targetUID == 0 || targetUID == selfID { return }

				targetUName := ""
				for _, uClass := range usersList {
					if u, ok := uClass.(*tg.User); ok && u.ID == targetUID {
						targetUName = formatTelegramUser(u)
						break
					}
				}
				if targetUName == "" { targetUName = fmt.Sprintf("کاربر (%d)", targetUID) }

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
				sendReq.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: repID})
				_, _ = client.API().MessagesSendMessage(dCtx, sendReq)
			}(header.ReplyToMsgID, inputPeer, msg.ID)
			return

		} else if text == "حذف دوست" {
			if msg.ReplyTo == nil {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!") }
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 { return }

			go func(repID int, p tg.InputPeerClass, mID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()
				go notifyAndSelfDestruct(dCtx, client, p, mID, "✅ حذف دوست شد")

				repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil { return }

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok { targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok { targetUID = peerU.UserID }

				if targetUID == 0 { return }

				_, _ = db.Exec("DELETE FROM wolf_friends WHERE owner_id = ? AND friend_id = ?", userID, targetUID)
				removeFriendFromCache(userID, targetUID)
			}(header.ReplyToMsgID, inputPeer, msg.ID)
			return

		} else if text == "لیست دوست" {
			go func(mID int, p tg.InputPeerClass) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()
				go notifyAndSelfDestruct(dCtx, client, p, mID, "📋 لیست دوست ارسال شد")

				rows, err := db.Query("SELECT friend_id, friend_name FROM wolf_friends WHERE owner_id = ?", userID)
				if err != nil { return }
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
				if len(list) == 0 { msgText = "⚠️ <i>لیست دوستان شما در حال حاضر خالی است!</i>" }

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

		// === سیستم دشمن ===
		if text == "تنظیم دشمن" {
			if msg.ReplyTo == nil {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!") }
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 { return }

			go func(repID int, p tg.InputPeerClass, mID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()
				go notifyAndSelfDestruct(dCtx, client, p, mID, "⚔️ تنظیم دشمن شد")

				repMsg, usersList, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil { return }

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok { targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok { targetUID = peerU.UserID }

				if targetUID == 0 || targetUID == selfID { return }

				targetUName := ""
				for _, uClass := range usersList {
					if u, ok := uClass.(*tg.User); ok && u.ID == targetUID {
						targetUName = formatTelegramUser(u)
						break
					}
				}
				if targetUName == "" { targetUName = fmt.Sprintf("کاربر (%d)", targetUID) }

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
				sendReq.SetReplyTo(&tg.InputReplyToMessage{ReplyToMsgID: repID})
				_, _ = client.API().MessagesSendMessage(dCtx, sendReq)
			}(header.ReplyToMsgID, inputPeer, msg.ID)
			return

		} else if text == "حذف دشمن" {
			if msg.ReplyTo == nil {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی پیام فرد ریپلای کنید!") }
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 { return }

			go func(repID int, p tg.InputPeerClass, mID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()
				go notifyAndSelfDestruct(dCtx, client, p, mID, "✅ حذف دشمن شد")

				repMsg, _, err := getRepliedMessageAndUsers(dCtx, client, p, repID)
				if err != nil || repMsg == nil { return }

				var targetUID int64
				if f, ok := repMsg.FromID.(*tg.PeerUser); ok { targetUID = f.UserID
				} else if peerU, ok := repMsg.PeerID.(*tg.PeerUser); ok { targetUID = peerU.UserID }

				if targetUID == 0 { return }

				_, _ = db.Exec("DELETE FROM wolf_enemies WHERE owner_id = ? AND enemy_id = ?", userID, targetUID)
				removeEnemyFromCache(userID, targetUID)
			}(header.ReplyToMsgID, inputPeer, msg.ID)
			return

		} else if text == "لیست دشمن" {
			go func(mID int, p tg.InputPeerClass) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer dCancel()
				go notifyAndSelfDestruct(dCtx, client, p, mID, "📋 لیست دشمن ارسال شد")

				rows, err := db.Query("SELECT enemy_id, enemy_name FROM wolf_enemies WHERE owner_id = ?", userID)
				if err != nil { return }
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
				if len(list) == 0 { msgText = "⚠️ <i>لیست دشمنان شما در حال حاضر خالی است!</i>" }

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

		// === سیستم خوشنویسی ===
		if text == "خوشنویسی روشن" {
			_, _ = db.Exec("UPDATE users SET is_font_enabled = TRUE WHERE id = ?", userID)
			_, mode := getFontSetting(userID)
			updateFontCache(userID, true, mode)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🟢 خوشنویسی روشن شد") }
			return
		} else if text == "خوشنویسی خاموش" {
			_, _ = db.Exec("UPDATE users SET is_font_enabled = FALSE WHERE id = ?", userID)
			_, mode := getFontSetting(userID)
			updateFontCache(userID, false, mode)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "🔴 خوشنویسی خاموش شد") }
			return
		} else if strings.HasPrefix(text, "خوشنویسی ") {
			cleanSub := strings.TrimSpace(strings.TrimPrefix(text, "خوشنویسی "))
			mode := ""
			switch cleanSub {
			case "بولد": mode = "bold"
			case "ایتالیک": mode = "italic"
			case "بولد ایتالیک": mode = "bold_italic"
			case "زیر خط": mode = "underline"
			case "خط خورده": mode = "strike"
			case "مونو": mode = "mono"
			case "اسپویل": mode = "spoiler"
			}

			if mode != "" {
				_, _ = db.Exec("UPDATE users SET font_mode = ?, is_font_enabled = TRUE WHERE id = ?", mode, userID)
				en, _ := getFontSetting(userID)
				updateFontCache(userID, en, mode)
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, fmt.Sprintf("✅ فونت به %s تغییر یافت", text)) }
				return
			}
		}

		if text == "دانلود" || text == "سیو" {
			if msg.ReplyTo != nil {
				if header, ok := msg.ReplyTo.(*tg.MessageReplyHeader); ok && header.ReplyToMsgID != 0 {
					go func() {
						HandleProtectedDownloadByReply(ctx, client, inputPeer, header.ReplyToMsgID, userID)
						if inputPeer != nil { notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "محتوا به پیام‌های ذخیره‌شده ارسال شد") }
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

		// === امکانات متفرقه ===
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
		} else if text == "بیو روشن شو" || text == "بیو روشن" {
			handleBioOn(ctx, userID, client)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "بیو روشن شد") }
			return
		} else if text == "بیو خاموش شو" || text == "بیو خاموش" {
			handleBioOff(ctx, userID, client)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "بیو خاموش شد") }
			return
		} else if text == "رندوم شو" {
			handleBioRandom(ctx, userID, client)
			if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "بیو به حالت رندوم تغییر یافت") }
			return
		} else if text == "بیو شو" {
			if msg.ReplyTo == nil {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ لطفاً روی یک پیام ریپلای کنید!") }
				return
			}
			header, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
			if !ok || header.ReplyToMsgID == 0 {
				if inputPeer != nil { go notifyAndSelfDestruct(ctx, client, inputPeer, msg.ID, "⚠️ پیام معتبر نیست!") }
				return
			}
			go func(replyID int) {
				dCtx, dCancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer dCancel()
				res, err := client.API().MessagesGetMessages(dCtx, []tg.InputMessageClass{&tg.InputMessageID{ID: replyID}})
				if err != nil {
					if inputPeer != nil { notifyAndSelfDestruct(dCtx, client, inputPeer, msg.ID, "❌ خطا در خواندن پیام!") }
					return
				}
				var targetText string
				switch mSlice := res.(type) {
				case *tg.MessagesMessages:
					if len(mSlice.Messages) > 0 { if m, ok := mSlice.Messages[0].(*tg.Message); ok { targetText = m.Message } }
				case *tg.MessagesMessagesSlice:
					if len(mSlice.Messages) > 0 { if m, ok := mSlice.Messages[0].(*tg.Message); ok { targetText = m.Message } }
				case *tg.MessagesChannelMessages:
					if len(mSlice.Messages) > 0 { if m, ok := mSlice.Messages[0].(*tg.Message); ok { targetText = m.Message } }
				}

				targetText = strings.TrimSpace(targetText)
				if targetText == "" {
					if inputPeer != nil { notifyAndSelfDestruct(dCtx, client, inputPeer, msg.ID, "⚠️ پیام متنی یافت نشد!") }
					return
				}

				handleBioCustom(dCtx, userID, client, targetText)
				if inputPeer != nil { notifyAndSelfDestruct(dCtx, client, inputPeer, msg.ID, "بیو با موفقیت تنظیم شد") }
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

		// اعمال بلادرنگ فونت
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

// =====================================
// ورود به اکانت (Authentication)
// =====================================
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
		if !ok { return "", errors.New("auth canceled") }
		return code, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (b *botAuthenticator) Password(ctx context.Context) (string, error) {
	b.resultChan <- AuthResult{Type: AuthResultNeeds2FA}
	select {
	case pwd, ok := <-b.passwordChan:
		if !ok { return "", errors.New("auth canceled") }
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

// =====================================
// راه‌اندازی اصلی ربات مادر
// =====================================
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
		if firstName == "" { firstName = "کاربر" }
		username := "ثبت نشده"
		if user.Username != "" { username = "@" + html.EscapeString(user.Username) }

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
			if uState.Cancel != nil { uState.Cancel() }
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
		if IsUserBlocked(user.ID) { return c.Send("❌ حساب کاربری مسدود است.") }

		var joinedAt time.Time
		var selfStatus string
		err := db.QueryRow("SELECT joined_at, self_status FROM users WHERE id = ?", user.ID).Scan(&joinedAt, &selfStatus)
		if err != nil || joinedAt.IsZero() { joinedAt = time.Now() }

		loc := getTehranLocation()
		now := time.Now().In(loc)
		tNow := gpc.New(now)
		tJoined := gpc.New(joinedAt.In(loc))

		daysActive := int(now.Sub(joinedAt.In(loc)).Hours() / 24)
		if daysActive < 1 { daysActive = 1 }

		statusIcon := "❌"
		if selfStatus == "روشن" { statusIcon = "✅"
		} else if selfStatus == "خاموش" { statusIcon = "⏸️" }

		tNowStr := toPersianDigits(tNow.Format("yyyy/MM/dd"))
		tTimeStr := toPersianDigits(tNow.Format("HH:mm:ss"))
		tJoinedStr := toPersianDigits(tJoined.Format("yyyy/MM/dd"))

		text := fmt.Sprintf("💙 تاریخ امروز: %s\n\n⏰ ساعت: %s\n\n🔒 اطلاعات حساب کاربری\n\n⭐ آیدی عددی: <code>%d</code>\n📅 تاریخ عضویت در ربات: %s\n👀 فعالیت در ربات: %d روز\n💰 موجودی: %s تومان\n🔥 وضعیت سلف: %s %s",
			tNowStr, tTimeStr, user.ID, tJoinedStr, daysActive, formatMoney(GetUserBalance(user.ID)), statusIcon, selfStatus)

		return c.Send(text, profileMenu, tele.ModeHTML)
	})

	bot.Handle(&btnWolfPlus, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) { return c.Send("❌ حساب کاربری شما مسدود شده است.") }
		selfStatus := GetUserSelfStatus(userID)
		if selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Send("❌ <b>دسترسی محدود!</b>\n\nامکانات ویژه ولف + فقط برای کاربران دارای اشتراک فعال است.", getKeyboard(userID), tele.ModeHTML)
		}
		return c.Send(buildWolfPlusDashboardText(userID), wolfPlusMenu, tele.ModeHTML)
	})

	RegisterWolfPlusHandlers(bot)

	bot.Handle(&btnGuide, func(c tele.Context) error {
		userID := c.Sender().ID
		if IsUserBlocked(userID) { return c.Send("❌ حساب کاربری شما مسدود است.") }
		if selfStatus := GetUserSelfStatus(userID); selfStatus == "خرید نداشته" || selfStatus == "خروج" {
			return c.Send("❌ <b>دسترسی محدود!</b>\n\nبخش راهنما فقط برای کاربران دارای اشتراک فعال است.", getKeyboard(userID), tele.ModeHTML)
		}

		var isClock, isEmoji, isBio, isFont bool
		var bioMode string
		_ = db.QueryRow("SELECT is_clock_enabled, is_emoji_enabled, is_bio_enabled, bio_mode, is_font_enabled FROM users WHERE id = ?", userID).Scan(&isClock, &isEmoji, &isBio, &bioMode, &isFont)
		var friendCount, enemyCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM wolf_friends WHERE owner_id = ?", userID).Scan(&friendCount)
		_ = db.QueryRow("SELECT COUNT(*) FROM wolf_enemies WHERE owner_id = ?", userID).Scan(&enemyCount)

		clockStatus := "🔴 خاموش"
		if isClock { clockStatus = "🟢 روشن" }
		emojiStatus := "🔴 خاموش"
		if isEmoji { emojiStatus = "🟢 روشن" }
		bioStatus := "🔴 خاموش"
		if isBio {
			if bioMode == "custom" { bioStatus = "🟢 روشن (دستی)"
			} else { bioStatus = "🟢 روشن (رندوم)" }
		}
		fontStatus := "🔴 خاموش"
		if isFont { fontStatus = "🟢 روشن" }

		text := fmt.Sprintf(`📚 <b>بخش راهنما و امکانات سلف ولف 🐺</b>
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
💡 <i>جهت مطالعه راهنما و تنظیم قابلیت‌ها، از کیبورد ثابت زیر استفاده کنید:</i>`,
			clockStatus, emojiStatus, bioStatus, fontStatus, friendCount, enemyCount)
		return c.Send(text, guideMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGClock, func(c tele.Context) error {
		var isClock bool
		_ = db.QueryRow("SELECT is_clock_enabled FROM users WHERE id = ?", c.Sender().ID).Scan(&isClock)
		statusStr := "🔴 خاموش"
		if isClock { statusStr = "🟢 روشن" }
		text := fmt.Sprintf(`⏱ <b>راهنمای ساعت زنده (Live Clock)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
با روشن کردن این قابلیت، ساعت جاری به صورت زنده و با فونت بولد در نام خانوادگی (LastName) شما نمایش داده می‌شود و هر دقیقه به‌روزرسانی می‌گردد.`, statusStr)
		return c.Send(text, guideClockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnClockOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil { return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideClockMenu, tele.ModeHTML) }
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
		if !ok || ub.Client == nil { return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideClockMenu, tele.ModeHTML) }
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleClockOff(ctx, userID, ub.Client)
		return c.Send("🔴 <b>ساعت زنده خاموش شد.</b>", guideClockMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGEmoji, func(c tele.Context) error {
		var isEmoji bool
		_ = db.QueryRow("SELECT is_emoji_enabled FROM users WHERE id = ?", c.Sender().ID).Scan(&isEmoji)
		statusStr := "🔴 خاموش"
		if isEmoji { statusStr = "🟢 روشن" }
		text := fmt.Sprintf(`🎭 <b>راهنمای اموجی رندوم (Random Emoji)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
با فعال‌سازی این قابلیت، ربات به صورت دوره‌ای یک اموجی جذاب و زیبا را به انتهای نام شما اضافه می‌کند.`, statusStr)
		return c.Send(text, guideEmojiMenu, tele.ModeHTML)
	})
	bot.Handle(&btnEmojiOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil { return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideEmojiMenu, tele.ModeHTML) }
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleEmojiOn(ctx, userID, ub.Client)
		return c.Send("🟢 <b>اموجی رندوم روشن شد.</b>", guideEmojiMenu, tele.ModeHTML)
	})
	bot.Handle(&btnEmojiOff, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil { return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideEmojiMenu, tele.ModeHTML) }
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleEmojiOff(ctx, userID, ub.Client)
		return c.Send("🔴 <b>اموجی خاموش شد.</b>", guideEmojiMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGBio, func(c tele.Context) error {
		var isBio bool
		var bioMode string
		_ = db.QueryRow("SELECT is_bio_enabled, bio_mode FROM users WHERE id = ?", c.Sender().ID).Scan(&isBio, &bioMode)
		statusStr := "🔴 خاموش"
		if isBio {
			if bioMode == "custom" { statusStr = "🟢 روشن (دستی)" } else { statusStr = "🟢 روشن (رندوم)" }
		}
		text := fmt.Sprintf(`📝 <b>راهنمای بیوگرافی هوشمند (Smart Bio)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
بیوگرافی پروفایل شما به صورت هوشمند و رندوم از جملات خاص آپدیت می‌شود. همچنین با ریپلای روی هر متنی و ارسال دستور <code>بیو شو</code> می‌توانید بیوگرافی خود را تنظیم کنید.`, statusStr)
		return c.Send(text, guideBioMenu, tele.ModeHTML)
	})
	bot.Handle(&btnBioOn, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil { return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideBioMenu, tele.ModeHTML) }
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleBioOn(ctx, userID, ub.Client)
		return c.Send("🟢 <b>بیوگرافی رندوم فعال شد.</b>", guideBioMenu, tele.ModeHTML)
	})
	bot.Handle(&btnBioOff, func(c tele.Context) error {
		userID := c.Sender().ID
		activeUserbotsMu.RLock()
		ub, ok := activeUserbots[userID]
		activeUserbotsMu.RUnlock()
		if !ok || ub.Client == nil { return c.Send("❌ <b>سلف شما آنلاین نیست!</b>", guideBioMenu, tele.ModeHTML) }
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		handleBioOff(ctx, userID, ub.Client)
		return c.Send("🔴 <b>بیوگرافی خاموش شد.</b>", guideBioMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGFriend, func(c tele.Context) error {
		var friendCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM wolf_friends WHERE owner_id = ?", c.Sender().ID).Scan(&friendCount)
		text := fmt.Sprintf(`🌸 <b>سیستم دوست (Friend System)</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>تعداد دوستان ثبت‌شده:</b> <code>%d نفر</code>
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
با ریپلای روی پیام هر شخص و ارسال دستور <code>تنظیم دوست</code>، آن فرد به لیست دوستان شما اضافه می‌شود و سلف به طور خودکار پیام‌های او را با جملات دوستانه پاسخ می‌دهد.`, friendCount)
		return c.Send(text, guideFriendMenu, tele.ModeHTML)
	})
	bot.Handle(&btnFriendList, func(c tele.Context) error {
		rows, err := db.Query("SELECT friend_id, friend_name FROM wolf_friends WHERE owner_id = ?", c.Sender().ID)
		if err != nil { return c.Send("❌ خطا در دریافت لیست دوستان.", guideFriendMenu, tele.ModeHTML) }
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
		if len(list) == 0 { return c.Send("⚠️ <i>لیست دوستان شما خالی است.</i>", guideFriendMenu, tele.ModeHTML) }
		return c.Send("📋 <b>لیست دوستان شما:</b>\n\n"+strings.Join(list, "\n"), guideFriendMenu, tele.ModeHTML)
	})
	bot.Handle(&btnFriendClear, func(c tele.Context) error {
		_, _ = db.Exec("DELETE FROM wolf_friends WHERE owner_id = ?", c.Sender().ID)
		clearFriendsCache(c.Sender().ID)
		return c.Send("🗑 <b>لیست دوستان پاکسازی شد.</b>", guideFriendMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGEnemy, func(c tele.Context) error {
		var enemyCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM wolf_enemies WHERE owner_id = ?", c.Sender().ID).Scan(&enemyCount)
		text := fmt.Sprintf(`⚔️ <b>سیستم دشمن (Enemy System)</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>تعداد دشمنان ثبت‌شده:</b> <code>%d نفر</code>
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
با ریپلای روی پیام هر شخص و ارسال دستور <code>تنظیم دشمن</code>، آن فرد در لیست سیاه قرار می‌گیرد و سلف به طور خودکار با پاسخ‌های خاص به او جواب می‌دهد.`, enemyCount)
		return c.Send(text, guideEnemyMenu, tele.ModeHTML)
	})
	bot.Handle(&btnEnemyList, func(c tele.Context) error {
		rows, err := db.Query("SELECT enemy_id, enemy_name FROM wolf_enemies WHERE owner_id = ?", c.Sender().ID)
		if err != nil { return c.Send("❌ خطا در دریافت لیست دشمنان.", guideEnemyMenu, tele.ModeHTML) }
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
		if len(list) == 0 { return c.Send("⚠️ <i>لیست دشمنان شما خالی است.</i>", guideEnemyMenu, tele.ModeHTML) }
		return c.Send("📋 <b>لیست دشمنان شما:</b>\n\n"+strings.Join(list, "\n"), guideEnemyMenu, tele.ModeHTML)
	})
	bot.Handle(&btnEnemyClear, func(c tele.Context) error {
		_, _ = db.Exec("DELETE FROM wolf_enemies WHERE owner_id = ?", c.Sender().ID)
		clearEnemiesCache(c.Sender().ID)
		return c.Send("🗑 <b>لیست دشمنان پاکسازی شد.</b>", guideEnemyMenu, tele.ModeHTML)
	})

	bot.Handle(&btnGFont, func(c tele.Context) error {
		en, mode := getFontSetting(c.Sender().ID)
		st := "🔴 خاموش"
		if en { st = "🟢 روشن" }
		text := fmt.Sprintf(`✒️ <b>سیستم خوشنویسی (Calligraphy)</b>
➖➖➖➖➖➖➖➖➖➖
📌 <b>وضعیت فعلی:</b> %s
🎨 <b>استایل فعلی فونت:</b> <code>%s</code>
➖➖➖➖➖➖➖➖➖➖
📖 <b>راهنمای عملکرد:</b>
با روشن کردن خوشنویسی، تمام پیام‌هایی که ارسال می‌کنید به صورت خودکار با فونت و استایل دلخواه (بولد، ایتالیک، مونو و...) فرمت می‌شوند.`, st, mode)
		return c.Send(text, guideFontMenu, tele.ModeHTML)
	})
	bot.Handle(&btnFontOn, func(c tele.Context) error {
		_, _ = db.Exec("UPDATE users SET is_font_enabled = TRUE WHERE id = ?", c.Sender().ID)
		_, mode := getFontSetting(c.Sender().ID)
		updateFontCache(c.Sender().ID, true, mode)
		return c.Send("🟢 <b>خوشنویسی روشن شد.</b>", guideFontMenu, tele.ModeHTML)
	})
	bot.Handle(&btnFontOff, func(c tele.Context) error {
		_, _ = db.Exec("UPDATE users SET is_font_enabled = FALSE WHERE id = ?", c.Sender().ID)
		_, mode := getFontSetting(c.Sender().ID)
		updateFontCache(c.Sender().ID, false, mode)
		return c.Send("🔴 <b>خوشنویسی خاموش شد.</b>", guideFontMenu, tele.ModeHTML)
	})
	setFontHandler := func(mode, label string) tele.HandlerFunc {
		return func(c tele.Context) error {
			_, _ = db.Exec("UPDATE users SET font_mode = ?, is_font_enabled = TRUE WHERE id = ?", mode, c.Sender().ID)
			updateFontCache(c.Sender().ID, true, mode)
			return c.Send(fmt.Sprintf("✅ <b>فونت به «%s» تغییر یافت و خوشنویسی فعال شد.</b>", label), guideFontMenu, tele.ModeHTML)
		}
	}
	bot.Handle(&btnFontBoldItalic, setFontHandler("bold_italic", "بولد ایتالیک"))
	bot.Handle(&btnFontBold, setFontHandler("bold", "بولد"))
	bot.Handle(&btnFontItalic, setFontHandler("italic", "ایتالیک"))
	bot.Handle(&btnFontUnderline, setFontHandler("underline", "زیر خط"))
	bot.Handle(&btnFontStrike, setFontHandler("strike", "خط خورده"))
	bot.Handle(&btnFontMono, setFontHandler("mono", "مونو"))
	bot.Handle(&btnFontSpoiler, setFontHandler("spoiler", "اسپویل"))

	bot.Handle(&btnGAction, func(c tele.Context) error {
		return c.Send("🎬 <b>راهنمای اکشن‌های جعلی (Fake Actions)</b>\n\nبا ارسال دستور <code>تایپینگ</code> یا <code>اکشن تایپ 30</code> در چت، وضعیت نوشتن (Typing) برای مخاطب نمایش داده می‌شود.", guideActionMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGPurge, func(c tele.Context) error {
		return c.Send("🗑 <b>راهنمای پاکسازی پیام‌ها (Purge)</b>\n\nبا ارسال دستور <code>پاکشو 20</code> در چت، ۲۰ پیام اخیر خودتان به سرعت پاکسازی می‌شود.", guidePurgeMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGTimer, func(c tele.Context) error {
		return c.Send("⏳ <b>راهنمای شمارش معکوس (Timer)</b>\n\nبا ارسال دستور <code>تایمر 10</code>، یک شمارش معکوس زیبا در پیام ایجاد می‌شود.", guideTimerMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGAutoReact, func(c tele.Context) error {
		return c.Send("🔥 <b>راهنمای ری‌اکشن خودکار (Auto React)</b>\n\nبا ریپلای روی پیام یک نفر و ارسال <code>ری‌اکشن ❤️</code>، ربات به طور خودکار به پیام‌های آن شخص ری‌اکشن می‌زند.", guideAutoReactMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGPV, func(c tele.Context) error {
		return c.Send("📩 <b>راهنمای ارسال به پیوی همه</b>\n\nبا ریپلای روی یک پیام و ارسال <code>پیوی همه</code>، آن پیام برای تمام مخاطبان پیوی ارسال می‌شود.", guidePVMenu, tele.ModeHTML)
	})
	bot.Handle(&btnGGroup, func(c tele.Context) error {
		return c.Send("👥 <b>راهنمای ارسال به گروه همه</b>\n\nبا ریپلای روی یک پیام و ارسال <code>گروه همه</code>، آن پیام برای تمام گروه‌های شما فروارد می‌شود.", guideGroupMenu, tele.ModeHTML)
	})

	backToGuideHandler := func(c tele.Context) error {
		var isClock, isEmoji, isBio, isFont bool
		var bioMode string
		_ = db.QueryRow("SELECT is_clock_enabled, is_emoji_enabled, is_bio_enabled, bio_mode, is_font_enabled FROM users WHERE id = ?", c.Sender().ID).Scan(&isClock, &isEmoji, &isBio, &bioMode, &isFont)
		
		clockStatus := "🔴 خاموش"
		if isClock { clockStatus = "🟢 روشن" }
		emojiStatus := "🔴 خاموش"
		if isEmoji { emojiStatus = "🟢 روشن" }
		bioStatus := "🔴 خاموش"
		if isBio {
			if bioMode == "custom" { bioStatus = "🟢 روشن (دستی)"
			} else { bioStatus = "🟢 روشن (رندوم)" }
		}
		fontStatus := "🔴 خاموش"
		if isFont { fontStatus = "🟢 روشن" }

		text := fmt.Sprintf(`📚 <b>بخش راهنما و امکانات سلف ولف 🐺</b>
➖➖➖➖➖➖➖➖➖➖
📊 <b>وضعیت لحظه‌ای قابلیت‌ها:</b>
▫️ ⏱ <b>ساعت زنده:</b> %s
▫️ 🎭 <b>اموجی رندوم:</b> %s
▫️ 📝 <b>بیوگرافی هوشمند:</b> %s
▫️ ✒️ <b>خوشنویسی پیام‌ها:</b> %s
➖➖➖➖➖➖➖➖➖➖
💡 <i>جهت مطالعه راهنما و تنظیم قابلیت‌ها، از کیبورد ثابت زیر استفاده کنید:</i>`,
			clockStatus, emojiStatus, bioStatus, fontStatus)
		
		return c.Send(text, guideMenu, tele.ModeHTML)
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
	bot.Handle(&btnGBackMain, func(c tele.Context) error { return c.Send("🔙 بازگشت به منوی اصلی", getMainKeyboard(c.Sender().ID), tele.ModeHTML) })

	bot.Handle(&btnWallet, func(c tele.Context) error {
		if IsUserBlocked(c.Sender().ID) { return c.Send("❌ مسدود هستید") }
		stateMu.Lock()
		userWalletTemp[c.Sender().ID] = 0
		stateMu.Unlock()
		return c.Send(fmt.Sprintf("👛 <b>موجودی:</b> %s تومان\nاز کیبورد شیشه‌ای مبلغ را انتخاب کنید:", formatMoney(GetUserBalance(c.Sender().ID))), getWalletInlineKeyboard(), walletReplyMenu, tele.ModeHTML)
	})

	bot.Handle(&tele.Btn{Unique: "wallet_change"}, func(c tele.Context) error {
		userID := c.Sender().ID
		val, _ := strconv.Atoi(c.Data())
		stateMu.Lock()
		current := userWalletTemp[userID] + val
		if current < 0 { current = 0 }
		userWalletTemp[userID] = current
		stateMu.Unlock()
		_ = c.Edit(fmt.Sprintf("👛 <b>مبلغ قابل پرداخت:</b> <code>%s تومان</code>", formatMoney(current)), getWalletInlineKeyboard(), tele.ModeHTML)
		return c.Respond()
	})

	bot.Handle(&btnWalletConfirm, func(c tele.Context) error {
		stateMu.RLock()
		amount := userWalletTemp[c.Sender().ID]
		stateMu.RUnlock()
		if amount <= 0 { return c.Send("❌ مبلغی انتخاب نکردید.", tele.ModeHTML) }
		text := fmt.Sprintf("💳 <b>واریز به:</b>\n%s\n%s\nمبلغ: %s تومان\n📸 عکس فیش را بفرستید.", GetSetting("card_bank"), GetSetting("card_number"), formatMoney(amount))
		return c.Send(text, waitingReceiptMenu, tele.ModeHTML)
	})

	bot.Handle(&btnCancelReceipt, func(c tele.Context) error {
		stateMu.Lock()
		userWalletTemp[c.Sender().ID] = 0
		stateMu.Unlock()
		return c.Send("❌ لغو شد.", getMainKeyboard(c.Sender().ID), tele.ModeHTML)
	})

	bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		user := c.Sender()
		if IsUserBlocked(user.ID) { return c.Send("❌ مسدود") }
		stateMu.RLock()
		amount := userWalletTemp[user.ID]
		stateMu.RUnlock()
		if amount <= 0 { return c.Send("📸 تصویر دریافت شد.") }
		res, err := db.Exec(`INSERT INTO transactions (user_id, amount, status) VALUES (?, ?, 'pending')`, user.ID, amount)
		if err != nil { return c.Send("❌ خطا در سرور") }
		txID, _ := res.LastInsertId()
		
		menu := &tele.ReplyMarkup{}
		btnApprove := menu.Data("✅ تایید", "admin_approve", strconv.FormatInt(txID, 10))
		btnReject := menu.Data("❌ رد", "admin_reject", strconv.FormatInt(txID, 10))
		menu.Inline(menu.Row(btnApprove, btnReject))
		
		photo := c.Message().Photo
		photo.Caption = fmt.Sprintf("🔔 فیش %d\nمبلغ: %d\nکاربر: %d", txID, amount, user.ID)
		for _, adminID := range cfg.AdminIDs {
			_, _ = bot.Send(&tele.User{ID: adminID}, photo, menu, tele.ModeHTML)
		}
		stateMu.Lock()
		userWalletTemp[user.ID] = 0
		stateMu.Unlock()
		return c.Send("✅ فیش برای مدیریت ارسال شد.", getMainKeyboard(user.ID))
	})

	bot.Handle(&btnBuy, func(c tele.Context) error {
		keys := GetUserBalance(c.Sender().ID) / getKeyPrice()
		if GetUserSelfStatus(c.Sender().ID) == "روشن" {
			return c.Send(fmt.Sprintf("🎉 سلف فعال است! کلید: %d", keys), getKeyboard(c.Sender().ID), tele.ModeHTML)
		}
		if keys < 30 {
			return c.Send(fmt.Sprintf("❌ حداقل ۳۰ کلید نیاز است. شما %d کلید دارید.", keys), tele.ModeHTML)
		}
		return c.Send("🎉 کلید کافیست. تایید کنید:", confirmSelfMenu, tele.ModeHTML)
	})

	bot.Handle(&btnTurnOnSelf, func(c tele.Context) error {
		if GetUserSelfStatus(c.Sender().ID) == "روشن" { return c.Send("سلف روشن است") }
		if _, err := os.Stat(fmt.Sprintf("/opt/wolf/sessions/user_%d.json", c.Sender().ID)); err == nil {
			_, _ = db.Exec("UPDATE users SET self_status = 'روشن' WHERE id = ?", c.Sender().ID)
			startUserbot(c.Sender().ID, cfg, bot)
			return c.Send("🟢 سلف روشن شد.", getKeyboard(c.Sender().ID))
		}
		return c.Send("❌ سلف ندارید.")
	})

	bot.Handle(&btnTurnOffSelf, func(c tele.Context) error {
		_, _ = db.Exec("UPDATE users SET self_status = 'خاموش' WHERE id = ?", c.Sender().ID)
		stopUserbot(c.Sender().ID)
		return c.Send("🔴 سلف خاموش شد.", getKeyboard(c.Sender().ID))
	})

	bot.Handle(&btnExitSelf, func(c tele.Context) error {
		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(menu.Data("🛑 بله", "exit_confirm"), menu.Data("❌ خیر", "exit_cancel")))
		return c.Send("آیا مطمئن هستید؟ نشست پاک میشود.", menu)
	})

	bot.Handle(&tele.Btn{Unique: "exit_confirm"}, func(c tele.Context) error {
		stopUserbot(c.Sender().ID)
		_ = os.Remove(fmt.Sprintf("/opt/wolf/sessions/user_%d.json", c.Sender().ID))
		_, _ = db.Exec("UPDATE users SET self_status = 'خروج' WHERE id = ?", c.Sender().ID)
		return c.Send("🛑 خروج انجام شد.", getKeyboard(c.Sender().ID))
	})

	bot.Handle(&tele.Btn{Unique: "exit_cancel"}, func(c tele.Context) error { return c.Send("✅ لغو شد.") })

	bot.Handle(&btnConfirmSelfAction, func(c tele.Context) error {
		stateMu.Lock()
		userStates[c.Sender().ID] = &UserState{Action: "waiting_for_contact"}
		stateMu.Unlock()
		menu := &tele.ReplyMarkup{ResizeKeyboard: true}
		menu.Reply(menu.Row(menu.Contact("📱 ارسال شماره (Share Contact)")), menu.Row(menu.Text("🔙 بازگشت")))
		return c.Send("شماره خود را بفرستید:", menu)
	})

	bot.Handle(tele.OnContact, func(c tele.Context) error {
		userID := c.Sender().ID
		stateMu.RLock()
		st, ok := userStates[userID]
		stateMu.RUnlock()
		if !ok || st.Action != "waiting_for_contact" { return nil }

		cc, pc, rc := make(chan string, 1), make(chan string, 1), make(chan AuthResult, 1)
		ctx, cancel := context.WithCancel(context.Background())

		stateMu.Lock()
		userStates[userID] = &UserState{Action: "waiting_for_code", Phone: c.Message().Contact.PhoneNumber, CodeChan: cc, PasswordChan: pc, ResultChan: rc, Cancel: cancel}
		stateMu.Unlock()

		go startTelegramLogin(ctx, userID, cfg, &botAuthenticator{phone: c.Message().Contact.PhoneNumber, codeChan: cc, passwordChan: pc, resultChan: rc})
		
		menu := &tele.ReplyMarkup{ResizeKeyboard: true}
		menu.Reply(menu.Row(menu.Text("🔙 بازگشت")))
		return c.Send("✅ کد ۵ رقمی را با فاصله بفرستید:", menu)
	})

	bot.Handle(&btnAdminPanel, func(c tele.Context) error {
		if !cfg.IsAdmin(c.Sender().ID) { return nil }
		return c.Send(getAdminDashboard(), adminPanelMenu, tele.ModeHTML)
	})
	bot.Handle(&btnConfigAccount, func(c tele.Context) error { return c.Send("تنظیمات بانک:", accountConfigMenu) })
	bot.Handle(&btnConfigCardNum, func(c tele.Context) error {
		stateMu.Lock()
		adminStates[c.Sender().ID] = AdminAction{Action: "set_card_num"}
		stateMu.Unlock()
		return c.Send("شماره جدید را بفرستید:")
	})
	bot.Handle(&btnConfigSupport, func(c tele.Context) error { return c.Send("تنظیمات پشتیبانی:", supportConfigMenu) })

	bot.Handle(&tele.Btn{Unique: "admin_approve"}, func(c tele.Context) error {
		txID, _ := strconv.ParseInt(c.Data(), 10, 64)
		tx, _ := db.Begin()
		defer tx.Rollback()
		var stat string
		var tUID int64
		var amt int
		_ = tx.QueryRow("SELECT status, user_id, amount FROM transactions WHERE id = ? FOR UPDATE", txID).Scan(&stat, &tUID, &amt)
		if stat != "pending" { return c.Respond(&tele.CallbackResponse{Text: "قبلا بررسی شده"}) }
		_, _ = tx.Exec("UPDATE transactions SET status = 'approved' WHERE id = ?", txID)
		_, _ = tx.Exec(`INSERT IGNORE INTO users (id, first_name, username) VALUES (?, 'کاربر', 'ثبت_نشده')`, tUID)
		_, _ = tx.Exec(`INSERT INTO wallets (user_id, balance) VALUES (?, ?) ON DUPLICATE KEY UPDATE balance = balance + ?`, tUID, amt, amt)
		_ = tx.Commit()
		_, _ = bot.Send(&tele.User{ID: tUID}, fmt.Sprintf("✅ کیف شما شارژ شد: %d", amt))
		_ = c.Reply("✅ تایید شد.")
		return c.Respond()
	})

	bot.Handle(tele.OnText, func(c tele.Context) error {
		if HandleWolfPlusText(c) { return nil }
		
		userID := c.Sender().ID
		txt := strings.TrimSpace(c.Text())

		stateMu.RLock()
		st, ok := userStates[userID]
		stateMu.RUnlock()
		if ok && st != nil {
			if st.Action == "waiting_for_code" {
				cleanCode := extractDigits(txt)
				select {
				case st.CodeChan <- cleanCode:
					select {
					case res := <-st.ResultChan:
						if res.Type == AuthResultNeeds2FA {
							stateMu.Lock()
							st.Action = "waiting_for_password"
							stateMu.Unlock()
							return c.Send("🔒 رمز عبور دو مرحله‌ای را وارد کنید:")
						} else if res.Type == AuthResultSuccess {
							_, _ = db.Exec("UPDATE users SET self_status = 'روشن', phone = ?, last_billed_at = NOW() WHERE id = ?", st.Phone, userID)
							startUserbot(userID, cfg, bot)
							stateMu.Lock()
							delete(userStates, userID)
							stateMu.Unlock()
							return c.Send("🎉 سلف روشن شد!", getKeyboard(userID))
						}
					case <-time.After(35 * time.Second): return c.Send("تایم‌اوت")
					}
				default: return nil
				}
			} else if st.Action == "waiting_for_password" {
				select {
				case st.PasswordChan <- txt:
					select {
					case res := <-st.ResultChan:
						if res.Type == AuthResultSuccess {
							_, _ = db.Exec("UPDATE users SET self_status = 'روشن', phone = ?, last_billed_at = NOW() WHERE id = ?", st.Phone, userID)
							startUserbot(userID, cfg, bot)
							stateMu.Lock()
							delete(userStates, userID)
							stateMu.Unlock()
							return c.Send("🎉 رمز درست بود، سلف روشن شد!", getKeyboard(userID))
						}
					case <-time.After(35 * time.Second): return c.Send("تایم‌اوت")
					}
				default: return nil
				}
			}
		}

		if cfg.IsAdmin(userID) {
			stateMu.RLock()
			ast, aok := adminStates[userID]
			stateMu.RUnlock()
			if aok {
				switch ast.Action {
				case "set_card_num": SetSetting("card_number", txt); c.Send("✅ ذخیره شد")
				}
				stateMu.Lock()
				delete(adminStates, userID)
				stateMu.Unlock()
				return nil
			}
		}
		return nil
	})

	bot.Handle(&btnSupport, func(c tele.Context) error {
		return c.Send(fmt.Sprintf("%s\n\n🆔 %s", GetSetting("support_text"), GetSetting("support_id")), tele.ModeHTML)
	})

	startBillingWorker(bot)
	startClockWorker()
	startEmojiWorker()
	startBioWorker()

	rows, err := db.Query("SELECT id FROM users WHERE self_status = 'روشن'")
	if err == nil {
		for rows.Next() {
			var uid int64
			if err := rows.Scan(&uid); err == nil { startUserbot(uid, cfg, bot) }
		}
		rows.Close()
	}

	log.Println("⚡ ربات ولف سلف (Clean Architecture) آماده است!")
	bot.Start()
}
