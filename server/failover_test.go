package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestFailoverPromotion(t *testing.T) {
	dbSlave, cleanupSlave := setupTestDB(t)
	defer cleanupSlave()

	// 1. Tạo Mock Master Server
	var masterHeartbeatCalls int
	mockMasterMux := http.NewServeMux()
	mockMasterMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		masterHeartbeatCalls++
		res := HealthResponse{
			Status:        "OK",
			Role:          "MASTER",
			Uptime:        "10s",
			MemoryAllocMB: 1.5,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(res)
	})

	mockMaster := httptest.NewServer(mockMasterMux)

	// 2. Tạo Slave Server thực tế kết nối tới Mock Master
	srvSlave := NewServer(dbSlave, nil, "")

	// 3. Kích hoạt Heartbeat worker từ Slave trỏ tới Mock Master
	// Đặt chu kỳ thăm dò siêu ngắn (30ms) để test nhanh
	srvSlave.StartHeartbeatWorker(mockMaster.URL, 30*time.Millisecond)

	// Đợi cho Heartbeat chạy thành công một vài lần
	time.Sleep(150 * time.Millisecond)
	if masterHeartbeatCalls == 0 {
		t.Error("expected heartbeat worker to call mock master, but call count is 0")
	}
	if srvSlave.GetRole() != "SLAVE" {
		t.Errorf("expected role to remain SLAVE during active master connection, got %s", srvSlave.GetRole())
	}

	// 4. Kiểm tra: Ghi dữ liệu trực tiếp vào Slave ban đầu PHẢI bị cấm (403)
	form := url.Values{}
	form.Add("key", "secret")
	form.Add("value", "treasure")
	reqPut, _ := http.NewRequest("POST", "/put", strings.NewReader(form.Encode()))
	reqPut.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrPut := httptest.NewRecorder()
	srvSlave.ServeHTTP(rrPut, reqPut)

	if rrPut.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for writing to slave, got %d", rrPut.Code)
	}

	// 5. Tắt Mock Master (Giả lập Master sập đột ngột)
	mockMaster.Close()

	// Đợi 250ms (đủ thời gian cho 3 lần thăm dò lỗi: 3 x 30ms = 90ms + trừ hao)
	time.Sleep(200 * time.Millisecond)

	// 6. Kiểm tra xem Slave đã tự động chuyển đổi sang MASTER chưa
	if srvSlave.GetRole() != "MASTER" {
		t.Errorf("expected slave to self-promote to MASTER, but current role is %s", srvSlave.GetRole())
	}

	// 7. Kiểm tra: Gọi Ghi dữ liệu trực tiếp lên Slave (đã thăng chức thành MASTER mới)
	rrPutAfterPromotion := httptest.NewRecorder()
	srvSlave.ServeHTTP(rrPutAfterPromotion, reqPut)

	if rrPutAfterPromotion.Code != http.StatusOK {
		t.Errorf("expected 200 OK for writing to promoted master, got %d", rrPutAfterPromotion.Code)
	}

	// Xác nhận dữ liệu được lưu thành công trên database của Slave
	val, err := dbSlave.Get([]byte("secret"))
	if err != nil || string(val) != "treasure" {
		t.Errorf("expected to read 'treasure' from slave db, got '%s' (err: %v)", val, err)
	}
}
