/*
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *  Domain Layer — Core Types & Interfaces
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *
 *  Pure domain definitions with zero external deps.
 *  Other layers depend on these; this layer depends
 *  on nothing except stdlib.
 *
 *  Contents:
 *    • MediaType          – enum for media categories
 *    • ChatMapping        – WA ↔ TG chat link record
 *    • MappingRepository  – persistence interface
 *    • WhatsAppClient     – WA connection interface
 *    • QREvent            – QR login lifecycle event
 *    • IncomingWAMessage  – normalized incoming message
 *    • WAMessageHandler   – callback type for WA msgs
 */
package domain

import "context"

/* ── Media Type Enum ─────────────────────────── */

/**
 * MediaType classifies the kind of media attached
 * to a message (image, video, audio, doc, sticker).
 */
type MediaType int

const (
	MediaNone     MediaType = iota /* No media — plain text message */
	MediaImage                     /* JPEG / PNG image              */
	MediaVideo                     /* MP4 / video file              */
	MediaAudio                     /* OGG / voice note / audio      */
	MediaDocument                  /* Any file / document           */
	MediaSticker                   /* WebP sticker                  */
)

/* ── Chat Mapping ────────────────────────────── */

/**
 * ChatMapping represents a single bridge link
 * between a WhatsApp chat and a Telegram chat.
 */
type ChatMapping struct {
	WAChatID string /* WhatsApp JID (e.g. "628xxx@s.whatsapp.net") */
	TGChatID int64  /* Telegram chat/group ID                      */
}

/* ── Repository Interface ────────────────────── */

/**
 * MappingRepository defines persistence operations
 * for chat bridge mappings (implemented by SQLite).
 */
type MappingRepository interface {
	Add(waChatID string, tgChatID int64) error /* Upsert a mapping           */
	GetByTG(tgChatID int64) (string, error)    /* Lookup WA chat by TG ID    */
	GetByWA(waChatID string) ([]int64, error)  /* Lookup TG chats by WA JID  */
	RemoveByTG(tgChatID int64) error           /* Delete mapping by TG ID    */
	GetAll() ([]ChatMapping, error)            /* List every active mapping  */
	Close() error                              /* Release DB resources       */
}

/* ── WhatsApp Client Interface ───────────────── */

/**
 * WhatsAppClient abstracts the WhatsApp connection,
 * allowing the bridge to work without knowing about
 * whatsmeow internals (dependency inversion).
 */
type WhatsAppClient interface {
	Connect() error
	Disconnect()
	IsLoggedIn() bool
	IsConnected() bool
	GenerateQR(ctx context.Context) (<-chan QREvent, error)
	GetKnownChats() map[string]string
	SendText(ctx context.Context, jid string, text string) error
	SendMedia(ctx context.Context, jid string, data []byte, mediaType MediaType, filename string, caption string) error
}

/* ── QR Login Event ──────────────────────────── */

/**
 * QREvent represents a single QR code lifecycle event
 * emitted during WhatsApp login.
 *
 *   Event values: "code" | "success" | "timeout" | "error"
 */
type QREvent struct {
	Code  string /* QR code data string (when Event == "code") */
	Event string /* Lifecycle stage                             */
}

/* ── Incoming WhatsApp Message ───────────────── */

/**
 * IncomingWAMessage is a normalized, platform-agnostic
 * representation of a WhatsApp message passed from the
 * WA service layer to the bridge service.
 */
type IncomingWAMessage struct {
	ChatJID     string    /* Chat JID (e.g. "628xxx@lid")           */
	SenderName  string    /* Push name / display name               */
	SenderPhone string    /* Phone number or fallback identifier    */
	Text        string    /* Plain text body (or caption for media) */
	IsFromMe    bool      /* True if sent by the bridge user        */
	IsGroup     bool      /* True if from a group chat              */
	MediaData   []byte    /* Raw media bytes (nil for text-only)    */
	MediaType   MediaType /* Type of attached media                 */
	MimeType    string    /* MIME type (e.g. "image/jpeg")          */
	FileName    string    /* Original filename (documents)          */
	Caption     string    /* Media caption text                     */
}

/* ── Callback Type ───────────────────────────── */

/**
 * WAMessageHandler is a callback function invoked
 * by the WhatsApp service whenever a new message arrives.
 */
type WAMessageHandler func(msg IncomingWAMessage)
