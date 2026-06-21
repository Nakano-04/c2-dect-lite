package profiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type MalleableProfile struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Method      string            `json:"method"` // GET, POST
	URIs        []string          `json:"uris"`
	UserAgents  []string          `json:"user_agents"`
	Headers     map[string]string `json:"headers"`
	Cookies     map[string]string `json:"cookies"`
	PostData    string            `json:"post_data"` // template
	Prepend     string            `json:"prepend"`
	Append      string            `json:"append"`
	Jitter      int               `json:"jitter"` // percentage
	DefaultSleep int              `json:"default_sleep"`
	ContentType string            `json:"content_type"`
	EncodeMode  string            `json:"encode_mode"` // base64, json
}

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
		Description: "Mimics normal browser traffic",
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
			"Accept-Encoding": "gzip, deflate, br",
			"Connection":      "keep-alive",
			"Referer":         "https://app.example.com/dashboard",
		},
		Cookies: map[string]string{
			"session": "{{random}}",
		},
		PostData:    `{"data":"{{payload}}","timestamp":{{timestamp}},"nonce":"{{nonce}}"}`,
		Prepend:     "",
		Append:      "",
		Jitter:      30,
		DefaultSleep: 10,
		ContentType: "application/json",
		EncodeMode:  "json",
	}

	pm.profiles["api"] = &MalleableProfile{
		Name:        "api",
		Description: "Mimics REST API traffic",
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
		PostData:    `{"events":[{{payload}}],"meta":{"source":"client","version":"1.0"}}`,
		Jitter:      20,
		DefaultSleep: 15,
		ContentType: "application/json",
		EncodeMode:  "json",
	}

	pm.profiles["sleep"] = &MalleableProfile{
		Name:        "sleep",
		Description: "Minimal traffic for long sleep intervals",
		Method:      "GET",
		URIs:        []string{"/health", "/ping"},
		UserAgents: []string{
			"curl/7.88.1",
			"Wget/1.21.3",
		},
		Headers: map[string]string{
			"Accept": "*/*",
		},
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
	os.MkdirAll(pm.dir, 0755)
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
		pm.profiles[p.Name] = &p
	}
}

func (pm *ProfileManager) GetDefault() *MalleableProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if p, ok := pm.profiles[pm.current]; ok {
		return p
	}
	return pm.profiles["browser"]
}

func (pm *ProfileManager) Get(name string) *MalleableProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.profiles[name]
}

func (pm *ProfileManager) SetCurrent(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.current = name
}

func (pm *ProfileManager) List() []*MalleableProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var result []*MalleableProfile
	for _, p := range pm.profiles {
		result = append(result, p)
	}
	return result
}

func (pm *ProfileManager) Save(p *MalleableProfile) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.profiles[p.Name] = p

	if pm.dir != "" {
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(pm.dir, p.Name+".json"), data, 0644)
	}
	return nil
}

// GenerateRequest builds HTTP request parameters from the profile
func (p *MalleableProfile) GenerateRequest(payload string) (method, uri, contentType string, headers map[string]string, body string) {
	method = p.Method
	if len(p.URIs) > 0 {
		uri = p.URIs[0] // could randomize
	}
	contentType = p.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	headers = make(map[string]string)
	for k, v := range p.Headers {
		headers[k] = v
	}

	if p.Method == "POST" {
		body = p.PostData
		// Replace placeholders
		body = replacePlaceholders(body, payload)
	}

	return
}

func replacePlaceholders(template, payload string) string {
	result := template
	// Simple placeholder replacement
	result = replaceAll(result, "{{payload}}", payload)
	result = replaceAll(result, "{{timestamp}}", "0") // will be filled at runtime
	result = replaceAll(result, "{{nonce}}", "随机nonce")
	result = replaceAll(result, "{{uuid}}", "随机uuid")
	return result
}

func replaceAll(s, old, new string) string {
	for {
		idx := indexOf(s, old)
		if idx == -1 {
			return s
		}
		s = s[:idx] + new + s[idx+len(old):]
	}
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
