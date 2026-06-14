package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rosedblabs/rosedb/v2"
)

// setupTestDB creates a temporary RoseDB for testing.
// Returns the database instance and a cleanup function.
func setupTestDB(t *testing.T) (*rosedb.DB, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "rosedb-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	options := rosedb.DefaultOptions
	options.DirPath = tempDir

	db, err := rosedb.Open(options)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open test db: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, cleanup
}

func TestStandaloneCRUD(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Khởi tạo server không có replicator
	srv := NewServer(db, nil, "")

	// Test /put
	form := url.Values{}
	form.Add("key", "hello")
	form.Add("value", "world")
	req, _ := http.NewRequest("POST", "/put", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Test /get
	req, _ = http.NewRequest("GET", "/get?key=hello", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	if strings.TrimSpace(rr.Body.String()) != "world" {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), "world\n")
	}

	// Test /delete
	form = url.Values{}
	form.Add("key", "hello")
	req, _ = http.NewRequest("POST", "/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Verify /get returns 404 after deletion
	req, _ = http.NewRequest("GET", "/get?key=hello", nil)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %v", rr.Code)
	}
}

func TestHandleHealth(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	srv := NewServer(db, nil, "")

	req, _ := http.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var res HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Errorf("failed to unmarshal health response: %v", err)
	}

	if res.Status != "OK" {
		t.Errorf("expected status OK, got %v", res.Status)
	}
	if res.Role != "MASTER" { // Standalone node allows read and write, so it is MASTER
		t.Errorf("expected role MASTER, got %v", res.Role)
	}
}

func TestSyncAuthSecurity(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Khởi tạo server với API Key
	srv := NewServer(db, nil, "sieubaomat")

	syncData := SyncRequest{Action: "put", Key: []byte("k1"), Value: []byte("v1")}
	body, _ := json.Marshal(syncData)

	// Case 1: Không gửi auth header (Hacker)
	req1, _ := http.NewRequest("POST", "/sync", bytes.NewReader(body))
	rr1 := httptest.NewRecorder()
	srv.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for no auth, got %v", rr1.Code)
	}

	// Case 2: Gửi sai auth header
	req2, _ := http.NewRequest("POST", "/sync", bytes.NewReader(body))
	req2.Header.Set("X-Sync-Key", "sai-pass")
	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for wrong auth, got %v", rr2.Code)
	}

	// Case 3: Gửi ĐÚNG auth header
	req3, _ := http.NewRequest("POST", "/sync", bytes.NewReader(body))
	req3.Header.Set("X-Sync-Key", "sieubaomat")
	rr3 := httptest.NewRecorder()
	srv.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Errorf("expected 200 OK for correct auth, got %v", rr3.Code)
	}

	// Verify key was saved
	val, err := db.Get([]byte("k1"))
	if err != nil || string(val) != "v1" {
		t.Errorf("sync failed to save data")
	}
}

func TestMasterSlaveReplication(t *testing.T) {
	dbMaster, cleanupMaster := setupTestDB(t)
	defer cleanupMaster()

	// Tạo 1 Mock Slave Server
	var slaveReceivedSync bool
	mockSlave := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sync" {
			if r.Header.Get("X-Sync-Key") == "matkhau" {
				slaveReceivedSync = true
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mockSlave.Close()

	// Khởi tạo Master trỏ tới Mock Slave
	replicator := NewReplicator([]string{mockSlave.URL}, "matkhau")
	srvMaster := NewServer(dbMaster, replicator, "matkhau")

	// Tạo request PUT trên Master
	form := url.Values{}
	form.Add("key", "sync_test")
	form.Add("value", "12345")
	req, _ := http.NewRequest("POST", "/put", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srvMaster.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("put failed on master: %v", rr.Code)
	}

	// Vì replication broadcast là async (goroutine), chúng ta đợi một chút để goroutine chạy xong
	time.Sleep(100 * time.Millisecond)

	if !slaveReceivedSync {
		t.Errorf("Slave did not receive sync request, or failed auth")
	}
}
