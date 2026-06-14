package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SyncRequest is the payload sent from Master to Slaves
type SyncRequest struct {
	Action string `json:"action"` // "put" or "delete"
	Key    []byte `json:"key"`
	Value  []byte `json:"value,omitempty"`
}

// Replicator handles sending sync requests to all registered slaves
type Replicator struct {
	slaves  []string
	syncKey string
}

// NewReplicator creates a new replicator with a list of slave URLs
func NewReplicator(slaves []string, syncKey string) *Replicator {
	return &Replicator{
		slaves:  slaves,
		syncKey: syncKey,
	}
}

// SyncPut sends a put operation to all slaves
func (r *Replicator) SyncPut(key, value []byte) {
	req := SyncRequest{Action: "put", Key: key, Value: value}
	r.broadcast(req)
}

// SyncDelete sends a delete operation to all slaves
func (r *Replicator) SyncDelete(key []byte) {
	req := SyncRequest{Action: "delete", Key: key}
	r.broadcast(req)
}

// broadcast asynchronously sends the request to all slaves
func (r *Replicator) broadcast(req SyncRequest) {
	if len(r.slaves) == 0 {
		return
	}
	data, _ := json.Marshal(req)

	client := &http.Client{
		Timeout: 3 * time.Second, // Cài đặt Timeout 3 giây
	}

	for _, slave := range r.slaves {
		// Send async to avoid blocking the main master response
		go func(url string) {
			maxRetries := 3
			for i := 1; i <= maxRetries; i++ {
				reqHTTP, err := http.NewRequest("POST", fmt.Sprintf("%s/sync", url), bytes.NewReader(data))
				if err != nil {
					fmt.Printf("[Replication] Không thể tạo request: %v\n", err)
					return
				}
				reqHTTP.Header.Set("Content-Type", "application/json")
				if r.syncKey != "" {
					reqHTTP.Header.Set("X-Sync-Key", r.syncKey)
				}

				resp, err := client.Do(reqHTTP)

				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						// Đồng bộ thành công
						return
					}
					err = fmt.Errorf("trạng thái phản hồi không hợp lệ: %d", resp.StatusCode)
				}

				fmt.Printf("[Replication] Đồng bộ thất bại tới %s (lần thử %d/%d): %v\n", url, i, maxRetries, err)

				if i < maxRetries {
					time.Sleep(1 * time.Second) // Chờ 1 giây trước khi retry
				}
			}
			fmt.Printf("[Replication] LỖI: Bỏ cuộc đồng bộ tới %s sau %d lần thử.\n", url, maxRetries)
		}(slave)
	}
}
