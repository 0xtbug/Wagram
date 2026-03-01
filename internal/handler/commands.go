/*
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *  Handler Layer — Telegram Bot Commands
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *
 *  Registers and handles all Telegram bot commands
 *  and the catch-all media/text forwarder.
 *
 *  Commands:
 *    /start    – Welcome message with usage instructions
 *    /scan     – Generate WhatsApp QR code for login
 *    /status   – Show WA connection & login status
 *    /list     – List all active chat mappings
 *    /bridge   – Link this TG chat to a WA contact
 *    /unbridge – Remove link for this TG chat
 *
 *  Catch-all:
 *    Any non-command message (text, photo, video,
 *    audio, voice, document, sticker) is forwarded
 *    to the mapped WhatsApp chat via WagramService.
 */
package handler

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	qrcode "github.com/skip2/go-qrcode"

	"telegram-wa/internal/domain"
	"telegram-wa/internal/service"
)

/**
 * CommandHandler holds all dependencies needed
 * by the bot command handlers.
 */
type CommandHandler struct {
	wa     domain.WhatsAppClient    /* WA connection (interface)   */
	repo   domain.MappingRepository /* Chat mapping DB (interface) */
	wagram *service.WagramService   /* Bidirectional relay         */
}

/**
 * Register wires all command handlers and the catch-all
 * message forwarder into the given dispatcher.
 *
 * ⚠ The catch-all handler MUST be registered last so that
 *   commands are matched before falling through.
 */
func Register(
	d *ext.Dispatcher,
	wa domain.WhatsAppClient,
	repo domain.MappingRepository,
	wagram *service.WagramService,
) {
	h := &CommandHandler{wa: wa, repo: repo, wagram: wagram}

	d.AddHandler(handlers.NewCommand("start", h.start))
	d.AddHandler(handlers.NewCommand("scan", h.scan))
	d.AddHandler(handlers.NewCommand("status", h.status))
	d.AddHandler(handlers.NewCommand("list", h.list))
	d.AddHandler(handlers.NewCommand("bridge", h.bridgeCmd))
	d.AddHandler(handlers.NewCommand("unbridge", h.unbridge))
	d.AddHandler(handlers.NewCallback(callbackquery.Prefix("bridge_wa_"), h.bridgeCallback))

	d.AddHandler(handlers.NewMessage(message.All, h.forwardToWA))
}

/* ═══════════════════════════════════════════════
 *  /start — Welcome Message
 * ═══════════════════════════════════════════════ */

/**
 * start replies with a Markdown welcome message
 * listing all available commands.
 */
func (h *CommandHandler) start(bot *gotgbot.Bot, ctx *ext.Context) error {
	text := "👋 *Welcome to Wagram!*\n\n" +
		"1. /scan - Log into WhatsApp\n" +
		"2. /bridge - Link this chat to a WhatsApp contact\n" +
		"3. /unbridge - Remove the link\n" +
		"4. /status - Check connection status\n" +
		"5. /list - Show active links"
	_, err := ctx.EffectiveMessage.Reply(bot, text, &gotgbot.SendMessageOpts{ParseMode: "Markdown"})
	return err
}

/* ═══════════════════════════════════════════════
 *  /scan — WhatsApp QR Login
 * ═══════════════════════════════════════════════ */

/**
 * scan initiates the WhatsApp QR login flow:
 *   1. Sends a "Generating..." placeholder
 *   2. Listens for QR events from whatsmeow
 *   3. Renders the QR code as a PNG image (local)
 *   4. Sends it as a Telegram photo
 *   5. Reports success or timeout
 */
func (h *CommandHandler) scan(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg, err := ctx.EffectiveMessage.Reply(bot, "⏳ Generating WhatsApp QR code...", nil)
	if err != nil {
		return err
	}

	qrChan, err := h.wa.GenerateQR(context.Background())
	if err != nil {
		_, _, _ = bot.EditMessageText(
			fmt.Sprintf("❌ %s", err.Error()),
			&gotgbot.EditMessageTextOpts{ChatId: msg.Chat.Id, MessageId: msg.MessageId},
		)
		return nil
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			png, qrErr := qrcode.Encode(evt.Code, qrcode.Medium, 512)
			if qrErr != nil {
				_, _, _ = bot.EditMessageText("❌ Failed to generate QR image.", &gotgbot.EditMessageTextOpts{
					ChatId: msg.Chat.Id, MessageId: msg.MessageId,
				})
				continue
			}

			_, _ = bot.SendPhoto(msg.Chat.Id, gotgbot.InputFileByReader("qr.png", bytes.NewReader(png)), &gotgbot.SendPhotoOpts{
				Caption: "📱 Scan this QR code with WhatsApp\n⏰ Expires soon!",
			})

			_, _ = bot.DeleteMessage(msg.Chat.Id, msg.MessageId, nil)

		case "success":
			_, _ = bot.SendMessage(msg.Chat.Id, "✅ WhatsApp login successful!", nil)
			return nil
		case "timeout":
			_, _ = bot.SendMessage(msg.Chat.Id, "⏰ QR code expired. Send /scan again.", nil)
			return nil
		}
	}
	return nil
}

/* ═══════════════════════════════════════════════
 *  /status — Connection Status
 * ═══════════════════════════════════════════════ */

/**
 * status reports the current WhatsApp connection
 * and authentication state.
 */
func (h *CommandHandler) status(bot *gotgbot.Bot, ctx *ext.Context) error {
	connected := h.wa.IsConnected()
	loggedIn := h.wa.IsLoggedIn()
	text := fmt.Sprintf(
		"📊 *Wagram Status*\n\nWhatsApp Connected: `%v`\nWhatsApp Logged In: `%v`",
		connected, loggedIn,
	)
	_, err := ctx.EffectiveMessage.Reply(bot, text, &gotgbot.SendMessageOpts{ParseMode: "Markdown"})
	return err
}

/* ═══════════════════════════════════════════════
 *  /list — Show Active Links
 * ═══════════════════════════════════════════════ */

/**
 * list displays all active WA ↔ TG chat mappings
 * stored in the database.
 */
func (h *CommandHandler) list(bot *gotgbot.Bot, ctx *ext.Context) error {
	mappings, err := h.repo.GetAll()
	if err != nil {
		_, _ = ctx.EffectiveMessage.Reply(bot, "❌ Failed to load mappings.", nil)
		return err
	}
	if len(mappings) == 0 {
		_, err = ctx.EffectiveMessage.Reply(bot, "No active links.", nil)
		return err
	}

	var sb strings.Builder
	sb.WriteString("🔗 *Active Links:*\n\n")
	for i, m := range mappings {
		sb.WriteString(fmt.Sprintf("%d. WA: `%s` ↔ TG: `%d`\n", i+1, m.WAChatID, m.TGChatID))
	}
	_, err = ctx.EffectiveMessage.Reply(bot, sb.String(), &gotgbot.SendMessageOpts{ParseMode: "Markdown"})
	return err
}

/* ═══════════════════════════════════════════════
 *  /bridge — Link Chats
 * ═══════════════════════════════════════════════ */

/**
 * bridgeCmd shows an inline keyboard of known WA chats
 * so the user can select one to link with this TG chat.
 *
 * Known chats are populated dynamically as messages arrive.
 */
func (h *CommandHandler) bridgeCmd(bot *gotgbot.Bot, ctx *ext.Context) error {
	if !h.wa.IsLoggedIn() {
		_, err := ctx.EffectiveMessage.Reply(bot, "⚠️ Not logged in. Send /scan first.", nil)
		return err
	}

	chats := h.wa.GetKnownChats()
	if len(chats) == 0 {
		_, err := ctx.EffectiveMessage.Reply(bot, "No known chats yet. Send a WhatsApp message to someone first, then try /bridge again.", nil)
		return err
	}

	var keyboard [][]gotgbot.InlineKeyboardButton
	count := 0
	for jid, name := range chats {
		if count >= 10 {
			break
		}
		label := name
		if label == "" {
			label = jid
		}
		keyboard = append(keyboard, []gotgbot.InlineKeyboardButton{{
			Text:         label,
			CallbackData: "bridge_wa_" + jid,
		}})
		count++
	}

	_, err := ctx.EffectiveMessage.Reply(bot, "Select a WhatsApp chat to link:", &gotgbot.SendMessageOpts{
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard},
	})
	return err
}

/**
 * bridgeCallback processes the inline keyboard selection
 * and persists the WA ↔ TG mapping.
 */
func (h *CommandHandler) bridgeCallback(bot *gotgbot.Bot, ctx *ext.Context) error {
	cb := ctx.Update.CallbackQuery
	_, _ = cb.Answer(bot, nil)

	waJID := strings.TrimPrefix(cb.Data, "bridge_wa_")
	tgChatID := cb.Message.GetChat().Id

	if err := h.repo.Add(waJID, tgChatID); err != nil {
		_, _, _ = bot.EditMessageText("❌ Failed to link.", &gotgbot.EditMessageTextOpts{
			ChatId: cb.Message.GetChat().Id, MessageId: cb.Message.GetMessageId(),
		})
		return err
	}

	_, _, _ = bot.EditMessageText(
		fmt.Sprintf("✅ Linked!\n\nThis chat ↔ WA `%s`", waJID),
		&gotgbot.EditMessageTextOpts{
			ChatId: cb.Message.GetChat().Id, MessageId: cb.Message.GetMessageId(),
			ParseMode: "Markdown",
		},
	)
	return nil
}

/* ═══════════════════════════════════════════════
 *  /unbridge — Remove Link
 * ═══════════════════════════════════════════════ */

/**
 * unbridge deletes the chat mapping for this TG chat.
 */
func (h *CommandHandler) unbridge(bot *gotgbot.Bot, ctx *ext.Context) error {
	if err := h.repo.RemoveByTG(ctx.EffectiveMessage.Chat.Id); err != nil {
		_, _ = ctx.EffectiveMessage.Reply(bot, "❌ Failed to unlink.", nil)
		return err
	}
	_, err := ctx.EffectiveMessage.Reply(bot, "✅ Link removed for this chat.", nil)
	return err
}

/* ═══════════════════════════════════════════════
 *  Catch-All — Message & Media Forwarder
 * ═══════════════════════════════════════════════ */

/**
 * forwardToWA is the catch-all handler that intercepts
 * all non-command messages (text, photo, video, audio,
 * voice, document, sticker) and relays them to WhatsApp
 * through the WagramService.
 */
func (h *CommandHandler) forwardToWA(bot *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if msg == nil {
		return nil
	}

	if msg.Text != "" && strings.HasPrefix(msg.Text, "/") {
		return nil
	}

	senderName := msg.From.FirstName
	if msg.From.LastName != "" {
		senderName += " " + msg.From.LastName
	}
	username := ""
	if msg.From.Username != "" {
		username = "@" + msg.From.Username
	}

	if len(msg.Photo) > 0 {
		best := msg.Photo[len(msg.Photo)-1]
		h.wagram.HandleTGMedia(msg.Chat.Id, best.FileId, domain.MediaImage, "photo.jpg", msg.Caption)
		return nil
	}

	if msg.Video != nil {
		h.wagram.HandleTGMedia(msg.Chat.Id, msg.Video.FileId, domain.MediaVideo, msg.Video.FileName, msg.Caption)
		return nil
	}

	if msg.Audio != nil {
		h.wagram.HandleTGMedia(msg.Chat.Id, msg.Audio.FileId, domain.MediaAudio, msg.Audio.FileName, msg.Caption)
		return nil
	}

	if msg.Voice != nil {
		h.wagram.HandleTGMedia(msg.Chat.Id, msg.Voice.FileId, domain.MediaAudio, "voice.ogg", "")
		return nil
	}

	if msg.Document != nil {
		h.wagram.HandleTGMedia(msg.Chat.Id, msg.Document.FileId, domain.MediaDocument, msg.Document.FileName, msg.Caption)
		return nil
	}

	if msg.Sticker != nil {
		h.wagram.HandleTGMedia(msg.Chat.Id, msg.Sticker.FileId, domain.MediaSticker, "sticker.webp", "")
		return nil
	}

	if msg.Text == "" {
		return nil
	}

	h.wagram.HandleTGMessage(msg.Chat.Id, senderName, username, msg.Text)
	return nil
}
