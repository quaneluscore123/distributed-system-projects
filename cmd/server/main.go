package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rosedblabs/rosedb/v2"
	"github.com/rosedblabs/rosedb/v2/server"
)

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Started %s %s", r.Method, r.RequestURI)
		next.ServeHTTP(w, r)
		log.Printf("Completed %s %s in %v", r.Method, r.RequestURI, time.Since(start))
	})
}

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
		log.Println("Closing database...")
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		} else {
			log.Println("Database closed successfully")
		}
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

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: loggingMiddleware(srv),
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("RoseDB HTTP Server is listening on %s\n", addr)
		fmt.Printf("Data directory: %s\n", *dbPath)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Channel to listen for OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Printf("Server startup failed: %v", err)
		return // Return thay vì log.Fatalf để hàm defer db.Close() có cơ hội chạy
	case <-quit:
		log.Println("Shutting down server...")
	}

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting gracefully")
}
