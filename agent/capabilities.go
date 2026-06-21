package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ==================== DATA DIRECTORY ====================

func getDataDir() string {
	exePath, _ := os.Executable()
	base := filepath.Dir(exePath)
	dataDir := filepath.Join(base, "c2-dect-lite", "data")
	os.MkdirAll(filepath.Join(dataDir, "downloads"), 0755)
	os.MkdirAll(filepath.Join(dataDir, "uploads"), 0755)
	return dataDir
}

// ==================== PROCESS ====================

func getProcessList() (string, error) {
	cmd := exec.Command("tasklist", "/FO", "CSV")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func killProcess(pid string) error {
	cmd := exec.Command("taskkill", "/F", "/PID", pid)
	return cmd.Run()
}

// ==================== SYSTEM INFO ====================

func getSystemInfo() (string, error) {
	hostname, _ := os.Hostname()
	homeDir, _ := os.UserHomeDir()
	username := filepath.Base(homeDir)
	admin := isAdmin()

	info := fmt.Sprintf(`Hostname: %s
Username: %s
OS: windows
Arch: amd64
Admin: %v
Time: %s`,
		hostname, username, admin, time.Now().Format("2006-01-02 15:04:05"))

	return info, nil
}

func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

func getNetworkInfo() (string, error) {
	cmd := exec.Command("ipconfig", "/all")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func getRunningServices() (string, error) {
	cmd := exec.Command("net", "start")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func getEnvironmentVars() (string, error) {
	vars := os.Environ()
	result := ""
	for _, v := range vars {
		result += v + "\n"
	}
	return result, nil
}

func getConnections() (string, error) {
	cmd := exec.Command("netstat", "-ano")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ==================== HELPER ====================

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func getFileInfo(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Name: %s\nSize: %s\nModified: %s\nIsDir: %v",
		info.Name(), formatSize(info.Size()), info.ModTime().Format("2006-01-02 15:04:05"), info.IsDir()), nil
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if strings.Contains(str, v) {
			return true
		}
	}
	return false
}
