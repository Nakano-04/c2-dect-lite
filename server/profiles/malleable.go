// Package profiles implements the traffic profiles applied to the agent
// transport: request path, user agent, headers and beacon timing.
package profiles

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// randomHex returns a random hex string of n bytes.
func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(buf)
}

// MalleableProfile describes how the agent shapes its traffic.
type MalleableProfile struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Method       string            `json:"method"` // GET or POST
	URIs         []string          `json:"uris"`
	UserAgents   []string          `json:"user_agents"`
	Headers      map[string]string `json:"headers"`
	Cookies      map[string]string `json:"cookies"`
	PostData     string            `json:"post_data"`
	Prepend      string            `json:"prepend"`
	Append       string            `json:"append"`
	Jitter       int               `json:"jitter"` // percentage
	DefaultSleep int               `json:"default_sleep"`
	ContentType  string            `json:"content_type"`
	EncodeMode   string            `json:"encode_mode"` // json or base64
	KillDate     string            `json:"kill_date"`   // YYYY-MM-DD, optional
}

// AgentProfile is the projection of a traffic profile handed to the agents.
type AgentProfile struct {
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	URIs        []string          `json:"uris"`
	UserAgents  []string          `json:"user_agents"`
	Headers     map[string]string `json:"headers"`
	Cookies     map[string]string `json:"cookies"`
	ContentType string            `json:"content_type"`
	EncodeMode  string            `json:"encode_mode"`
	Prepend     string            `json:"prepend"`
	Append      string            `json:"append"`
}

// ProfileManager stores the profiles of the server.
type ProfileManager struct {
	profiles map[string]*MalleableProfile
	current  string
	mu       sync.RWMutex
	dir      string
}

func NewProfileManager(dir string) *ProfileManager {
	pm := &ProfileManager{
		profiles: make(map[string]*MalleableProfile),
		dir:      dir,
	}
	pm.loadDefaults()
	pm.loadFromDisk()
	return pm
}

func (pm *ProfileManager) loadDefaults() {
	pm.profiles["browser"] = &MalleableProfile{
		Name:        "browser",
		Description: "Shaped like a regular browser request",
		Method:      "POST",
		URIs:        []string{"/api/update", "/login", "/dashboard/data"},
		UserAgents: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		},
		Headers: map[string]string{
			"Accept":          "application/json, text/plain, */*",
			"Accept-Language": "en-US,en;q=0.9",
			"Connection":      "keep-alive",
			"Referer":         "https://app.example.com/dashboard",
		},
		Cookies:     map[string]string{"session": "{{random}}"},
		PostData:    `{"data":"{{payload}}","timestamp":{{timestamp}},"nonce":"{{nonce}}"}`,
		Jitter:      30,
		DefaultSleep: 10,
		ContentType: "application/json",
		EncodeMode:  "json",
	}

	pm.profiles["api"] = &MalleableProfile{
		Name:        "api",
		Description: "Shaped like a regular REST API call",
		Method:      "POST",
		URIs:        []string{"/v1/telemetry", "/api/v2/events"},
		UserAgents: []string{
			"Mozilla/5.0 (compatible; MSIE 10.0; Windows NT 6.1; Trident/6.0)",
			"Go-http-client/1.1",
			"python-requests/2.31.0",
		},
		Headers: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
			"X-Request-ID": "{{uuid}}",
		},
		PostData:     `{"events":[{{payload}}],"meta":{"source":"client","version":"1.0"}}`,
		Jitter:       20,
		DefaultSleep: 15,
		ContentType:  "application/json",
		EncodeMode:   "json",
	}

	pm.profiles["sleep"] = &MalleableProfile{
		Name:        "sleep",
		Description: "Minimal traffic for long beacon intervals",
		Method:      "GET",
		URIs:        []string{"/health", "/ping"},
		UserAgents:  []string{"curl/7.88.1", "Wget/1.21.3"},
		Headers:     map[string]string{"Accept": "*/*"},
		Jitter:      50,
		DefaultSleep: 60,
		EncodeMode:  "base64",
	}

	pm.current = "browser"
}

func (pm *ProfileManager) loadFromDisk() {
	if pm.dir == "" {
		return
	}
	_ = os.MkdirAll(pm.dir, 0755)
	entries, err := os.ReadDir(pm.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(pm.dir, entry.Name()))
		if err != nil {
			continue
		}
		var p MalleableProfile
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		if p.Name == "" {
			continue
		}
		pm.profiles[p.Name] = &p
	}
}

// GetDefault returns the profile in use.
func (pm *ProfileManager) GetDefault() *MalleableProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if p, ok := pm.profiles[pm.current]; ok {
		return p
	}
	return pm.profiles["browser"]
}

// Get returns one profile by name.
func (pm *ProfileManager) Get(name string) *MalleableProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.profiles[name]
}

// SetCurrent changes the profile in use.
func (pm *ProfileManager) SetCurrent(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, ok := pm.profiles[name]; ok {
		pm.current = name
	}
}

// List returns every profile, sorted by name.
func (pm *ProfileManager) List() []*MalleableProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]*MalleableProfile, 0, len(pm.profiles))
	for _, p := range pm.profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Save stores a profile, on disk when a directory is configured.
func (pm *ProfileManager) Save(p *MalleableProfile) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.profiles[p.Name] = p
	if pm.dir == "" {
		return nil
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(pm.dir, p.Name+".json"), data, 0644)
}

// AgentProfileFor projects a profile for the agents.
func (pm *ProfileManager) AgentProfileFor(name string) *AgentProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.profiles[name]
	if !ok {
		p = pm.profiles[pm.current]
	}
	if p == nil {
		return nil
	}
	return &AgentProfile{
		Name:        p.Name,
		Method:      p.Method,
		URIs:        p.URIs,
		UserAgents:  p.UserAgents,
		Headers:     p.Headers,
		Cookies:     p.Cookies,
		ContentType: p.ContentType,
		EncodeMode:  p.EncodeMode,
		Prepend:     p.Prepend,
		Append:      p.Append,
	}
}

// Expired reports whether the kill date of a profile has passed.
func (p *MalleableProfile) Expired(now time.Time) bool {
	if p == nil || p.KillDate == "" {
		return false
	}
	kill, err := time.Parse("2006-01-02", p.KillDate)
	if err != nil {
		return false
	}
	return now.After(kill.Add(24 * time.Hour))
}

// GenerateRequest builds the request parameters of a profile.
func (p *MalleableProfile) GenerateRequest(payload string) (method, uri, contentType string, headers map[string]string, body string) {
	method = p.Method
	if method == "" {
		method = "POST"
	}
	if len(p.URIs) > 0 {
		uri = p.URIs[0]
	}
	contentType = p.ContentType
	if contentType == "" {
		contentType = "application/json"
	}
	headers = make(map[string]string, len(p.Headers))
	for k, v := range p.Headers {
		headers[k] = v
	}
	if method == "POST" {
		body = ReplacePlaceholders(p.PostData, payload)
	}
	return
}

// ReplacePlaceholders expands the templates supported by the profiles.
func ReplacePlaceholders(template, payload string) string {
	if template == "" {
		return payload
	}
	out := strings.ReplaceAll(template, "{{payload}}", payload)
	out = strings.ReplaceAll(out, "{{timestamp}}", strconv.FormatInt(time.Now().Unix(), 10))
	out = strings.ReplaceAll(out, "{{nonce}}", randomHex(8))
	out = strings.ReplaceAll(out, "{{uuid}}", randomHex(16))
	out = strings.ReplaceAll(out, "{{random}}", randomHex(16))
	return out
}
