package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// SyncRequest is the payload sent from Master to Slaves
type SyncRequest struct {
	Action string `json:"action"` // "put" or "delete"
	Key    []byte `json:"key"`
	Value  []byte `json:"value,omitempty"`
}

// Replicator handles sending sync requests to all registered slaves
type Replicator struct {
	slaves []string
}

// NewReplicator creates a new replicator with a list of slave URLs
func NewReplicator(slaves []string) *Replicator {
	return &Replicator{
		slaves: slaves,
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
	for _, slave := range r.slaves {
		// Send async to avoid blocking the main master response
		go func(url string) {
			_, err := http.Post(fmt.Sprintf("%s/sync", url), "application/json", bytes.NewReader(data))
			if err != nil {
				fmt.Printf("[Replication] failed to sync to %s: %v\n", url, err)
			}
		}(slave)
	}
}
