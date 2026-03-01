/*
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *  Telegram Service — Bot API & Update Pipeline
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *
 *  Wraps the gotgbot library to provide:
 *    • Bot creation and long-polling
 *    • Text message sending (Markdown)
 *    • Media sending (photo, video, audio, doc, sticker)
 *    • File downloading from Telegram servers
 */
package service

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"telegram-wa/internal/domain"
)

/**
 * TelegramService manages the Telegram Bot API client,
 * the update dispatcher, and the long-poll updater.
 */
type TelegramService struct {
	Bot        *gotgbot.Bot    /* Authenticated bot client       */
	Updater    *ext.Updater    /* Receives updates via polling   */
	Dispatcher *ext.Dispatcher /* Routes updates to handlers    */
}

/**
 * NewTelegramService creates a bot instance, dispatcher,
 * and updater from the given API token.
 */
func NewTelegramService(token string) (*TelegramService, error) {
	bot, err := gotgbot.NewBot(token, nil)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(_ *gotgbot.Bot, _ *ext.Context, err error) ext.DispatcherAction {
			log.Printf("telegram handler error: %v", err)
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})

	updater := ext.NewUpdater(dispatcher, nil)

	return &TelegramService{
		Bot:        bot,
		Updater:    updater,
		Dispatcher: dispatcher,
	}, nil
}

/**
 * Start begins long-polling for Telegram updates.
 * Uses a 30-second HTTP timeout to avoid deadline issues.
 */
func (s *TelegramService) Start() error {
	log.Printf("Telegram bot started: @%s", s.Bot.User.Username)
	return s.Updater.StartPolling(s.Bot, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 9,
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: 30 * time.Second,
			},
		},
	})
}

/**
 * Stop gracefully shuts down the updater.
 */
func (s *TelegramService) Stop() {
	s.Updater.Stop()
}

/**
 * SendMessage sends a Markdown-formatted text message
 * to the specified Telegram chat.
 */
func (s *TelegramService) SendMessage(chatID int64, text string) error {
	_, err := s.Bot.SendMessage(chatID, text, &gotgbot.SendMessageOpts{
		ParseMode: "Markdown",
	})
	return err
}

/**
 * SendMedia sends a media file (photo, video, audio, doc, sticker)
 * to the specified Telegram chat using in-memory byte data.
 */
func (s *TelegramService) SendMedia(chatID int64, data []byte, mediaType domain.MediaType, filename, caption string) error {
	reader := bytes.NewReader(data)

	switch mediaType {
	case domain.MediaImage:
		_, err := s.Bot.SendPhoto(chatID, gotgbot.InputFileByReader("photo.jpg", reader), &gotgbot.SendPhotoOpts{
			Caption: caption,
		})
		return err
	case domain.MediaVideo:
		_, err := s.Bot.SendVideo(chatID, gotgbot.InputFileByReader("video.mp4", reader), &gotgbot.SendVideoOpts{
			Caption: caption,
		})
		return err
	case domain.MediaAudio:
		_, err := s.Bot.SendAudio(chatID, gotgbot.InputFileByReader("audio.ogg", reader), &gotgbot.SendAudioOpts{
			Caption: caption,
		})
		return err
	case domain.MediaSticker:
		_, err := s.Bot.SendSticker(chatID, gotgbot.InputFileByReader("sticker.webp", reader), nil)
		return err
	default:
		name := filename
		if name == "" {
			name = "file"
		}
		_, err := s.Bot.SendDocument(chatID, gotgbot.InputFileByReader(name, reader), &gotgbot.SendDocumentOpts{
			Caption: caption,
		})
		return err
	}
}

/**
 * DownloadFile fetches a file from Telegram servers by its file ID.
 * Returns the raw bytes of the downloaded file.
 */
func (s *TelegramService) DownloadFile(fileID string) ([]byte, error) {
	file, err := s.Bot.GetFile(fileID, nil)
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", s.Bot.Token, file.FilePath)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return data, nil
}
