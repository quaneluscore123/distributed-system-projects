package server

import (
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/rosedblabs/rosedb/v2"
)

type Server struct {
	db         *rosedb.DB
	replicator *Replicator
	mux        *http.ServeMux
	startTime  time.Time
	syncKey    string
	mu         sync.RWMutex
	role       string // "MASTER" or "SLAVE"
}

func NewServer(db *rosedb.DB, replicator *Replicator, syncKey string) *Server {
	s := &Server{
		db:         db,
		replicator: replicator,
		mux:        http.NewServeMux(),
		startTime:  time.Now(),
		syncKey:    syncKey,
		role:       "MASTER",
	}

	// API endpoints
	s.mux.HandleFunc("/put", s.handlePut)
	s.mux.HandleFunc("/get", s.handleGet)
	s.mux.HandleFunc("/delete", s.handleDelete)
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/keys", s.handleKeys)
	s.mux.HandleFunc("/admin", s.handleAdmin)

	// Internal sync endpoint for Master to call on Slaves
	s.mux.HandleFunc("/sync", s.handleSync)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) GetRole() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.role
}

func (s *Server) SetRole(role string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.role = role
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) {
	if s.GetRole() != "MASTER" {
		http.Error(w, "Forbidden: Only MASTER node can accept write requests", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.FormValue("key")
	value := r.FormValue("value")
	if key == "" || value == "" {
		http.Error(w, "Missing key or value", http.StatusBadRequest)
		return
	}

	err := s.db.Put([]byte(key), []byte(value))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Replicate to slaves if this is a master node
	if s.replicator != nil {
		s.replicator.SyncPut([]byte(key), []byte(value))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing key", http.StatusBadRequest)
		return
	}

	val, err := s.db.Get([]byte(key))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(val)
	w.Write([]byte("\n"))
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if s.GetRole() != "MASTER" {
		http.Error(w, "Forbidden: Only MASTER node can accept delete requests", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.FormValue("key")
	if key == "" {
		key = r.URL.Query().Get("key")
	}
	if key == "" {
		http.Error(w, "Missing key", http.StatusBadRequest)
		return
	}

	err := s.db.Delete([]byte(key))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Replicate to slaves if this is a master node
	if s.replicator != nil {
		s.replicator.SyncDelete([]byte(key))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.syncKey != "" {
		key := r.Header.Get("X-Sync-Key")
		if key != s.syncKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var req SyncRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Action == "put" {
		err = s.db.Put(req.Key, req.Value)
	} else if req.Action == "delete" {
		err = s.db.Delete(req.Key)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("SYNC OK\n"))
}

type HealthResponse struct {
	Status        string  `json:"status"`
	Role          string  `json:"role"`
	Uptime        string  `json:"uptime"`
	MemoryAllocMB float64 `json:"memory_alloc_mb"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	res := HealthResponse{
		Status:        "OK",
		Role:          s.GetRole(),
		Uptime:        time.Since(s.startTime).Truncate(time.Second).String(),
		MemoryAllocMB: float64(m.Alloc) / 1024 / 1024,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var keys []string
	err := s.db.AscendKeys(nil, true, func(k []byte) (bool, error) {
		keys = append(keys, string(k))
		return true, nil
	})

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(keys)
}
