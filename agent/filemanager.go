package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func listFiles(path string) (string, error) {
	if path == "" {
		homeDir, _ := os.UserHomeDir()
		path = homeDir
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Directory: %s\n", path))
	result.WriteString("─────────────────────────────────────\n")

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("[DIR]  %s/\n", entry.Name()))
		} else {
			size := formatSize(info.Size())
			result.WriteString(fmt.Sprintf("      %s (%s)\n", entry.Name(), size))
		}
	}

	return result.String(), nil
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writeFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func deleteFile(path string) error {
	return os.Remove(path)
}

func moveFile(src, dst string) error {
	return os.Rename(src, dst)
}

func createDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

func searchFiles(dir string, pattern string) (string, error) {
	var results strings.Builder
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			results.WriteString(path + "\n")
		}
		return nil
	})
	return results.String(), err
}
