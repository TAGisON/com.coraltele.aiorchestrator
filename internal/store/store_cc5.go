package store

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// AppendTranscriptTurn inserts one turn with next seq for the session (transactional).
func (s *Store) AppendTranscriptTurn(ctx context.Context, turn TranscriptTurn) (TranscriptTurn, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TranscriptTurn{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, transcriptSessionLockKey(turn.SessionID)); err != nil {
		return TranscriptTurn{}, err
	}

	var nextSeq int
	err = tx.QueryRow(ctx, `
SELECT COALESCE(MAX(seq), 0) + 1 FROM transcript_turn WHERE session_id=$1
`, turn.SessionID).Scan(&nextSeq)
	if err != nil {
		return TranscriptTurn{}, err
	}

	payload := turn.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	var turnID any
	if strings.TrimSpace(turn.TurnID) != "" {
		turnID = turn.TurnID
	}

	var out TranscriptTurn
	var actionable *bool
	var outTurnID *string
	var outPayload []byte
	err = tx.QueryRow(ctx, `
INSERT INTO transcript_turn (
  session_id, seq, role, text, turn_id,
  event_kind, actionable, actionable_reason, node_id, edge_id, language, payload
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
RETURNING id, session_id, seq, role, text, turn_id, event_kind, actionable, actionable_reason,
          node_id, edge_id, language, payload, created_at
`, turn.SessionID, nextSeq, turn.Role, turn.Text, turnID,
		turn.EventKind, turn.Actionable, turn.ActionableReason, turn.NodeID, turn.EdgeID, turn.Language, payload,
	).Scan(
		&out.ID, &out.SessionID, &out.Seq, &out.Role, &out.Text, &outTurnID,
		&out.EventKind, &actionable, &out.ActionableReason,
		&out.NodeID, &out.EdgeID, &out.Language, &outPayload, &out.CreatedAt,
	)
	if err != nil {
		return TranscriptTurn{}, err
	}
	if outTurnID != nil {
		out.TurnID = *outTurnID
	}
	out.Actionable = actionable
	if len(outPayload) > 0 {
		out.Payload = json.RawMessage(outPayload)
	}
	if err := tx.Commit(ctx); err != nil {
		return TranscriptTurn{}, err
	}
	return out, nil
}

func (s *Store) ListTranscriptTurns(ctx context.Context, sessionID string) ([]TranscriptTurn, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, session_id, seq, role, text, turn_id, event_kind, actionable, actionable_reason,
       node_id, edge_id, language, payload, created_at
FROM transcript_turn WHERE session_id=$1 ORDER BY seq ASC
`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TranscriptTurn
	for rows.Next() {
		var t TranscriptTurn
		var turnID *string
		var payload []byte
		if err := rows.Scan(
			&t.ID, &t.SessionID, &t.Seq, &t.Role, &t.Text, &turnID,
			&t.EventKind, &t.Actionable, &t.ActionableReason,
			&t.NodeID, &t.EdgeID, &t.Language, &payload, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		if turnID != nil {
			t.TurnID = *turnID
		}
		if len(payload) > 0 {
			t.Payload = json.RawMessage(payload)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func transcriptSessionLockKey(sessionID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("transcript:"))
	_, _ = h.Write([]byte(strings.TrimSpace(sessionID)))
	return int64(h.Sum64())
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
