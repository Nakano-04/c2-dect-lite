package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// cmdTimeout es el techo de una orden externa (tasklist, ipconfig, shell...).
// Sin este techo, una orden bloqueada (tasklist se queda pegado cuando el
// SCM va lento) deja al agente sin beacon y la sesion se queda colgada.
const cmdTimeout = 20 * time.Second

// runCmd ejecuta una orden externa con limite de tiempo.
func runCmd(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// execute runs one task and returns its output.
//
// El conjunto de ordenes es cerrado: una orden desconocida se rechaza en vez
// de ejecutarse por la shell. Ademas, las ordenes de IMPACTO (shell, kill, rm,
// mv) solo se ejecutan si el agente arranco con -allow-impact.
func execute(cfg *Config, task *taskSpec) (string, string, bool) {
	name := strings.TrimSpace(strings.ToLower(task.Command))
	args := task.Args
	if args == "" && strings.Contains(task.Command, " ") {
		parts := strings.SplitN(task.Command, " ", 2)
		name = strings.ToLower(parts[0])
		args = parts[1]
	}

	spec, known := LookupCommand(name)
	if !known {
		// El script firmado es un caso aparte: no va por el catalogo, lo
		// ejecuta el runtime con su propio sandbox.
		if name == "script" {
			out, serr := executeScriptTask(cfg, args)
			if serr != nil {
				return out, serr.Error(), false
			}
			return out, "", true
		}
		return "", fmt.Sprintf("unknown command %q", name), false
	}
	if !cfg.Allowed(spec) {
		return "", fmt.Sprintf("the command %q has impact level %s and the agent was started without -allow-impact", name, spec.Risk), false
	}

	switch spec.Name {
	case "sysinfo":
		return systemInfo()
	case "hostname":
		host, _ := os.Hostname()
		return host, "", true
	case "whoami":
		return currentUsername(), "", true
	case "uptime":
		return fmt.Sprintf("uptime: %s", time.Since(startTime).Round(time.Second)), "", true
	case "netinfo":
		return networkInfo()
	case "ps":
		return processList()
	case "kill":
		return killProcess(args)
	case "services":
		return runningServices()
	case "env":
		vars, _, _ := environmentVars()
		return vars, "", true
	case "connections":
		return networkConnections()
	case "hashfile":
		return hashFile(args)
	case "stat":
		return fileStat(args)
	case "ls":
		return listFiles(args)
	case "cat":
		return readFileText(args)
	case "upload":
		return uploadFile(args)
	case "download":
		return downloadFile(args)
	case "cp":
		return copyFile(args)
	case "mv":
		return moveFile(args)
	case "rm":
		return deleteFile(args)
	case "mkdir":
		return createDirectory(args)
	case "find":
		return searchFiles(args)
	case "shell":
		return runShell(args)
	case "sleep":
		secs := atoi(args)
		if secs < 1 || secs > 86400 {
			return "", "the sleep value must be between 1 and 86400 seconds", false
		}
		cfg.SleepSec = uint32(secs)
		return fmt.Sprintf("sleep set to %d seconds", secs), "", true
	case "exit":
		return "agent terminated by the operator", "", true
	default:
		return "", fmt.Sprintf("command %q is not implemented", spec.Name), false
	}
}

// runShell executes a command through the system shell.
func runShell(command string) (string, string, bool) {
	if strings.TrimSpace(command) == "" {
		return "", "no command supplied", false
	}
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = runCmd("cmd.exe", "/C", command)
	} else {
		out, err = runCmd("sh", "-c", command)
	}
	if err != nil {
		return string(out), err.Error(), false
	}
	return string(out), "", true
}

// systemInfo returns the inventory of the host.
func systemInfo() (string, string, bool) {
	hostname, _ := os.Hostname()
	wd, _ := os.Getwd()
	info := fmt.Sprintf(`Hostname: %s
Username: %s
OS: %s
Arch: %s
PID: %d
Working directory: %s
Uptime: %s
Time: %s`,
		hostname, currentUsername(), runtime.GOOS, runtime.GOARCH, os.Getpid(), wd,
		time.Since(startTime).Round(time.Second), time.Now().Format("2006-01-02 15:04:05"))
	return info, "", true
}

// processList lists the running processes in a portable way.
func processList() (string, string, bool) {
	if runtime.GOOS == "windows" {
		out, err := runCmd("tasklist", "/FO", "CSV")
		if err != nil {
			return string(out), err.Error(), false
		}
		return string(out), "", true
	}
	out, err := runCmd("ps", "-eo", "pid,ppid,user,etime,comm,args")
	if err != nil {
		return string(out), err.Error(), false
	}
	return string(out), "", true
}

// killProcess terminates a process.
func killProcess(pid string) (string, string, bool) {
	id, err := strconv.Atoi(strings.TrimSpace(pid))
	if err != nil || id <= 0 {
		return "", "a valid process id is required", false
	}
	var out []byte
	if runtime.GOOS == "windows" {
		out, err = runCmd("taskkill", "/F", "/PID", strconv.Itoa(id))
	} else {
		out, err = runCmd("kill", "-9", strconv.Itoa(id))
	}
	if err != nil {
		return string(out), err.Error(), false
	}
	return fmt.Sprintf("process %d terminated", id), "", true
}

// runningServices lists the services of the host.
func runningServices() (string, string, bool) {
	var name string
	var args []string
	switch runtime.GOOS {
	case "windows":
		name, args = "sc", []string{"query", "state=", "all"}
	case "linux":
		name, args = "systemctl", []string{"list-units", "--type=service", "--no-pager", "--no-legend"}
	case "darwin":
		name, args = "launchctl", []string{"list"}
	default:
		return "service listing is not available on this platform", "", true
	}
	out, err := runCmd(name, args...)
	if err != nil && len(out) == 0 {
		return "", err.Error(), false
	}
	return string(out), "", true
}

// networkInfo returns the configuration of the network interfaces.
func networkInfo() (string, string, bool) {
	var name string
	var args []string
	switch runtime.GOOS {
	case "windows":
		name, args = "ipconfig", []string{"/all"}
	case "linux":
		name, args = "ip", []string{"-o", "addr"}
	case "darwin":
		name, args = "ifconfig", nil
	default:
		name, args = "hostname", []string{"-I"}
	}
	out, err := runCmd(name, args...)
	if err != nil && len(out) == 0 {
		return "", err.Error(), false
	}
	return string(out), "", true
}

// networkConnections lists the established network connections.
func networkConnections() (string, string, bool) {
	var name string
	var args []string
	switch runtime.GOOS {
	case "windows":
		name, args = "netstat", []string{"-ano"}
	case "linux":
		name, args = "ss", []string{"-tunap"}
	default:
		name, args = "netstat", []string{"-an"}
	}
	out, err := runCmd(name, args...)
	if err != nil && len(out) == 0 {
		return "", err.Error(), false
	}
	return string(out), "", true
}

// environmentVars returns the environment of the agent.
func environmentVars() (string, string, bool) {
	return strings.Join(os.Environ(), "\n"), "", true
}

// fileStat returns the metadata of a file.
func fileStat(path string) (string, string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "a path is required", false
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err.Error(), false
	}
	return fmt.Sprintf("Name: %s\nSize: %s\nModified: %s\nDirectory: %v\nMode: %s",
		info.Name(), formatSize(info.Size()), info.ModTime().Format("2006-01-02 15:04:05"),
		info.IsDir(), info.Mode().String()), "", true
}

// listFiles lists the content of a directory.
func listFiles(path string) (string, string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err.Error(), false
		}
		path = home
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err.Error(), false
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory: %s\n", path))
	sb.WriteString("-----------------------------------------\n")
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if entry.IsDir() {
			sb.WriteString(fmt.Sprintf("[DIR]  %s/\n", entry.Name()))
			continue
		}
		sb.WriteString(fmt.Sprintf("      %s (%s)\n", entry.Name(), formatSize(info.Size())))
	}
	return sb.String(), "", true
}

// readFileText returns the content of a file.
func readFileText(path string) (string, string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "a path is required", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err.Error(), false
	}
	return string(data), "", true
}

// uploadFile writes the payload sent by the operator on disk.
func uploadFile(args string) (string, string, bool) {
	parts := strings.SplitN(args, "|", 2)
	if len(parts) != 2 {
		return "", "usage: upload <remote_path>|<base64 payload>", false
	}
	target := strings.TrimSpace(parts[0])
	data, err := decodeBase64(parts[1])
	if err != nil {
		return "", err.Error(), false
	}
	if target == "" {
		target = filepath.Join(dataDir(), "uploads", fmt.Sprintf("upload_%d.bin", time.Now().UnixMilli()))
	}
	if err := writeFile(target, data); err != nil {
		return "", err.Error(), false
	}
	return fmt.Sprintf("file written: %s (%d bytes)", target, len(data)), "", true
}

// downloadFile returns the content of a file, base64 encoded.
func downloadFile(path string) (string, string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "a path is required", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err.Error(), false
	}
	return encodeBase64(data), "", true
}

// copyFile copies a file.
func copyFile(args string) (string, string, bool) {
	parts := strings.Fields(args)
	if len(parts) != 2 {
		return "", "usage: cp <source> <destination>", false
	}
	if err := copyFileImpl(parts[0], parts[1]); err != nil {
		return "", err.Error(), false
	}
	return fmt.Sprintf("copied %s to %s", parts[0], parts[1]), "", true
}

// moveFile moves a file.
func moveFile(args string) (string, string, bool) {
	parts := strings.Fields(args)
	if len(parts) != 2 {
		return "", "usage: mv <source> <destination>", false
	}
	if err := os.Rename(parts[0], parts[1]); err != nil {
		return "", err.Error(), false
	}
	return fmt.Sprintf("moved %s to %s", parts[0], parts[1]), "", true
}

// deleteFile removes a file.
func deleteFile(path string) (string, string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "a path is required", false
	}
	if err := os.Remove(path); err != nil {
		return "", err.Error(), false
	}
	return fmt.Sprintf("deleted %s", path), "", true
}

// createDirectory creates a directory.
func createDirectory(path string) (string, string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "a path is required", false
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err.Error(), false
	}
	return fmt.Sprintf("created %s", path), "", true
}

// searchFiles walks a tree looking for a pattern.
func searchFiles(args string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	if len(parts) != 2 {
		return "", "usage: find <directory> <pattern>", false
	}
	dir := strings.TrimSpace(parts[0])
	pattern := strings.TrimSpace(parts[1])
	if dir == "" {
		dir = "."
	}
	var sb strings.Builder
	count := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		matched, merr := filepath.Match(pattern, info.Name())
		if merr != nil {
			return nil
		}
		if matched {
			sb.WriteString(path + "\n")
			count++
			if count > 2000 {
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return sb.String(), err.Error(), false
	}
	return sb.String(), "", true
}

// hashFile returns the SHA-256 digest of a file.
func hashFile(path string) (string, string, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "a path is required", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err.Error(), false
	}
	return fmt.Sprintf("%s  %s (%d bytes)", sha256Hex(data), path, len(data)), "", true
}

// formatSize renders a size in human units.
func formatSize(size int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case size >= gb:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(gb))
	case size >= mb:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(kb))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

