package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"eth-blockchain-parser/pkg/database"
	"eth-blockchain-parser/pkg/server"
)

func main() {
	// Command line flags
	var (
		dbPath   = flag.String("db", "./blockchain.db", "Path to SQLite database file")
		port     = flag.String("port", "8015", "HTTP server port")
		host     = flag.String("host", "localhost", "HTTP server host")
		username = flag.String("username", "admin", "Basic auth username")
		password = flag.String("password", "password123", "Basic auth password")
	)
	flag.Parse()

	// Create logger
	logger := log.New(os.Stdout, "[HTTP-SERVER] ", log.LstdFlags|log.Lshortfile)
	logger.Println("Starting SQLite HTTP API Server")

	dbConfig := database.DefaultConfig(*dbPath)
	dbManager, err := database.NewDatabaseManager(dbConfig, logger)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbManager.Close()

	//ctx := context.Background()

	// Test database connection
	if err := dbManager.Ping(); err != nil {
		logger.Fatalf("Failed to ping database: %v", err)
	}
	logger.Println("Database connection successful")

	// Server configuration
	serverConfig := &server.ServerConfig{
		Port:     *port,
		Host:     *host,
		Username: *username,
		Password: *password,
	}

	// Create HTTP server
	httpServer := server.NewServer(dbManager, serverConfig, logger)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Println("Received shutdown signal, stopping server...")
		os.Exit(0)
	}()

	// Start HTTP server
	logger.Printf("Server configuration:")
	logger.Printf("  Database: %s", *dbPath)
	logger.Printf("  Listen: %s:%s", *host, *port)
	logger.Printf("  Username: %s", *username)
	logger.Printf("  Password: %s", *password)

	if err := httpServer.Start(); err != nil {
		logger.Fatalf("HTTP server failed: %v", err)
	}
}
