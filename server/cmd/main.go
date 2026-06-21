package main

import (
	"c2-dect/server/api"
	"c2-dect/server/auth"
	"c2-dect/server/db"
	"c2-dect/server/profiles"
	"fmt"
	"os"
)

const (
	defaultAddr  = "0.0.0.0:8443"
	dbPath       = "c2-dect-lite.db"
	profileDir   = "profiles"
	jwtSecret    = "c2-dect-jwt-secret-change-me"
	jwtExpiryMin = 480 // 8 hours
)

func main() {
	addr := defaultAddr
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║          C2-DECT Lite v1.0 (Open Source)        ║")
	fmt.Println("║     Multi-session C2 with REST API & JWT        ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()

	// Initialize database
	database, err := db.New(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[-] Database error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Create default admin user
	if err := database.InitDefaultUser(); err != nil {
		fmt.Fprintf(os.Stderr, "[-] Default user error: %v\n", err)
	}

	// Initialize JWT manager
	jwtMgr := auth.NewJWTManager(jwtSecret, jwtExpiryMin)

	// Initialize profile manager
	profMgr := profiles.NewProfileManager(profileDir)

	// Create API server
	srv := api.NewServer(database, jwtMgr, profMgr)
	router := srv.SetupRouter()

	fmt.Printf("[+] Server listening on %s\n", addr)
	fmt.Printf("[+] Database: %s\n", dbPath)
	fmt.Printf("[+] Default credentials: admin / c2-dect\n")
	fmt.Printf("[+] REST API: http://%s/api/\n", addr)
	fmt.Printf("[+] Agent checkin: http://%s/agent/checkin\n", addr)
	fmt.Println()
	fmt.Println("[*] Press Ctrl+C to stop")

	if err := router.Run(addr); err != nil {
		fmt.Fprintf(os.Stderr, "[-] Server error: %v\n", err)
		os.Exit(1)
	}
}
