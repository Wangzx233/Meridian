package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var notifyHelperNames = []string{"send-back", "meridian-notify"}

var notifyHelperDir struct {
	sync.Mutex
	path string
	err  error
}

func ensureNotifyHelperDir() (string, error) {
	notifyHelperDir.Lock()
	defer notifyHelperDir.Unlock()
	if notifyHelperDir.path != "" || notifyHelperDir.err != nil {
		return notifyHelperDir.path, notifyHelperDir.err
	}
	exe, err := os.Executable()
	if err != nil {
		notifyHelperDir.err = err
		return "", err
	}
	dir, err := os.MkdirTemp("", "meridian-runner-tools-*")
	if err != nil {
		notifyHelperDir.err = err
		return "", err
	}
	for _, name := range notifyHelperNames {
		if runtime.GOOS == "windows" {
			script := "@echo off\r\n\"" + exe + "\" notify-helper %*\r\n"
			notifyHelperDir.err = os.WriteFile(filepath.Join(dir, name+".cmd"), []byte(script), 0o600)
		} else {
			script := "#!/bin/sh\nexec '" + strings.ReplaceAll(exe, "'", "'\"'\"'") + "' notify-helper \"$@\"\n"
			notifyHelperDir.err = os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700)
		}
		if notifyHelperDir.err != nil {
			return "", notifyHelperDir.err
		}
	}
	notifyHelperDir.path = dir
	return dir, nil
}

func RunNotifyHelper(args []string, stdout, stderr io.Writer) int {
	title, message, err := parseNotifyHelperArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	url := strings.TrimSpace(os.Getenv("MERIDIAN_NOTIFY_URL"))
	token := strings.TrimSpace(os.Getenv("MERIDIAN_NOTIFY_TOKEN"))
	if url == "" || token == "" {
		fmt.Fprintln(stderr, "Send back is not available in this process.")
		return 1
	}
	body, _ := json.Marshal(map[string]string{
		"title":   title,
		"message": message,
	})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		text, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		fmt.Fprintf(stderr, "Send back failed: %s %s\n", resp.Status, strings.TrimSpace(string(text)))
		return 1
	}
	fmt.Fprintln(stdout, "Sent back.")
	return 0
}

func parseNotifyHelperArgs(args []string) (string, string, error) {
	var title string
	var message string
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--title":
			i++
			if i >= len(args) {
				return "", "", errors.New("--title requires a value")
			}
			title = args[i]
		case "--message":
			i++
			if i >= len(args) {
				return "", "", errors.New("--message requires a value")
			}
			message = args[i]
		default:
			rest = append(rest, args[i])
		}
	}
	if strings.TrimSpace(message) == "" && len(rest) > 0 {
		message = strings.Join(rest, " ")
	}
	title = strings.TrimSpace(title)
	message = strings.TrimSpace(message)
	if title == "" && message == "" {
		return "", "", errors.New("usage: send-back --title \"...\" --message \"...\"")
	}
	return title, message, nil
}
