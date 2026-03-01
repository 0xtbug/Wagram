/*
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *  WhatsApp Service — whatsmeow Wrapper
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *
 *  Manages the WhatsApp Multi-Device connection:
 *    • Session storage in SQLite (wa_session.db)
 *    • QR login flow with event channel
 *    • Sending text and media messages
 *    • Receiving & downloading incoming media
 *    • Dynamic 1-to-1 chat discovery (known chats)
 *    • LID → phone number resolution
 */
package service

import (
	"context"
	"fmt"
	"log"
	"sync"

	_ "github.com/glebarez/go-sqlite"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"telegram-wa/internal/domain"
)

/**
 * WhatsAppService implements domain.WhatsAppClient.
 * It wraps the whatsmeow library and maintains a map
 * of dynamically discovered 1-to-1 chats.
 */
type WhatsAppService struct {
	client *whatsmeow.Client       /* whatsmeow multi-device client */
	onMsg  domain.WAMessageHandler /* Wagram callback for incoming  */
	mu     sync.Mutex              /* Guards the chats map          */
	chats  map[string]string       /* JID → display name (dynamic)  */
}

/* ═══════════════════════════════════════════════
 *  Initialization
 * ═══════════════════════════════════════════════ */

/**
 * NewWhatsAppService creates a new WhatsApp service connected
 * to a dedicated SQLite session database.
 */
func NewWhatsAppService(sessionDBPath string) (*WhatsAppService, error) {
	dbLog := waLog.Stdout("Database", "WARN", true)
	container, err := sqlstore.New(context.Background(), "sqlite", "file:"+sessionDBPath+"?_pragma=foreign_keys(1)", dbLog)
	if err != nil {
		return nil, fmt.Errorf("init whatsmeow session store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}

	clientLog := waLog.Stdout("Client", "WARN", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	svc := &WhatsAppService{
		client: client,
		chats:  make(map[string]string),
	}
	client.AddEventHandler(svc.handleEvent)

	return svc, nil
}

/**
 * SetMessageHandler registers the callback that will be
 * invoked for every incoming WhatsApp message.
 */
func (s *WhatsAppService) SetMessageHandler(h domain.WAMessageHandler) {
	s.onMsg = h
}

/* ═══════════════════════════════════════════════
 *  Connection Management
 * ═══════════════════════════════════════════════ */

/**
 * Connect establishes the WhatsApp WebSocket connection.
 * Fails if no session exists (must call GenerateQR first).
 */
func (s *WhatsAppService) Connect() error {
	if s.client.Store.ID == nil {
		return fmt.Errorf("not logged in, generate QR first")
	}
	if err := s.client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	log.Println("WhatsApp connected")
	return nil
}

/**
 * Disconnect closes the WhatsApp connection.
 */
func (s *WhatsAppService) Disconnect() {
	s.client.Disconnect()
}

/**
 * IsLoggedIn returns true if the device is authenticated.
 */
func (s *WhatsAppService) IsLoggedIn() bool {
	return s.client.IsLoggedIn()
}

/**
 * IsConnected returns true if the WebSocket is open.
 */
func (s *WhatsAppService) IsConnected() bool {
	return s.client.IsConnected()
}

/* ═══════════════════════════════════════════════
 *  QR Login Flow
 * ═══════════════════════════════════════════════ */

/**
 * GenerateQR initiates the WhatsApp login process
 * and returns a channel that emits QR code lifecycle events.
 *
 * The caller should read from the channel and react to:
 *   "code"    → display the QR code
 *   "success" → login complete
 *   "timeout" → QR expired, need to retry
 */
func (s *WhatsAppService) GenerateQR(ctx context.Context) (<-chan domain.QREvent, error) {
	if s.client.Store.ID != nil {
		return nil, fmt.Errorf("already logged in")
	}

	qrChan, _ := s.client.GetQRChannel(ctx)
	if err := s.client.Connect(); err != nil {
		return nil, fmt.Errorf("connect for QR: %w", err)
	}

	out := make(chan domain.QREvent, 1)
	go func() {
		defer close(out)
		for evt := range qrChan {
			out <- domain.QREvent{Code: evt.Code, Event: evt.Event}
		}
	}()

	return out, nil
}

/* ═══════════════════════════════════════════════
 *  Chat Discovery
 * ═══════════════════════════════════════════════ */

/**
 * GetKnownChats returns a snapshot of all dynamically
 * discovered 1-to-1 chats (populated as messages arrive).
 *
 * Returns: map[JID_string]display_name
 */
func (s *WhatsAppService) GetKnownChats() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string]string, len(s.chats))
	for k, v := range s.chats {
		result[k] = v
	}
	return result
}

/* ═══════════════════════════════════════════════
 *  Sending Messages
 * ═══════════════════════════════════════════════ */

/**
 * SendText sends a plain text message to the given JID.
 */
func (s *WhatsAppService) SendText(ctx context.Context, jid string, text string) error {
	target, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("parse JID %q: %w", jid, err)
	}
	_, err = s.client.SendMessage(ctx, target, &waProto.Message{
		Conversation: proto.String(text),
	})
	return err
}

/**
 * SendMedia uploads a media file to WhatsApp servers
 * and sends it to the given JID.
 *
 * Supports: Image, Video, Audio, Document.
 * The upload response fields (URL, keys, hashes) are
 * embedded into the protobuf message automatically.
 */
func (s *WhatsAppService) SendMedia(ctx context.Context, jid string, data []byte, mediaType domain.MediaType, filename string, caption string) error {
	target, err := types.ParseJID(jid)
	if err != nil {
		return fmt.Errorf("parse JID %q: %w", jid, err)
	}

	/* Map domain media type → whatsmeow upload type */
	var waMediaType whatsmeow.MediaType
	switch mediaType {
	case domain.MediaImage:
		waMediaType = whatsmeow.MediaImage
	case domain.MediaVideo:
		waMediaType = whatsmeow.MediaVideo
	case domain.MediaAudio:
		waMediaType = whatsmeow.MediaAudio
	case domain.MediaDocument:
		waMediaType = whatsmeow.MediaDocument
	default:
		waMediaType = whatsmeow.MediaDocument
	}

	/* Upload the encrypted media blob */
	resp, err := s.client.Upload(ctx, data, waMediaType)
	if err != nil {
		return fmt.Errorf("upload media: %w", err)
	}

	/* Build the protobuf message per media type */
	var msg *waProto.Message
	switch mediaType {
	case domain.MediaImage:
		msg = &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				Caption:       proto.String(caption),
				URL:           proto.String(resp.URL),
				DirectPath:    proto.String(resp.DirectPath),
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    proto.Uint64(resp.FileLength),
				Mimetype:      proto.String("image/jpeg"),
			},
		}
	case domain.MediaVideo:
		msg = &waProto.Message{
			VideoMessage: &waProto.VideoMessage{
				Caption:       proto.String(caption),
				URL:           proto.String(resp.URL),
				DirectPath:    proto.String(resp.DirectPath),
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    proto.Uint64(resp.FileLength),
				Mimetype:      proto.String("video/mp4"),
			},
		}
	case domain.MediaAudio:
		msg = &waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				URL:           proto.String(resp.URL),
				DirectPath:    proto.String(resp.DirectPath),
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    proto.Uint64(resp.FileLength),
				Mimetype:      proto.String("audio/ogg; codecs=opus"),
			},
		}
	default:
		msg = &waProto.Message{
			DocumentMessage: &waProto.DocumentMessage{
				Caption:       proto.String(caption),
				Title:         proto.String(filename),
				FileName:      proto.String(filename),
				URL:           proto.String(resp.URL),
				DirectPath:    proto.String(resp.DirectPath),
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    proto.Uint64(resp.FileLength),
				Mimetype:      proto.String("application/octet-stream"),
			},
		}
	}

	_, err = s.client.SendMessage(ctx, target, msg)
	return err
}

/* ═══════════════════════════════════════════════
 *  Event Handling (Private)
 * ═══════════════════════════════════════════════ */

/**
 * trackChat records a 1-to-1 chat JID and display name
 * for later use in the /bridge command (chat discovery).
 *
 * Accepts both @s.whatsapp.net and @lid (Linked Identity)
 * JIDs. Rejects groups, broadcasts, and status JIDs.
 */
func (s *WhatsAppService) trackChat(chatJID types.JID, name string) {
	server := chatJID.Server
	if server != types.DefaultUserServer && server != "lid" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := chatJID.String()
	if name == "" {
		name = chatJID.User
	}
	s.chats[key] = name
}

/**
 * handleEvent is the whatsmeow event dispatcher.
 *
 * For *events.Message it:
 *   1. Tracks the chat for /bridge discovery
 *   2. Detects media type (image/video/audio/doc/sticker)
 *   3. Downloads media bytes if present
 *   4. Builds a domain.IncomingWAMessage
 *   5. Invokes the wagram callback
 */
func (s *WhatsAppService) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if !v.Info.IsGroup {
			s.trackChat(v.Info.Chat, v.Info.PushName)
		}

		if s.onMsg == nil {
			return
		}

		senderPhone := s.resolvePhone(v.Info.Sender, v.Info.PushName)
		msg := domain.IncomingWAMessage{
			ChatJID:     v.Info.Chat.String(),
			SenderName:  v.Info.PushName,
			SenderPhone: senderPhone,
			IsFromMe:    v.Info.IsFromMe,
			IsGroup:     v.Info.IsGroup,
		}

		/* ── Image ──────────────────── */
		if img := v.Message.GetImageMessage(); img != nil {
			msg.MediaType = domain.MediaImage
			msg.MimeType = img.GetMimetype()
			msg.Caption = img.GetCaption()
			msg.Text = msg.Caption
			data, err := s.client.Download(context.Background(), img)
			if err != nil {
				log.Printf("wa: download image: %v", err)
			} else {
				msg.MediaData = data
			}

			/* ── Video ──────────────────── */
		} else if vid := v.Message.GetVideoMessage(); vid != nil {
			msg.MediaType = domain.MediaVideo
			msg.MimeType = vid.GetMimetype()
			msg.Caption = vid.GetCaption()
			msg.Text = msg.Caption
			data, err := s.client.Download(context.Background(), vid)
			if err != nil {
				log.Printf("wa: download video: %v", err)
			} else {
				msg.MediaData = data
			}

			/* ── Audio / Voice Note ─────── */
		} else if aud := v.Message.GetAudioMessage(); aud != nil {
			msg.MediaType = domain.MediaAudio
			msg.MimeType = aud.GetMimetype()
			data, err := s.client.Download(context.Background(), aud)
			if err != nil {
				log.Printf("wa: download audio: %v", err)
			} else {
				msg.MediaData = data
			}

			/* ── Document / File ────────── */
		} else if doc := v.Message.GetDocumentMessage(); doc != nil {
			msg.MediaType = domain.MediaDocument
			msg.MimeType = doc.GetMimetype()
			msg.FileName = doc.GetFileName()
			msg.Caption = doc.GetCaption()
			msg.Text = msg.Caption
			data, err := s.client.Download(context.Background(), doc)
			if err != nil {
				log.Printf("wa: download document: %v", err)
			} else {
				msg.MediaData = data
			}

			/* ── Sticker ─────────────────── */
		} else if stk := v.Message.GetStickerMessage(); stk != nil {
			msg.MediaType = domain.MediaSticker
			msg.MimeType = stk.GetMimetype()
			data, err := s.client.Download(context.Background(), stk)
			if err != nil {
				log.Printf("wa: download sticker: %v", err)
			} else {
				msg.MediaData = data
			}

			/* ── Plain text ──────────────── */
		} else {
			text := v.Message.GetConversation()
			if text == "" {
				text = v.Message.GetExtendedTextMessage().GetText()
			}
			if text == "" {
				return
			}
			msg.Text = text
		}

		s.onMsg(msg)
	}
}

/* ═══════════════════════════════════════════════
 *  Phone Number Resolution
 * ═══════════════════════════════════════════════ */

/**
 * resolvePhone attempts to get a real phone number from a JID.
 *
 *  @s.whatsapp.net → User field IS the phone number.
 *  @lid            → Search the contact store by push name
 *                    to find the matching phone-based JID.
 *  Fallback        → Returns push name or raw user ID.
 */
func (s *WhatsAppService) resolvePhone(jid types.JID, pushName string) string {
	if jid.Server == types.DefaultUserServer {
		return "+" + jid.User
	}

	if jid.Server == "lid" && pushName != "" {
		contacts, err := s.client.Store.Contacts.GetAllContacts(context.Background())
		if err == nil {
			for contactJID, info := range contacts {
				if contactJID.Server == types.DefaultUserServer {
					if info.PushName == pushName || info.FullName == pushName {
						return "+" + contactJID.User
					}
				}
			}
		}
	}

	if pushName != "" {
		return pushName
	}
	return jid.User
}
