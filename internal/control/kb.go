package control

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

type kbUploadReq struct {
	URI        string `json:"uri"`
	Collection string `json:"collection"`
	TenantID   string `json:"tenant_id"`
	Text       string `json:"text"` // optional inline body for lab
}

func (s *Server) handleKBUpload(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	var req kbUploadReq
	var inline []byte

	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "multipart parse failed", nil)
			return
		}
		req.Collection = r.FormValue("collection")
		req.TenantID = r.FormValue("tenant_id")
		req.URI = r.FormValue("uri")
		if f, _, err := r.FormFile("file"); err == nil {
			defer f.Close()
			inline, _ = io.ReadAll(io.LimitReader(f, 4<<20))
		}
	} else {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
			return
		}
	}
	if strings.TrimSpace(req.Collection) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "collection required", nil)
		return
	}
	id, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "id generate failed", nil)
		return
	}
	doc := store.KBDocument{
		ID:         id,
		TenantID:   req.TenantID,
		Collection: req.Collection,
		URI:        req.URI,
		Status:     store.KBIndexing,
	}
	if err := s.repo.CreateKBDocument(r.Context(), doc); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "create kb document failed", nil)
		return
	}

	text := req.Text
	if text == "" && len(inline) > 0 {
		text = string(inline)
	}
	if text == "" && req.URI != "" {
		path := strings.TrimPrefix(req.URI, "file://")
		if b, err := os.ReadFile(path); err == nil {
			text = string(b)
		}
	}
	if text == "" {
		_, _ = s.repo.UpdateKBDocumentStatus(r.Context(), id, store.KBFailed, "no content")
		writeJSON(w, http.StatusCreated, map[string]any{"id": id, "status": store.KBFailed})
		return
	}

	chunks := chunkText(id, req.TenantID, req.Collection, req.URI, text)
	if err := s.repo.ReplaceKBChunks(r.Context(), id, chunks); err != nil {
		_, _ = s.repo.UpdateKBDocumentStatus(r.Context(), id, store.KBFailed, err.Error())
		writeJSON(w, http.StatusCreated, map[string]any{"id": id, "status": store.KBFailed})
		return
	}
	doc, err = s.repo.UpdateKBDocumentStatus(r.Context(), id, store.KBReady, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "update status failed", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         doc.ID,
		"collection": doc.Collection,
		"status":     doc.Status,
		"uri":        doc.URI,
	})
}

func (s *Server) handleKBGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	doc, err := s.repo.GetKBDocument(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "document not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         doc.ID,
		"collection": doc.Collection,
		"status":     doc.Status,
		"uri":        doc.URI,
		"error":      doc.ErrorMessage,
	})
}

func chunkText(docID, tenant, collection, uri, text string) []store.KBChunk {
	parts := strings.Split(text, "\n\n")
	var out []store.KBChunk
	ord := 0
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// further split long paragraphs
		for len(p) > 0 {
			chunk := p
			if len(chunk) > 800 {
				chunk = p[:800]
				p = p[800:]
			} else {
				p = ""
			}
			out = append(out, store.KBChunk{
				DocumentID: docID,
				TenantID:   tenant,
				Collection: collection,
				Ordinal:    ord,
				Text:       chunk,
				SourceURI:  uri,
			})
			ord++
		}
	}
	if len(out) == 0 && strings.TrimSpace(text) != "" {
		out = append(out, store.KBChunk{
			DocumentID: docID,
			TenantID:   tenant,
			Collection: collection,
			Ordinal:    0,
			Text:       strings.TrimSpace(text),
			SourceURI:  uri,
		})
	}
	return out
}
