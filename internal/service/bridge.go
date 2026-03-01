/*
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *  Wagram Service — WA ↔ TG Message Relay
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *
 *  Coordinates bidirectional message forwarding:
 *
 *  WhatsApp → Telegram:
 *    Incoming WA messages (text + media) are matched
 *    against the mapping DB, then forwarded to each
 *    linked Telegram chat with sender info in caption.
 *
 *  Telegram → WhatsApp:
 *    Non-command TG messages (text + media) are looked
 *    up in the mapping DB, then sent to the linked
 *    WhatsApp JID.
 */
package service

import (
	"context"
	"fmt"
	"log"

	"telegram-wa/internal/domain"
)

/**
 * WagramService wires the WhatsApp and Telegram services
 * together, handling message relay in both directions.
 */
type WagramService struct {
	repo domain.MappingRepository /* Chat mapping persistence */
	wa   *WhatsAppService         /* WhatsApp connection      */
	tg   *TelegramService         /* Telegram bot connection  */
}

/**
 * NewWagramService creates the wagram service and registers
 * itself as the handler for incoming WA messages.
 */
func NewWagramService(repo domain.MappingRepository, wa *WhatsAppService, tg *TelegramService) *WagramService {
	b := &WagramService{
		repo: repo,
		wa:   wa,
		tg:   tg,
	}
	wa.SetMessageHandler(b.handleWAMessage)
	return b
}

/* ── Telegram → WhatsApp ─────────────────────── */

/**
 * HandleTGMessage forwards a plain text message
 * from Telegram to the mapped WhatsApp chat.
 */
func (b *WagramService) HandleTGMessage(chatID int64, senderName, username, text string) {
	waChatID, err := b.repo.GetByTG(chatID)
	if err != nil {
		log.Printf("wagram: db lookup TG %d: %v", chatID, err)
		return
	}
	if waChatID == "" {
		return
	}

	// Format: [senderName, username]: text
	formatted := fmt.Sprintf("Telegram: %s", text)
	if err := b.wa.SendText(context.Background(), waChatID, formatted); err != nil {
		log.Printf("wagram: send to WA %s: %v", waChatID, err)
	}
}

/**
 * HandleTGMedia downloads a media file from Telegram
 * and re-uploads it to the mapped WhatsApp chat.
 */
func (b *WagramService) HandleTGMedia(chatID int64, fileID string, mediaType domain.MediaType, filename, caption string) {
	waChatID, err := b.repo.GetByTG(chatID)
	if err != nil {
		log.Printf("wagram: db lookup TG %d: %v", chatID, err)
		return
	}
	if waChatID == "" {
		return
	}

	data, err := b.tg.DownloadFile(fileID)
	if err != nil {
		log.Printf("wagram: download TG file: %v", err)
		return
	}

	if err := b.wa.SendMedia(context.Background(), waChatID, data, mediaType, filename, caption); err != nil {
		log.Printf("wagram: send media to WA %s: %v", waChatID, err)
	}
}

/* ── WhatsApp → Telegram ─────────────────────── */

/**
 * handleWAMessage is the callback invoked by WhatsAppService
 * whenever a new message arrives. It looks up mapped TG chats
 * and forwards the message (text or media) to each one.
 */
func (b *WagramService) handleWAMessage(msg domain.IncomingWAMessage) {
	log.Printf("wagram: WA msg from %s in chat %s (media=%d)", msg.SenderPhone, msg.ChatJID, msg.MediaType)

	tgChatIDs, err := b.repo.GetByWA(msg.ChatJID)
	if err != nil {
		log.Printf("wagram: db lookup WA %s: %v", msg.ChatJID, err)
		return
	}
	if len(tgChatIDs) == 0 {
		log.Printf("wagram: no TG mapping for WA chat %s", msg.ChatJID)
		return
	}

	senderName := msg.SenderName
	if senderName == "" {
		senderName = "Unknown"
	}

	for _, tgID := range tgChatIDs {

		/* ── Forward media message ─── */
		if len(msg.MediaData) > 0 {
			caption := msg.Caption
			if caption == "" && msg.Text != "" {
				caption = msg.Text
			}

			var header string
			if msg.IsFromMe {
				header = "Me (WhatsApp)"
			} else {
				header = fmt.Sprintf("%s (%s)", senderName, msg.SenderPhone)
			}
			if caption != "" {
				caption = fmt.Sprintf("%s:\n%s", header, caption)
			} else {
				caption = header
			}

			if err := b.tg.SendMedia(tgID, msg.MediaData, msg.MediaType, msg.FileName, caption); err != nil {
				log.Printf("wagram: send media to TG %d: %v", tgID, err)
			}
			continue
		}

		/* ── Forward text message ──── */
		if msg.Text == "" {
			continue
		}
		var formatted string
		if msg.IsFromMe {
			formatted = fmt.Sprintf("Me (WhatsApp): %s", msg.Text)
		} else {

			// Format: [msg.SenderName, msg.SenderPhone]: msg.Text
			formatted = fmt.Sprintf("WhatsApp: %s", msg.Text)
		}

		if err := b.tg.SendMessage(tgID, formatted); err != nil {
			log.Printf("wagram: send to TG %d: %v", tgID, err)
		}
	}
}
