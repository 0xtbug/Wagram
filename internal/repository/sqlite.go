/*
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *  Repository Layer — SQLite Persistence
 * ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
 *
 *  Implements domain.MappingRepository backed by
 *  a local SQLite database (wagram.db).
 *
 *  Schema:
 *    chat_mappings (
 *      wa_chat_id TEXT NOT NULL,
 *      tg_chat_id INTEGER NOT NULL PRIMARY KEY
 *    )
 */
package repository

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/glebarez/go-sqlite"

	"telegram-wa/internal/domain"
)

/**
 * SQLiteRepository implements domain.MappingRepository
 * using a lightweight, pure-Go SQLite driver.
 */
type SQLiteRepository struct {
	db *sql.DB
}

/**
 * NewSQLiteRepository opens (or creates) the SQLite database
 * at the given path and ensures the schema exists.
 */
func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS chat_mappings (
		wa_chat_id TEXT NOT NULL,
		tg_chat_id INTEGER NOT NULL,
		PRIMARY KEY (tg_chat_id)
	);`
	if _, err = database.Exec(schema); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &SQLiteRepository{db: database}, nil
}

/**
 * Add upserts a WA ↔ TG mapping.
 * If the TG chat already has a mapping, it gets overwritten.
 */
func (r *SQLiteRepository) Add(waChatID string, tgChatID int64) error {
	query := `INSERT INTO chat_mappings (wa_chat_id, tg_chat_id)
	           VALUES (?, ?)
	           ON CONFLICT(tg_chat_id) DO UPDATE SET wa_chat_id = excluded.wa_chat_id`
	_, err := r.db.Exec(query, waChatID, tgChatID)
	return err
}

/**
 * GetByTG looks up the WhatsApp JID linked to a Telegram chat.
 * Returns "" if no mapping exists.
 */
func (r *SQLiteRepository) GetByTG(tgChatID int64) (string, error) {
	var waChatID string
	err := r.db.QueryRow(
		`SELECT wa_chat_id FROM chat_mappings WHERE tg_chat_id = ?`, tgChatID,
	).Scan(&waChatID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return waChatID, err
}

/**
 * GetByWA returns all Telegram chat IDs linked to
 * the given WhatsApp JID (supports one-to-many).
 */
func (r *SQLiteRepository) GetByWA(waChatID string) ([]int64, error) {
	rows, err := r.db.Query(
		`SELECT tg_chat_id FROM chat_mappings WHERE wa_chat_id = ?`, waChatID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

/**
 * RemoveByTG deletes the chat mapping for a Telegram chat.
 */
func (r *SQLiteRepository) RemoveByTG(tgChatID int64) error {
	_, err := r.db.Exec(`DELETE FROM chat_mappings WHERE tg_chat_id = ?`, tgChatID)
	return err
}

/**
 * GetAll returns every active chat mapping in the database.
 */
func (r *SQLiteRepository) GetAll() ([]domain.ChatMapping, error) {
	rows, err := r.db.Query(`SELECT wa_chat_id, tg_chat_id FROM chat_mappings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []domain.ChatMapping
	for rows.Next() {
		var m domain.ChatMapping
		if err := rows.Scan(&m.WAChatID, &m.TGChatID); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

/**
 * Close releases the database connection.
 */
func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}
