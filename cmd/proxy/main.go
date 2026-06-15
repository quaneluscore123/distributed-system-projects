package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/rosedblabs/rosedb/v2/sharding"
)

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

	// Initialize Consistent Hash Ring (10 replicas per node is a good default)
	ring := sharding.New(10, nil)
	ring.AddNode(nodes...)

	log.Printf("Proxy server started on port %d", *port)
	log.Printf("Nodes in the ring: %v", nodes)

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

		targetNode := ring.GetNode(key)
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
