package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vutran1710/dating-dev/pkg/models"
)

func (s *Store) CreateMessage(ctx context.Context, convID, senderID uuid.UUID, body string) (*models.Message, error) {
	var msg models.Message
	err := s.pool.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, sender_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, conversation_id, sender_id, body, created_at
	`, convID, senderID, body).Scan(
		&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.Body, &msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating message: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE conversations SET last_message_at = $2 WHERE id = $1
	`, convID, msg.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating conversation timestamp: %w", err)
	}

	return &msg, nil
}

func (s *Store) GetMessages(ctx context.Context, convID uuid.UUID, before *time.Time, limit int) ([]models.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var query string
	var args []any

	if before != nil {
		query = `
			SELECT id, conversation_id, sender_id, body, created_at
			FROM messages WHERE conversation_id = $1 AND created_at < $2
			ORDER BY created_at DESC LIMIT $3
		`
		args = []any{convID, *before, limit}
	} else {
		query = `
			SELECT id, conversation_id, sender_id, body, created_at
			FROM messages WHERE conversation_id = $1
			ORDER BY created_at DESC LIMIT $2
		`
		args = []any{convID, limit}
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting messages: %w", err)
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Body, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, nil
}
