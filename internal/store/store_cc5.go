package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// AppendTranscriptTurn inserts one turn with next seq for the session (transactional).
func (s *Store) AppendTranscriptTurn(ctx context.Context, turn TranscriptTurn) (TranscriptTurn, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TranscriptTurn{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var nextSeq int
	err = tx.QueryRow(ctx, `
SELECT COALESCE(MAX(seq), 0) + 1 FROM transcript_turn WHERE session_id=$1
`, turn.SessionID).Scan(&nextSeq)
	if err != nil {
		return TranscriptTurn{}, err
	}

	var out TranscriptTurn
	err = tx.QueryRow(ctx, `
INSERT INTO transcript_turn (session_id, seq, role, text, turn_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, session_id, seq, role, text, turn_id, created_at
`, turn.SessionID, nextSeq, turn.Role, turn.Text, turn.TurnID).Scan(
		&out.ID, &out.SessionID, &out.Seq, &out.Role, &out.Text, &out.TurnID, &out.CreatedAt,
	)
	if err != nil {
		return TranscriptTurn{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TranscriptTurn{}, err
	}
	return out, nil
}

func (s *Store) ListTranscriptTurns(ctx context.Context, sessionID string) ([]TranscriptTurn, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, session_id, seq, role, text, turn_id, created_at
FROM transcript_turn WHERE session_id=$1 ORDER BY seq ASC
`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TranscriptTurn
	for rows.Next() {
		var t TranscriptTurn
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Seq, &t.Role, &t.Text, &t.TurnID, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) UpsertSessionDisposition(ctx context.Context, d SessionDisposition) (SessionDisposition, error) {
	if d.Source == "" {
		d.Source = "postcall_worker"
	}
	var final any
	if d.Final != "" {
		final = d.Final
	}
	var out SessionDisposition
	var finalOut *string
	err := s.pool.QueryRow(ctx, `
INSERT INTO session_disposition (session_id, suggestion, template_id, source, final, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (session_id) DO UPDATE SET
  suggestion = EXCLUDED.suggestion,
  template_id = EXCLUDED.template_id,
  source = EXCLUDED.source,
  final = COALESCE(EXCLUDED.final, session_disposition.final),
  updated_at = now()
RETURNING session_id, suggestion, template_id, source, final, updated_at
`, d.SessionID, d.Suggestion, d.TemplateID, d.Source, final).Scan(
		&out.SessionID, &out.Suggestion, &out.TemplateID, &out.Source, &finalOut, &out.UpdatedAt,
	)
	if err != nil {
		return SessionDisposition{}, err
	}
	if finalOut != nil {
		out.Final = *finalOut
	}
	return out, nil
}

func (s *Store) GetSessionDisposition(ctx context.Context, sessionID string) (SessionDisposition, error) {
	var out SessionDisposition
	var finalOut *string
	err := s.pool.QueryRow(ctx, `
SELECT session_id, suggestion, template_id, source, final, updated_at
FROM session_disposition WHERE session_id=$1
`, sessionID).Scan(&out.SessionID, &out.Suggestion, &out.TemplateID, &out.Source, &finalOut, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionDisposition{}, ErrNotFound
	}
	if err != nil {
		return SessionDisposition{}, err
	}
	if finalOut != nil {
		out.Final = *finalOut
	}
	return out, nil
}
