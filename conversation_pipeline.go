package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var conversationSchemaMu sync.Mutex
var conversationSchemaReady bool
var conversationHandlersMu sync.Mutex
var conversationHandlers = map[*whatsmeow.Client]uint32{}

func ensureConversationPipelineTables() error {
	conversationSchemaMu.Lock()
	defer conversationSchemaMu.Unlock()
	if conversationSchemaReady {
		return nil
	}
	if userDB == nil {
		return context.Canceled
	}
	_, err := userDB.Exec(`
CREATE TABLE IF NOT EXISTS public.conversations (
 account_key TEXT NOT NULL,
 chat_jid TEXT NOT NULL,
 consent_status TEXT NOT NULL DEFAULT 'active',
 human_handoff BOOLEAN NOT NULL DEFAULT false,
 last_inbound_at TIMESTAMPTZ,
 last_outbound_at TIMESTAMPTZ,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(account_key,chat_jid)
);
CREATE TABLE IF NOT EXISTS public.conversation_messages (
 id BIGSERIAL PRIMARY KEY,
 account_key TEXT NOT NULL,
 chat_jid TEXT NOT NULL,
 event_key TEXT NOT NULL,
 provider_message_id TEXT,
 reply_to_message_id TEXT,
 direction TEXT NOT NULL,
 body TEXT NOT NULL,
 status TEXT NOT NULL,
 error TEXT,
 next_attempt_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 sent_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS conversation_messages_event_idx ON public.conversation_messages(account_key,event_key);
CREATE INDEX IF NOT EXISTS conversation_messages_queue_idx ON public.conversation_messages(status,next_attempt_at,id);
CREATE INDEX IF NOT EXISTS conversation_messages_provider_idx ON public.conversation_messages(account_key,provider_message_id);
`)
	if err != nil {
		return err
	}
	conversationSchemaReady = true
	return nil
}

func normalizeInboundText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func inboundMessageText(message *waProto.Message) string {
	if message == nil {
		return ""
	}
	if text := message.GetConversation(); text != "" {
		return text
	}
	if extended := message.GetExtendedTextMessage(); extended != nil {
		return extended.GetText()
	}
	return ""
}

func conversationTarget(event *events.Message) (types.JID, bool) {
	if event == nil {
		return types.JID{}, false
	}
	chat := event.Info.Chat
	if chat.IsEmpty() {
		chat = event.Info.Sender
	}
	if chat.Server == types.HiddenUserServer {
		if !event.Info.SenderAlt.IsEmpty() {
			chat = event.Info.SenderAlt
		} else if event.Info.Sender.Server == types.DefaultUserServer {
			chat = event.Info.Sender
		}
	}
	if chat.IsEmpty() || chat.Server != types.DefaultUserServer || strings.TrimSpace(chat.User) == "" {
		return types.JID{}, false
	}
	return chat, true
}

func conversationCommand(body string) string {
	return strings.ToUpper(strings.Trim(strings.TrimSpace(body), ".,!?"))
}

const optOutConfirmationReply = "You’re unsubscribed from automated messages. Reply START if you want to resume."

func isOptOutCommand(command string) bool {
	switch command {
	case "STOP", "UNSUBSCRIBE", "OPT OUT", "OPTOUT", "OPT-OUT", "SAIR", "PARAR", "CANCELAR", "BASTA", "BAJA", "ARRET":
		return true
	}
	return false
}

func isResumeCommand(command string) bool {
	return command == "START" || command == "SUBSCRIBE"
}

func isOptOutConfirmationReply(body string) bool {
	return conversationCommand(body) == conversationCommand(optOutConfirmationReply)
}

func recordInboundConversation(accountKey string, target types.JID, messageID, body string) (string, bool, error) {
	if err := ensureConversationPipelineTables(); err != nil {
		return "", false, err
	}
	tx, err := userDB.Begin()
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	chat := target.String()
	result, err := tx.Exec(`INSERT INTO public.conversation_messages(account_key,chat_jid,event_key,direction,body,status,created_at) VALUES($1,$2,$3,'inbound',$4,'received',now()) ON CONFLICT(account_key,event_key) DO NOTHING`, accountKey, chat, messageID, body)
	if err != nil {
		return "", false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return "", false, err
	}
	if rows == 0 {
		return "", true, nil
	}
	if _, err = tx.Exec(`INSERT INTO public.conversations(account_key,chat_jid,consent_status,updated_at) VALUES($1,$2,'active',now()) ON CONFLICT(account_key,chat_jid) DO NOTHING`, accountKey, chat); err != nil {
		return "", false, err
	}
	var consent string
	var handoff bool
	if err = tx.QueryRow(`SELECT consent_status,human_handoff FROM public.conversations WHERE account_key=$1 AND chat_jid=$2 FOR UPDATE`, accountKey, chat).Scan(&consent, &handoff); err != nil {
		return "", false, err
	}
	command := conversationCommand(body)
	reply := ""
	switch command {
	case "STOP", "UNSUBSCRIBE", "OPT OUT", "OPTOUT", "OPT-OUT", "SAIR", "PARAR", "CANCELAR", "BASTA", "BAJA", "ARRET":
		consent = "opted_out"
		handoff = false
		reply = optOutConfirmationReply
	case "START", "SUBSCRIBE":
		consent = "active"
		handoff = false
		reply = "Automated replies are enabled again. How can I help?"
	case "HELP":
		if consent != "opted_out" {
			consent = "active"
			reply = "I can help with your request. Reply HUMAN if you want a person to join the conversation."
		}
	case "HUMAN", "AGENT", "PERSON":
		if consent != "opted_out" {
			consent = "active"
			handoff = true
			reply = "I’ll hand this conversation to a person. Please wait for a reply."
		}
	default:
		if consent != "opted_out" && !handoff {
			consent = "active"
			reply = "Thanks for your message. I’ve received it and will get back to you shortly."
		}
	}
	if _, err = tx.Exec(`UPDATE public.conversations SET consent_status=$3,human_handoff=$4,last_inbound_at=now(),updated_at=now() WHERE account_key=$1 AND chat_jid=$2`, accountKey, chat, consent, handoff); err != nil {
		return "", false, err
	}
	if err = tx.Commit(); err != nil {
		return "", false, err
	}
	if phone, phoneErr := normalizeRecipientPhone(target.User); phoneErr == nil && phone != "" {
		if isOptOutCommand(command) {
			if optErr := markRecipientOptOut(context.Background(), phone, command, accountKey); optErr != nil {
				fmt.Printf("global opt-out record failed for %s: %v\n", phone, optErr)
			}
		} else if isResumeCommand(command) {
			_ = removeRecipientOptOut(context.Background(), phone)
		}
	}
	return reply, false, nil
}

func enqueueConversationReply(accountKey string, target types.JID, inboundID, body string) error {
	if err := ensureConversationPipelineTables(); err != nil {
		return err
	}
	_, err := userDB.Exec(`INSERT INTO public.conversation_messages(account_key,chat_jid,event_key,reply_to_message_id,direction,body,status,created_at) VALUES($1,$2,$3,$4,'outbound',$5,'queued',now()) ON CONFLICT(account_key,event_key) DO NOTHING`, accountKey, target.String(), "reply:"+inboundID, inboundID, body)
	return err
}

func registerConversationPipeline(accountKey string, client *whatsmeow.Client) {
	if client == nil || strings.TrimSpace(accountKey) == "" {
		return
	}
	conversationHandlersMu.Lock()
	defer conversationHandlersMu.Unlock()
	if _, loaded := conversationHandlers[client]; loaded {
		return
	}
	handlerID := client.AddEventHandler(func(raw any) {
		switch event := raw.(type) {
		case *events.Message:
			if event.Info.IsFromMe {
				return
			}
			go handleInboundConversationEvent(accountKey, client, event)
		case *events.Receipt:
			handleConversationReceipt(accountKey, event)
		}
	})
	conversationHandlers[client] = handlerID
}

func unregisterConversationPipeline(client *whatsmeow.Client) {
	if client == nil {
		return
	}
	conversationHandlersMu.Lock()
	handlerID, registered := conversationHandlers[client]
	delete(conversationHandlers, client)
	conversationHandlersMu.Unlock()
	if registered {
		client.RemoveEventHandler(handlerID)
	}
}

func handleInboundConversationEvent(accountKey string, client *whatsmeow.Client, event *events.Message) {
	body := normalizeInboundText(inboundMessageText(event.Message))
	if body == "" || event.Info.ID == "" {
		return
	}
	target, ok := conversationTarget(event)
	if !ok {
		return
	}
	reply, duplicate, err := recordInboundConversation(accountKey, target, event.Info.ID, body)
	if err != nil || duplicate || reply == "" {
		return
	}
	if getAdminSetting("conversation_auto_reply_enabled", "true") != "true" {
		return
	}
	if err := enqueueConversationReply(accountKey, target, event.Info.ID, reply); err != nil {
		return
	}
	_ = client
}

func conversationTemporaryError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return isTemporaryRecipientLookupError(err) || strings.Contains(text, "temporarily unavailable") || strings.Contains(text, "rate")
}

func processConversationQueue() {
	if err := ensureConversationPipelineTables(); err != nil {
		return
	}
	rows, err := userDB.Query(`SELECT id,account_key,chat_jid,body,provider_message_id FROM public.conversation_messages WHERE direction='outbound' AND status='queued' AND (next_attempt_at IS NULL OR next_attempt_at<=now()) ORDER BY id LIMIT 20`)
	if err != nil {
		return
	}
	type queuedReply struct {
		id                                 int64
		accountKey, chat, body, providerID string
	}
	var queue []queuedReply
	for rows.Next() {
		var item queuedReply
		if rows.Scan(&item.id, &item.accountKey, &item.chat, &item.body, &item.providerID) == nil {
			queue = append(queue, item)
		}
	}
	rows.Close()
	for _, item := range queue {
		claimed, err := userDB.Exec(`UPDATE public.conversation_messages SET status='sending',error=NULL WHERE id=$1 AND status='queued'`, item.id)
		if err != nil {
			continue
		}
		n, _ := claimed.RowsAffected()
		if n == 0 {
			continue
		}
		session := getSession(item.accountKey)
		if session == nil || session.client == nil || !session.client.IsLoggedIn() || !session.client.IsConnected() {
			_, _ = userDB.Exec(`UPDATE public.conversation_messages SET status='queued',error=$1,next_attempt_at=now()+interval '30 seconds' WHERE id=$2`, "WhatsApp account is not connected", item.id)
			continue
		}
		target, err := types.ParseJID(item.chat)
		if err != nil {
			_, _ = userDB.Exec(`UPDATE public.conversation_messages SET status='failed',error=$1 WHERE id=$2`, err.Error(), item.id)
			continue
		}
		chatPhone, chatPhoneErr := normalizeRecipientPhone(target.User)
		if chatPhoneErr != nil || chatPhone == "" {
			_, _ = userDB.Exec(`UPDATE public.conversation_messages SET status='failed',error=$1 WHERE id=$2`, "invalid recipient phone", item.id)
			continue
		}
		sendCtx := context.Background()
		if isOptOutConfirmationReply(item.body) {
			sendCtx = WithSafetyBypass(sendCtx)
		} else {
			sendCtx = WithCooldownBypass(sendCtx)
			if suppressed, supErr := isRecipientSuppressed(sendCtx, chatPhone); supErr == nil && suppressed {
				_, _ = userDB.Exec(`UPDATE public.conversation_messages SET status='failed',error=$1 WHERE id=$2`, "recipient has opted out; send suppressed", item.id)
				continue
			}
		}
		if item.providerID == "" {
			item.providerID = string(session.client.GenerateMessageID())
			_, _ = userDB.Exec(`UPDATE public.conversation_messages SET provider_message_id=$1 WHERE id=$2`, item.providerID, item.id)
		}
		session.mu.Lock()
		sendErr := safeSendMessageWithIDContext(sendCtx, item.accountKey, session.client, target, item.body, types.MessageID(item.providerID))
		session.mu.Unlock()
		if sendErr != nil {
			if conversationTemporaryError(sendErr) {
				_, _ = userDB.Exec(`UPDATE public.conversation_messages SET status='queued',error=$1,next_attempt_at=now()+interval '90 seconds' WHERE id=$2`, sendErr.Error(), item.id)
			} else {
				_, _ = userDB.Exec(`UPDATE public.conversation_messages SET status='failed',error=$1 WHERE id=$2`, sendErr.Error(), item.id)
			}
			continue
		}
		_, _ = userDB.Exec(`UPDATE public.conversation_messages SET status='sent',sent_at=now(),error=NULL WHERE id=$1`, item.id)
		_, _ = userDB.Exec(`UPDATE public.conversations SET last_outbound_at=now(),updated_at=now() WHERE account_key=$1 AND chat_jid=$2`, item.accountKey, item.chat)
	}
}

func conversationWorkerLoop() {
	for userDB == nil {
		time.Sleep(2 * time.Second)
	}
	for {
		processConversationQueue()
		time.Sleep(2 * time.Second)
	}
}

func handleConversationReceipt(accountKey string, receipt *events.Receipt) {
	if receipt == nil || userDB == nil {
		return
	}
	status := ""
	switch receipt.Type {
	case types.ReceiptTypeRead, types.ReceiptTypeReadSelf, types.ReceiptTypePlayed:
		status = "read"
	case types.ReceiptTypeDelivered, types.ReceiptTypeSender:
		status = "delivered"
	default:
		return
	}
	for _, id := range receipt.MessageIDs {
		_, _ = userDB.Exec(`UPDATE public.conversation_messages SET status=CASE WHEN $1='read' OR status NOT IN ('read') THEN $1 ELSE status END WHERE account_key=$2 AND provider_message_id=$3 AND direction='outbound'`, status, accountKey, id)
	}
}

func sendTextMessage(ctx context.Context, client *whatsmeow.Client, target types.JID, text string, messageID types.MessageID) error {
	message := &waProto.Message{Conversation: protoString(text)}
	if messageID == "" {
		_, err := client.SendMessage(ctx, target, message)
		return err
	}
	_, err := client.SendMessage(ctx, target, message, whatsmeow.SendRequestExtra{ID: messageID})
	return err
}
func protoString(value string) *string { return &value }
func init()                            { go conversationWorkerLoop() }
