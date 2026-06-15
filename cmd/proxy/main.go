package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rosedblabs/rosedb/v2/sharding"
)

type Proxy struct {
	mu    sync.RWMutex
	ring  *sharding.ConsistentHashRing
	nodes []string
}

func NewProxy() *Proxy {
	return &Proxy{
		ring:  sharding.New(10, nil),
		nodes: make([]string, 0),
	}
}

func (p *Proxy) AddNode(node string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ring.AddNode(node)
	p.nodes = append(p.nodes, node)
}

func (p *Proxy) GetNode(key string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ring.GetNode(key)
}

func (p *Proxy) GetNodes() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	// Return a copy
	res := make([]string, len(p.nodes))
	copy(res, p.nodes)
	return res
}

func main() {
	port := flag.Int("port", 9000, "Port to run the proxy server")
	nodesFlag := flag.String("nodes", "", "Comma-separated list of node addresses (e.g., http://localhost:8080,http://localhost:8081)")
	flag.Parse()

	if *nodesFlag == "" {
		log.Fatal("Please provide at least one node using -nodes flag")
	}

	nodes := strings.Split(*nodesFlag, ",")
	for i, node := range nodes {
		nodes[i] = strings.TrimSpace(node)
	}

	proxy := NewProxy()
	for _, node := range nodes {
		proxy.AddNode(node)
	}

	log.Printf("Proxy server started on port %d", *port)
	log.Printf("Nodes in the ring: %v", proxy.GetNodes())

	// Handler for routing requests
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Read key from form value or URL query
		key := r.FormValue("key")
		if key == "" {
			key = r.URL.Query().Get("key")
		}

		if key == "" {
			http.Error(w, "Missing key parameter", http.StatusBadRequest)
			return
		}

		targetNode := proxy.GetNode(key)
		if targetNode == "" {
			http.Error(w, "No available nodes", http.StatusInternalServerError)
			return
		}

		// Forward the request to the target node
		forwardRequest(targetNode, w, r)
	}

	// Setup routes
	http.HandleFunc("/get", handler)
	http.HandleFunc("/put", handler)
	http.HandleFunc("/delete", handler)

	http.HandleFunc("/add-node", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		newNode := r.URL.Query().Get("node")
		if newNode == "" {
			http.Error(w, "Missing node parameter", http.StatusBadRequest)
			return
		}

		newNode = strings.TrimSpace(newNode)
		newNode = strings.TrimRight(newNode, "/")

		oldNodes := proxy.GetNodes()
		proxy.AddNode(newNode)

		go migrateData(oldNodes, newNode, proxy)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Node added and migration started\n"))
	})

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Proxy server failed: %v", err)
	}
}

func forwardRequest(targetNode string, w http.ResponseWriter, r *http.Request) {
	// Trim trailing slash from target node if exists
	targetNode = strings.TrimRight(targetNode, "/")

	// Construct the target URL
	targetURL := fmt.Sprintf("%s%s", targetNode, r.URL.RequestURI())

	log.Printf("Forwarding %s request to: %s", r.Method, targetURL)

	// Create a new request based on the incoming one
	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("Error forwarding request: %v", err)
		http.Error(w, "Error forwarding request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("Error copying response body: %v", err)
	}
}

func migrateData(oldNodes []string, newNode string, proxy *Proxy) {
	log.Printf("Starting data migration for new node: %s", newNode)
	client := &http.Client{Timeout: 10 * time.Second}

	for _, oldNode := range oldNodes {
		// Fetch all keys from oldNode
		resp, err := client.Get(fmt.Sprintf("%s/keys", oldNode))
		if err != nil {
			log.Printf("Migration error: failed to fetch keys from %s: %v", oldNode, err)
			continue
		}

		var keys []string
		if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
			log.Printf("Migration error: failed to decode keys from %s: %v", oldNode, err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		for _, key := range keys {
			// Check if key belongs to the new node
			target := proxy.GetNode(key)
			if target == newNode {
				log.Printf("Migrating key '%s' from %s to %s", key, oldNode, newNode)

				// 1. Get value from old node
				getResp, err := client.Get(fmt.Sprintf("%s/get?key=%s", oldNode, key))
				if err != nil || getResp.StatusCode != http.StatusOK {
					log.Printf("Failed to get key %s from %s", key, oldNode)
					continue
				}

				valueBytes, _ := io.ReadAll(getResp.Body)
				getResp.Body.Close()
				value := strings.TrimSpace(string(valueBytes))

				// 2. Put to new node
				putReq, _ := http.NewRequest("POST", fmt.Sprintf("%s/put", newNode), strings.NewReader(fmt.Sprintf("key=%s&value=%s", key, value)))
				putReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				putResp, err := client.Do(putReq)
				if err != nil || putResp.StatusCode != http.StatusOK {
					log.Printf("Failed to put key %s to %s", key, newNode)
					continue
				}
				putResp.Body.Close()

				// 3. Delete from old node
				delReq, _ := http.NewRequest("POST", fmt.Sprintf("%s/delete", oldNode), strings.NewReader(fmt.Sprintf("key=%s", key)))
				delReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				delResp, err := client.Do(delReq)
				if err == nil {
					delResp.Body.Close()
				}
			}
		}
	}
	log.Printf("Data migration for %s completed", newNode)
}
