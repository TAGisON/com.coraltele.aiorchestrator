package router

import (
	"context"
	"fmt"
	"sync"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
)

// MemRegistry is an in-process gateway registry.
type MemRegistry struct {
	mu   sync.RWMutex
	byID map[port.GatewayID]port.Registration
}

func NewMemRegistry() *MemRegistry {
	return &MemRegistry{byID: make(map[port.GatewayID]port.Registration)}
}

func (r *MemRegistry) Register(reg port.Registration) error {
	if reg.ID == "" {
		return fmt.Errorf("registration missing id")
	}
	if reg.Port == "" {
		return fmt.Errorf("registration %s missing port", reg.ID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[reg.ID] = reg
	return nil
}

func (r *MemRegistry) Get(id port.GatewayID) (port.Registration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.byID[id]
	return reg, ok
}

func (r *MemRegistry) List(p port.PortKind) []port.Registration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]port.Registration, 0)
	for _, reg := range r.byID {
		if reg.Port == p {
			out = append(out, reg)
		}
	}
	return out
}

// SelectOptions control capability filtering.
type SelectOptions struct {
	Clock string // "live" | "playback"
}

// Select walks provider IDs in order; skips missing/unhealthy/incapable.
func Select(reg port.Registry, providers []port.GatewayID, kind port.PortKind, opt SelectOptions) (port.Registration, error) {
	var lastErr error
	for _, id := range providers {
		rec, ok := reg.Get(id)
		if !ok {
			lastErr = &port.GatewayError{Code: port.CodeUnavailable, Message: "gateway not registered: " + string(id)}
			continue
		}
		if rec.Port != kind {
			lastErr = &port.GatewayError{Code: port.CodeBadRequest, Message: "gateway port mismatch: " + string(id)}
			continue
		}
		if opt.Clock == "live" && (kind == port.PortListen || kind == port.PortSpeak) && !rec.Capabilities.Streaming {
			lastErr = &port.GatewayError{Code: port.CodeUnsupported, Message: "live requires streaming: " + string(id), Retryable: false}
			continue
		}
		if rec.Probe != nil {
			h := rec.Probe(context.Background())
			if !h.Healthy {
				lastErr = &port.GatewayError{Code: port.CodeUnavailable, Message: "unhealthy: " + string(id), Retryable: true}
				continue
			}
		}
		return rec, nil
	}
	if lastErr == nil {
		lastErr = &port.GatewayError{Code: port.CodeUnavailable, Message: "no providers"}
	}
	return port.Registration{}, lastErr
}
