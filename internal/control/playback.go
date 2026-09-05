package control

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/file"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/profile"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/runtime/session"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/store"
)

type playbackReq struct {
	ProfileID      string          `json:"profile_id"`
	ProfileVersion json.RawMessage `json:"profile_version"`
	FileURI        string          `json:"file_uri"`
	TenantID       string          `json:"tenant_id"`
}

func (s *Server) handlePlaybackCreate(w http.ResponseWriter, r *http.Request) {
	var req playbackReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	if strings.TrimSpace(req.ProfileID) == "" || strings.TrimSpace(req.FileURI) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "profile_id and file_uri required", nil)
		return
	}
	pv, err := s.resolveProfileVersion(r.Context(), req.ProfileID, req.ProfileVersion)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "profile version not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	id, err := newID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "id generate failed", nil)
		return
	}
	job := store.PlaybackJob{
		ID:             id,
		TenantID:       req.TenantID,
		FileURI:        req.FileURI,
		ProfileID:      pv.ProfileID,
		ProfileVersion: pv.Version,
		State:          store.JobQueued,
	}
	if err := s.repo.CreatePlaybackJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "enqueue failed", nil)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"job_id": job.ID,
		"state":  job.State,
	})
}

func (s *Server) handleJobGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := s.repo.GetPlaybackJob(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, CodeNotFound, "job not found", nil)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "get failed", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"job_id":          job.ID,
		"state":           job.State,
		"file_uri":        job.FileURI,
		"profile_id":      job.ProfileID,
		"profile_version": job.ProfileVersion,
		"session_id":      job.SessionID,
		"error":           job.ErrorMessage,
	})
}

// PlaybackWorker leases queued jobs and feeds file audio into a playback-clock session.
type PlaybackWorker struct {
	Repo   store.Repository
	Mgr    *session.Manager
	Reg    port.Registry
	Owner  string
	Cancel context.CancelFunc
}

// StartPlaybackWorker runs a background poller. Safe no-op if mgr is nil.
func (s *Server) StartPlaybackWorker(parent context.Context, mgr *session.Manager) *PlaybackWorker {
	if mgr == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	w := &PlaybackWorker{
		Repo:   s.repo,
		Mgr:    mgr,
		Reg:    s.reg,
		Owner:  s.cfg.OwnerInstance,
		Cancel: cancel,
	}
	go w.loop(ctx)
	return w
}

func (w *PlaybackWorker) loop(ctx context.Context) {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			job, err := w.Repo.LeaseNextPlaybackJob(ctx, w.Owner)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				continue
			}
			w.runJob(ctx, job)
		}
	}
}

func (w *PlaybackWorker) runJob(ctx context.Context, job store.PlaybackJob) {
	pv, err := w.Repo.GetVersion(ctx, job.ProfileID, job.ProfileVersion)
	if err != nil {
		job.State = store.JobFailed
		job.ErrorMessage = "profile missing"
		_ = w.Repo.UpdatePlaybackJob(ctx, job)
		return
	}
	doc, err := profile.Parse(pv.Document)
	if err != nil {
		job.State = store.JobFailed
		job.ErrorMessage = "profile corrupt"
		_ = w.Repo.UpdatePlaybackJob(ctx, job)
		return
	}
	sid := "pb-" + job.ID
	if len(sid) > 40 {
		sid = sid[:40]
	}
	actor, err := w.Mgr.Start(ctx, session.StartParams{
		SessionID:  sid,
		TenantID:   job.TenantID,
		Clock:      "playback",
		SampleRate: profile.SampleRateHz(doc),
		Profile:    doc,
		ProfileRaw: pv.Document,
		Reg:        w.Reg,
	})
	if err != nil {
		job.State = store.JobFailed
		job.ErrorMessage = err.Error()
		_ = w.Repo.UpdatePlaybackJob(ctx, job)
		return
	}
	job.SessionID = sid
	_ = w.Repo.UpdatePlaybackJob(ctx, job)
	ensurePlaybackSession(ctx, w.Repo, job, sid, profile.SampleRateHz(doc))

	tap := actor.Bus.SubscribeAudio(64)
	evs := actor.Bus.SubscribeEvents(16)
	feeder, err := file.Open(ctx, job.FileURI, 0, actor.SampleRate, actor.FrameMs, actor.Clock)
	if err != nil {
		job.State = store.JobFailed
		job.ErrorMessage = err.Error()
		_ = w.Repo.UpdatePlaybackJob(ctx, job)
		_, _ = w.Mgr.Stop(ctx, sid, "failed")
		return
	}
	actor.AttachFeeder(ctx, feeder, "file-feeder")

	deadline := time.After(60 * time.Second)
	frames := 0
done:
	for {
		select {
		case <-ctx.Done():
			break done
		case <-deadline:
			break done
		case fr, ok := <-tap:
			if !ok {
				break done
			}
			frames++
			_ = fr
		case ev, ok := <-evs:
			if !ok {
				continue
			}
			if ev.Kind == "feeder_stop" || ev.Kind == "feeder_gone" || ev.Kind == "playback_exhausted" {
				// drain remaining audio briefly
				drainUntil := time.After(200 * time.Millisecond)
			drainLoop:
				for {
					select {
					case fr, ok := <-tap:
						if !ok {
							break drainLoop
						}
						frames++
						_ = fr
					case <-drainUntil:
						break drainLoop
					}
				}
				break done
			}
		}
	}
	_, _ = w.Mgr.Stop(ctx, sid, "completed")
	job.State = store.JobCompleted
	if frames == 0 {
		job.State = store.JobFailed
		job.ErrorMessage = "no frames"
	}
	_ = w.Repo.UpdatePlaybackJob(ctx, job)

	if job.State == store.JobCompleted {
		sess, err := w.Repo.UpdateSessionState(ctx, sid, store.StateCompleted)
		if err != nil {
			sess, _ = w.Repo.GetSession(ctx, sid)
			sess.State = store.StateCompleted
		}
		if sess.ID != "" {
			enqueuePostcallRepo(ctx, w.Repo, sess)
		}
	} else {
		_, _ = w.Repo.UpdateSessionState(ctx, sid, store.StateFailed)
	}
	log.Printf("playback job %s state=%s frames=%d", job.ID, job.State, frames)
}

func enqueuePostcallRepo(ctx context.Context, repo store.Repository, sess store.Session) {
	id, err := newID()
	if err != nil {
		return
	}
	job := store.PostcallJob{
		ID:             id,
		SessionID:      sess.ID,
		ProfileID:      sess.ProfileID,
		ProfileVersion: sess.ProfileVersion,
		State:          store.JobQueued,
	}
	if err := repo.CreatePostcallJob(ctx, job); err != nil && !errors.Is(err, store.ErrConflict) {
		log.Printf("postcall enqueue from playback fail-open session=%s err=%v", sess.ID, err)
	}
}
