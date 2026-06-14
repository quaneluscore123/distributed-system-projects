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

	// Đặt vai trò ban đầu là SLAVE khi bắt đầu thăm dò nhịp tim Master
	s.SetRole("SLAVE")

	log.Printf("[Heartbeat] Đang khởi động Heartbeat Worker thăm dò Master (%s) mỗi %v...\n", masterURL, interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		client := &http.Client{
			Timeout: 2 * time.Second,
		}

		consecutiveFailures := 0

		for range ticker.C {
			// Nếu node này đã tự thăng cấp lên MASTER, ta dừng thăm dò nhịp tim.
			if s.GetRole() == "MASTER" {
				log.Println("[Heartbeat] Node hiện tại đã thăng cấp làm MASTER. Dừng thăm dò.")
				return
			}

			resp, err := client.Get(fmt.Sprintf("%s/health", masterURL))
			if err != nil {
				consecutiveFailures++
				log.Printf("[Heartbeat] LỖI kết nối tới Master (%s) - Lần %d/3: %v\n", masterURL, consecutiveFailures, err)

				if consecutiveFailures >= 3 {
					log.Printf("[Failover] PHÁT HIỆN MASTER SẬP! Đã mất kết nối %d lần liên tiếp.\n", consecutiveFailures)
					log.Println("[Failover] Tự động thăng cấp Slave hiện tại thành MASTER để phục vụ Ghi dữ liệu.")
					s.SetRole("MASTER")
					return
				}
				continue
			}

			var health HealthResponse
			err = json.NewDecoder(resp.Body).Decode(&health)
			resp.Body.Close()
			if err != nil {
				consecutiveFailures++
				log.Printf("[Heartbeat] LỖI giải mã payload từ Master - Lần %d/3: %v\n", consecutiveFailures, err)
				if consecutiveFailures >= 3 {
					log.Printf("[Failover] PHÁT HIỆN MASTER LỖI! Đã mất kết nối %d lần liên tiếp.\n", consecutiveFailures)
					log.Println("[Failover] Tự động thăng cấp Slave hiện tại thành MASTER để phục vụ Ghi dữ liệu.")
					s.SetRole("MASTER")
					return
				}
				continue
			}

			if health.Status == "OK" {
				consecutiveFailures = 0 // Reset số lần lỗi liên tiếp khi kết nối thành công
				log.Printf("[Heartbeat] Master (%s) ALIVE. Uptime: %s, RAM: %.2f MB, Vai trò: %s\n",
					masterURL, health.Uptime, health.MemoryAllocMB, health.Role)
			} else {
				consecutiveFailures++
				log.Printf("[Heartbeat] Master (%s) báo cáo trạng thái bất thường - Lần %d/3: %s\n", masterURL, consecutiveFailures, health.Status)
				if consecutiveFailures >= 3 {
					log.Printf("[Failover] PHÁT HIỆN MASTER BẤT THƯỜNG! Đã mất kết nối %d lần liên tiếp.\n", consecutiveFailures)
					log.Println("[Failover] Tự động thăng cấp Slave hiện tại thành MASTER để phục vụ Ghi dữ liệu.")
					s.SetRole("MASTER")
					return
				}
			}
		}
	}()
}
