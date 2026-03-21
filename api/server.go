// Package api exposes the HTTP surface of the game server.
// It owns a single endpoint – POST /submit – and forwards every
// incoming response to the Game Engine for evaluation.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"game-engine/engine"
)

// Server wraps the HTTP mux and holds a reference to the Game Engine.
type Server struct {
	engine *engine.GameEngine
	mux    *http.ServeMux
}

// New wires up an API Server against the given Game Engine.
func New(ge *engine.GameEngine) *Server {
	s := &Server{
		engine: ge,
		mux:    http.NewServeMux(),
	}
	s.mux.HandleFunc("/submit", s.handleSubmit)
	s.mux.HandleFunc("/health", s.handleHealth)
	return s
}

// Start begins listening on addr (e.g. ":8080").
// It blocks until the server exits.
func (s *Server) Start(addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: s.mux,
		// Generous timeouts to accommodate the 1 000-ms simulated lag.
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Printf("[API] Listening on %s", addr)
	return srv.ListenAndServe()
}

// submitRequest is the JSON payload accepted by POST /submit.
type submitRequest struct {
	UserID        string `json:"user_id"`
	CorrectAnswer bool   `json:"correct_answer"`
}

// submitResponse is the JSON payload returned to the caller.
type submitResponse struct {
	Status   string `json:"status"`
	UserID   string `json:"user_id"`
	Received string `json:"received_at"`
}

// handleSubmit is the hot-path handler.
// It decodes the request, stamps a receive time, forwards to the engine,
// and replies immediately – all without holding any lock.
func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("bad request: %v", err), http.StatusBadRequest)
		return
	}

	resp := engine.Response{
		UserID:    req.UserID,
		Answer:    req.CorrectAnswer,
		Timestamp: time.Now(),
	}

	// Non-blocking forward to the game engine.
	s.engine.Submit(resp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 – received, evaluation is async.
	_ = json.NewEncoder(w).Encode(submitResponse{
		Status:   "accepted",
		UserID:   req.UserID,
		Received: resp.Timestamp.Format(time.RFC3339Nano),
	})
}

// handleHealth returns 200 OK so load-balancers and smoke tests can
// verify the server is alive without touching game state.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
