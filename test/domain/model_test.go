package domain_test

import (
	"testing"

	"telegram-wa/internal/domain"
)

func TestMediaType_Values(t *testing.T) {
	tests := []struct {
		name     string
		mt       domain.MediaType
		expected int
	}{
		{"MediaNone", domain.MediaNone, 0},
		{"MediaImage", domain.MediaImage, 1},
		{"MediaVideo", domain.MediaVideo, 2},
		{"MediaAudio", domain.MediaAudio, 3},
		{"MediaDocument", domain.MediaDocument, 4},
		{"MediaSticker", domain.MediaSticker, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.mt) != tt.expected {
				t.Errorf("%s: expected %d, got %d", tt.name, tt.expected, int(tt.mt))
			}
		})
	}
}

func TestChatMapping_Fields(t *testing.T) {
	m := domain.ChatMapping{
		WAChatID: "628123456@s.whatsapp.net",
		TGChatID: -100123456,
	}

	if m.WAChatID != "628123456@s.whatsapp.net" {
		t.Errorf("unexpected WAChatID: %q", m.WAChatID)
	}
	if m.TGChatID != -100123456 {
		t.Errorf("unexpected TGChatID: %d", m.TGChatID)
	}
}

func TestIncomingWAMessage_Defaults(t *testing.T) {
	msg := domain.IncomingWAMessage{}

	if msg.ChatJID != "" {
		t.Error("ChatJID should be empty by default")
	}
	if msg.IsFromMe {
		t.Error("IsFromMe should be false by default")
	}
	if msg.IsGroup {
		t.Error("IsGroup should be false by default")
	}
	if msg.MediaType != domain.MediaNone {
		t.Errorf("MediaType should be MediaNone, got %d", msg.MediaType)
	}
	if msg.MediaData != nil {
		t.Error("MediaData should be nil by default")
	}
}

func TestIncomingWAMessage_WithMedia(t *testing.T) {
	msg := domain.IncomingWAMessage{
		ChatJID:    "628123@lid",
		SenderName: "John",
		Text:       "Hello",
		MediaType:  domain.MediaImage,
		MediaData:  []byte{0xFF, 0xD8, 0xFF},
		MimeType:   "image/jpeg",
		IsGroup:    true,
	}

	if msg.MediaType != domain.MediaImage {
		t.Errorf("expected MediaImage, got %d", msg.MediaType)
	}
	if len(msg.MediaData) != 3 {
		t.Errorf("expected 3 bytes, got %d", len(msg.MediaData))
	}
	if msg.MimeType != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %q", msg.MimeType)
	}
	if !msg.IsGroup {
		t.Error("expected IsGroup to be true")
	}
}

func TestQREvent_Fields(t *testing.T) {
	events := []struct {
		code  string
		event string
	}{
		{"2@abc123", "code"},
		{"", "success"},
		{"", "timeout"},
	}

	for _, e := range events {
		evt := domain.QREvent{Code: e.code, Event: e.event}
		if evt.Code != e.code {
			t.Errorf("expected Code %q, got %q", e.code, evt.Code)
		}
		if evt.Event != e.event {
			t.Errorf("expected Event %q, got %q", e.event, evt.Event)
		}
	}
}
