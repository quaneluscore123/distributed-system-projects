package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// StartHeartbeatWorker bắt đầu một goroutine chạy ngầm gửi yêu cầu GET /health tới Master mỗi interval.
func (s *Server) StartHeartbeatWorker(masterURL string, interval time.Duration) {
	if masterURL == "" {
		return
	}

	log.Printf("[Heartbeat] Đang khởi động Heartbeat Worker thăm dò Master (%s) mỗi %v...\n", masterURL, interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		client := &http.Client{
			Timeout: 2 * time.Second,
		}

		for range ticker.C {
			resp, err := client.Get(fmt.Sprintf("%s/health", masterURL))
			if err != nil {
				log.Printf("[Heartbeat] LỖI kết nối tới Master (%s): %v\n", masterURL, err)
				// Sẽ xử lý logic failover (bầu chọn) ở Ngày 3 khi Master chết
				continue
			}

			var health HealthResponse
			err = json.NewDecoder(resp.Body).Decode(&health)
			resp.Body.Close()
			if err != nil {
				log.Printf("[Heartbeat] LỖI giải mã payload từ Master: %v\n", err)
				continue
			}

			if health.Status == "OK" {
				log.Printf("[Heartbeat] Master (%s) ALIVE. Uptime: %s, RAM: %.2f MB, Vai trò: %s\n",
					masterURL, health.Uptime, health.MemoryAllocMB, health.Role)
			} else {
				log.Printf("[Heartbeat] Master (%s) báo cáo trạng thái bất thường: %s\n", masterURL, health.Status)
			}
		}
	}()
}
