package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/rosedblabs/rosedb/v2"
	"github.com/rosedblabs/rosedb/v2/server"
)

func main() {
	port := flag.Int("port", 8080, "Port to run the HTTP server on")
	dbPath := flag.String("dbpath", "/tmp/rosedb_server", "Path to store the database files")
	slavesStr := flag.String("slaves", "", "Comma-separated list of slave URLs (e.g. http://localhost:8081)")
	flag.Parse()

	// Initialize RoseDB options
	options := rosedb.DefaultOptions
	options.DirPath = *dbPath

	// Open database
	db, err := rosedb.Open(options)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	// Initialize replication (only active if slaves are provided)
	var replicator *server.Replicator
	if *slavesStr != "" {
		slaves := strings.Split(*slavesStr, ",")
		for i := range slaves {
			slaves[i] = strings.TrimSpace(slaves[i])
		}
		replicator = server.NewReplicator(slaves)
		fmt.Printf("Running as MASTER with %d slaves\n", len(slaves))
	} else {
		fmt.Println("Running as Standalone or SLAVE node")
	}

	// Create and start the server
	srv := server.NewServer(db, replicator)
	
	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("RoseDB HTTP Server is listening on %s\n", addr)
	fmt.Printf("Data directory: %s\n", *dbPath)
	
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
