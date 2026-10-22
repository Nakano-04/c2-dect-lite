// Command server runs the C2-DECT Lite 2.0 API server.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"c2-dect/server/api"
	"c2-dect/server/auth"
	"c2-dect/server/db"
	"c2-dect/server/marketplace"
	"c2-dect/server/profiles"

	"github.com/gin-gonic/gin"
)

func main() {
	addr := flag.String("addr", envOr("C2_LISTEN_ADDR", "127.0.0.1:8443"), "listen address")
	dbPath := flag.String("db", envOr("C2_DB_PATH", "c2-dect-lite.db"), "sqlite database path")
	profileDir := flag.String("profiles", envOr("C2_PROFILE_DIR", "profiles"), "traffic profile directory")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("C2-DECT Lite server " + api.Version)
		return
	}

	fmt.Println()
	fmt.Println("  C2-DECT Lite " + api.Version + " (open source)")
	fmt.Println("  -------------------------------------------------")
	fmt.Println()

	if !envBool("C2_DEBUG", false) {
		gin.SetMode(gin.ReleaseMode)
	}

	// The JWT secret is mandatory: the server refuses to start without it so
	// a fresh clone can never run with a well known signing key.
	jwtMgr, err := auth.NewJWTManager(envInt("C2_ACCESS_MINUTES", 30), envInt("C2_REFRESH_HOURS", 12))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[-] %v\n", err)
		fmt.Fprintln(os.Stderr, "[-] Set C2_JWT_SECRET to a random string of at least 32 characters.")
		os.Exit(1)
	}

	database, err := db.New(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[-] database error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Marketplace signing key: taken from the environment when provided,
	// otherwise generated once and stored in the database.
	var signer *marketplace.Signer
	if seedHex := strings.TrimSpace(os.Getenv("C2_MARKET_SIGN_SEED")); seedHex != "" {
		seed, err := hex.DecodeString(seedHex)
		if err != nil || len(seed) != 32 {
			fmt.Fprintln(os.Stderr, "[-] C2_MARKET_SIGN_SEED must be 32 bytes in hex")
			os.Exit(1)
		}
		signer, err = marketplace.NewSigner(seed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[-] marketplace signer error: %v\n", err)
			os.Exit(1)
		}
	} else {
		_, seed, err := database.EnsureMarketKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[-] marketplace key error: %v\n", err)
			os.Exit(1)
		}
		signer, err = marketplace.NewSigner(seed)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[-] marketplace signer error: %v\n", err)
			os.Exit(1)
		}
	}

	catalog := marketplace.NewCatalog(signer)
	profMgr := profiles.NewProfileManager(*profileDir)

	srv := api.NewServer(database, jwtMgr, profMgr, catalog)
	router := srv.SetupRouter()

	fmt.Printf("[+] listening on      %s\n", *addr)
	fmt.Printf("[+] database          %s\n", *dbPath)
	fmt.Printf("[+] traffic profiles  %s\n", *profileDir)
	fmt.Printf("[+] marketplace key   %s\n", signer.Kid())
	fmt.Printf("[+] script pubkey     %s\n", signer.PubHex())
	fmt.Printf("[+]                   (ponla en el agente: C2_SCRIPT_PUBKEY=%s)\n", signer.PubHex())
	if user := strings.TrimSpace(os.Getenv("C2_BOOTSTRAP_USER")); user != "" {
		fmt.Printf("[+] bootstrap user    %s\n", user)
	} else {
		fmt.Println("[+] no bootstrap user  set C2_BOOTSTRAP_USER and C2_BOOTSTRAP_PASSWORD on the first run")
	}
	fmt.Println("[+] REST API          /api/")
	fmt.Println("[+] agent check-in    /agent/checkin")
	fmt.Println()
	fmt.Println("    Press Ctrl+C to stop")
	fmt.Println()

	if err := router.Run(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "[-] server error: %v\n", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return fallback
	}
	return n
}
