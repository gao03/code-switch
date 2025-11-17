package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type ConfigImportStatus struct {
	ConfigExists         bool   `json:"config_exists"`
	ConfigPath           string `json:"config_path"`
	PendingProviders     bool   `json:"pending_providers"`
	PendingMCP           bool   `json:"pending_mcp"`
	PendingProviderCount int    `json:"pending_provider_count"`
	PendingMCPCount      int    `json:"pending_mcp_count"`
}

type ConfigImportResult struct {
	Status            ConfigImportStatus `json:"status"`
	ImportedProviders int                `json:"imported_providers"`
	ImportedMCP       int                `json:"imported_mcp"`
}

type ImportService struct {
	providerService *ProviderService
	mcpService      *MCPService
}

func NewImportService(ps *ProviderService, ms *MCPService) *ImportService {
	return &ImportService{providerService: ps, mcpService: ms}
}

func (is *ImportService) Start() error { return nil }
func (is *ImportService) Stop() error  { return nil }

func (is *ImportService) GetStatus() (ConfigImportStatus, error) {
	path, err := ccSwitchConfigPath()
	if err != nil {
		return ConfigImportStatus{}, err
	}
	status := ConfigImportStatus{ConfigPath: path}
	cfg, exists, err := loadCcSwitchConfig(path)
	if err != nil {
		return status, err
	}
	status.ConfigExists = exists
	if !exists || cfg == nil {
		return status, nil
	}
	return is.evaluateStatus(cfg, path)
}

func (is *ImportService) ImportAll() (ConfigImportResult, error) {
	path, err := ccSwitchConfigPath()
	if err != nil {
		return ConfigImportResult{}, err
	}
	cfg, exists, err := loadCcSwitchConfig(path)
	if err != nil {
		return ConfigImportResult{}, err
	}
	if !exists || cfg == nil {
		return ConfigImportResult{
			Status: ConfigImportStatus{
				ConfigExists: exists,
				ConfigPath:   path,
			},
		}, nil
	}
	return is.processImport(cfg, path)
}

func (is *ImportService) ImportFromFile(path string) (ConfigImportResult, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return ConfigImportResult{}, fmt.Errorf("config path is required")
	}
	cfg, exists, err := loadCcSwitchConfig(cleanPath)
	if err != nil {
		return ConfigImportResult{}, err
	}
	if !exists || cfg == nil {
		return ConfigImportResult{}, fmt.Errorf("config file not found: %s", cleanPath)
	}
	return is.processImport(cfg, cleanPath)
}

func (is *ImportService) GetStatusForFile(path string) (ConfigImportStatus, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return ConfigImportStatus{}, fmt.Errorf("config path is required")
	}
	cfg, exists, err := loadCcSwitchConfig(cleanPath)
	if err != nil {
		return ConfigImportStatus{}, err
	}
	if !exists || cfg == nil {
		return ConfigImportStatus{}, fmt.Errorf("config file not found: %s", cleanPath)
	}
	return is.evaluateStatus(cfg, cleanPath)
}

func (is *ImportService) processImport(cfg *ccSwitchConfig, path string) (ConfigImportResult, error) {
	result := ConfigImportResult{
		Status: ConfigImportStatus{
			ConfigExists: true,
			ConfigPath:   path,
		},
	}
	pendingProviders, err := is.pendingProviders(cfg)
	if err != nil {
		return result, err
	}
	addedProviders, err := is.importProviders(cfg, pendingProviders)
	if err != nil {
		return result, err
	}
	result.ImportedProviders = addedProviders

	pendingServers, err := is.pendingMCPCandidates(cfg)
	if err != nil {
		return result, err
	}
	addedServers, err := is.importMCPServers(pendingServers)
	if err != nil {
		return result, err
	}
	result.ImportedMCP = addedServers

	status, err := is.evaluateStatus(cfg, path)
	if err != nil {
		return result, err
	}
	result.Status = status
	return result, nil
}

func (is *ImportService) evaluateStatus(cfg *ccSwitchConfig, configPath string) (ConfigImportStatus, error) {
	status := ConfigImportStatus{ConfigExists: true, ConfigPath: configPath}
	pendingProviders, err := is.pendingProviders(cfg)
	if err != nil {
		return status, err
	}
	providerCount := len(pendingProviders["claude"]) + len(pendingProviders["codex"])
	status.PendingProviders = providerCount > 0
	status.PendingProviderCount = providerCount

	pendingServers, err := is.pendingMCPCandidates(cfg)
	if err != nil {
		return status, err
	}
	status.PendingMCPCount = len(pendingServers)
	status.PendingMCP = status.PendingMCPCount > 0
	return status, nil
}

func loadCcSwitchConfig(path string) (*ccSwitchConfig, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(data) == 0 {
		return &ccSwitchConfig{}, true, nil
	}
	var cfg ccSwitchConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, true, err
	}
	return &cfg, true, nil
}

func ccSwitchConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cc-switch", "config.json"), nil
}

type ccSwitchConfig struct {
	Claude ccProviderSection `json:"claude"`
	Codex  ccProviderSection `json:"codex"`
	MCP    ccMCPSection      `json:"mcp"`
}

type ccProviderSection struct {
	Providers map[string]ccProviderEntry `json:"providers"`
}

type ccProviderEntry struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	WebsiteURL string            `json:"websiteUrl"`
	Settings   ccProviderSetting `json:"settingsConfig"`
}

type ccProviderSetting struct {
	Env    map[string]string `json:"env"`
	Auth   map[string]string `json:"auth"`
	Config string            `json:"config"`
}

type ccMCPSection struct {
	Claude ccMCPPlatform `json:"claude"`
	Codex  ccMCPPlatform `json:"codex"`
}

type ccMCPPlatform struct {
	Servers map[string]ccMCPServerEntry `json:"servers"`
}

type ccMCPServerEntry struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Enabled     bool              `json:"enabled"`
	Homepage    string            `json:"homepage"`
	Description string            `json:"description"`
	Server      ccMCPServerConfig `json:"server"`
}

type ccMCPServerConfig struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
}

type providerCandidate struct {
	Name   string
	APIURL string
	APIKey string
	Site   string
	Icon   string
}

func (is *ImportService) pendingProviders(cfg *ccSwitchConfig) (map[string][]providerCandidate, error) {
	result := map[string][]providerCandidate{
		"claude": {},
		"codex":  {},
	}
	claudeExisting, err := is.providerService.LoadProviders("claude")
	if err != nil {
		return nil, err
	}
	codexExisting, err := is.providerService.LoadProviders("codex")
	if err != nil {
		return nil, err
	}
	result["claude"] = diffProviderCandidates("claude", cfg.Claude.Providers, claudeExisting)
	result["codex"] = diffProviderCandidates("codex", cfg.Codex.Providers, codexExisting)
	return result, nil
}

func diffProviderCandidates(kind string, entries map[string]ccProviderEntry, existing []Provider) []providerCandidate {
	if len(entries) == 0 {
		return []providerCandidate{}
	}
	existingURL := make(map[string]struct{})
	existingNames := make(map[string]struct{})
	for _, provider := range existing {
		if url := normalizeURL(provider.APIURL); url != "" {
			existingURL[url] = struct{}{}
		}
		if name := normalizeName(provider.Name); name != "" {
			existingNames[name] = struct{}{}
		}
	}
	seen := make(map[string]struct{})
	candidates := make([]providerCandidate, 0, len(entries))
	for key, entry := range entries {
		candidate, ok := parseProviderEntry(kind, key, entry)
		if !ok {
			continue
		}
		if url := normalizeURL(candidate.APIURL); url != "" {
			if _, exists := existingURL[url]; exists {
				continue
			}
			if _, dup := seen[url]; dup {
				continue
			}
		}
		if name := normalizeName(candidate.Name); name != "" {
			if _, exists := existingNames[name]; exists {
				continue
			}
		}
		dedupKey := normalizeURL(candidate.APIURL)
		if dedupKey == "" {
			dedupKey = normalizeName(candidate.Name)
		}
		if dedupKey != "" {
			seen[dedupKey] = struct{}{}
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return strings.ToLower(candidates[i].Name) < strings.ToLower(candidates[j].Name)
	})
	return candidates
}

func parseProviderEntry(kind, key string, entry ccProviderEntry) (providerCandidate, bool) {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		name = strings.TrimSpace(entry.ID)
	}
	if name == "" {
		name = strings.TrimSpace(key)
	}
	site := strings.TrimSpace(entry.WebsiteURL)
	switch strings.ToLower(kind) {
	case "claude":
		apiURL := strings.TrimSpace(entry.Settings.Env["ANTHROPIC_BASE_URL"])
		apiKey := strings.TrimSpace(entry.Settings.Env["ANTHROPIC_AUTH_TOKEN"])
		if apiURL == "" || apiKey == "" {
			return providerCandidate{}, false
		}
		return providerCandidate{Name: name, APIURL: apiURL, APIKey: apiKey, Site: site}, true
	case "codex":
		apiKey := pickFirstNonEmpty(
			entry.Settings.Auth["OPENAI_API_KEY"],
			entry.Settings.Auth["OPENAI_API_KEY_1"],
			entry.Settings.Auth["OPENAI_API_KEY_V2"],
			entry.Settings.Env["OPENAI_API_KEY"],
		)
		if apiKey == "" {
			return providerCandidate{}, false
		}
		apiURL := resolveCodexAPIURL(entry.Settings.Config)
		if apiURL == "" {
			return providerCandidate{}, false
		}
		return providerCandidate{Name: name, APIURL: apiURL, APIKey: apiKey, Site: site}, true
	default:
		return providerCandidate{}, false
	}
}

type ccImportCodexConfig struct {
	ModelProvider    string                                 `toml:"model_provider"`
	AltModelProvider string                                 `toml:"nmodel_provider"`
	Providers        map[string]ccImportCodexProviderConfig `toml:"model_providers"`
}

type ccImportCodexProviderConfig struct {
	Name    string `toml:"name"`
	BaseURL string `toml:"base_url"`
}

func resolveCodexAPIURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var cfg ccImportCodexConfig
	if err := toml.Unmarshal([]byte(raw), &cfg); err != nil {
		return ""
	}
	providerKey := cfg.ModelProvider
	if providerKey == "" {
		providerKey = cfg.AltModelProvider
	}
	if providerKey != "" {
		if provider, ok := cfg.Providers[providerKey]; ok {
			return strings.TrimSpace(provider.BaseURL)
		}
		lower := strings.ToLower(providerKey)
		for key, provider := range cfg.Providers {
			if strings.ToLower(key) == lower {
				return strings.TrimSpace(provider.BaseURL)
			}
			if strings.ToLower(strings.TrimSpace(provider.Name)) == lower {
				return strings.TrimSpace(provider.BaseURL)
			}
		}
	}
	for _, provider := range cfg.Providers {
		if url := strings.TrimSpace(provider.BaseURL); url != "" {
			return url
		}
	}
	return ""
}

func pickFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeURL(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimRight(trimmed, "/")
	return strings.ToLower(trimmed)
}

func normalizeName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (is *ImportService) importProviders(cfg *ccSwitchConfig, pending map[string][]providerCandidate) (int, error) {
	total := 0
	if candidates := pending["claude"]; len(candidates) > 0 {
		added, err := is.saveProviders("claude", candidates)
		if err != nil {
			return total, err
		}
		total += added
	}
	if candidates := pending["codex"]; len(candidates) > 0 {
		added, err := is.saveProviders("codex", candidates)
		if err != nil {
			return total, err
		}
		total += added
	}
	return total, nil
}

func (is *ImportService) saveProviders(kind string, candidates []providerCandidate) (int, error) {
	existing, err := is.providerService.LoadProviders(kind)
	if err != nil {
		return 0, err
	}
	nextID := nextProviderID(existing)
	merged := make([]Provider, 0, len(existing)+len(candidates))
	merged = append(merged, existing...)
	accent, tint := defaultVisual(kind)
	for _, candidate := range candidates {
		provider := Provider{
			ID:      nextID,
			Name:    candidate.Name,
			APIURL:  candidate.APIURL,
			APIKey:  candidate.APIKey,
			Site:    candidate.Site,
			Icon:    candidate.Icon,
			Tint:    tint,
			Accent:  accent,
			Enabled: true,
		}
		merged = append(merged, provider)
		nextID++
	}
	if err := is.providerService.SaveProviders(kind, merged); err != nil {
		return 0, err
	}
	return len(candidates), nil
}

func nextProviderID(list []Provider) int {
	maxID := 0
	for _, provider := range list {
		if provider.ID > maxID {
			maxID = provider.ID
		}
	}
	return maxID + 1
}

func defaultVisual(kind string) (accent, tint string) {
	switch strings.ToLower(kind) {
	case "codex":
		return "#ec4899", "rgba(236, 72, 153, 0.16)"
	default:
		return "#0a84ff", "rgba(15, 23, 42, 0.12)"
	}
}

func (is *ImportService) pendingMCPCandidates(cfg *ccSwitchConfig) ([]MCPServer, error) {
	existing, err := is.mcpService.ListServers()
	if err != nil {
		return nil, err
	}
	existingNames := make(map[string]struct{}, len(existing))
	for _, server := range existing {
		if name := normalizeName(server.Name); name != "" {
			existingNames[name] = struct{}{}
		}
	}
	candidates := collectMCPServers(cfg)
	result := make([]MCPServer, 0, len(candidates))
	seen := make(map[string]struct{})
	for _, server := range candidates {
		name := normalizeName(server.Name)
		if name == "" {
			continue
		}
		if _, exists := existingNames[name]; exists {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		result = append(result, server)
		seen[name] = struct{}{}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

func (is *ImportService) importMCPServers(candidates []MCPServer) (int, error) {
	if len(candidates) == 0 {
		return 0, nil
	}
	existing, err := is.mcpService.ListServers()
	if err != nil {
		return 0, err
	}
	merged := make([]MCPServer, 0, len(existing)+len(candidates))
	merged = append(merged, existing...)
	merged = append(merged, candidates...)
	if err := is.mcpService.SaveServers(merged); err != nil {
		return 0, err
	}
	return len(candidates), nil
}

func collectMCPServers(cfg *ccSwitchConfig) []MCPServer {
	stores := map[string]*MCPServer{}
	appendMCPEntries(stores, cfg.MCP.Claude.Servers, platClaudeCode)
	appendMCPEntries(stores, cfg.MCP.Codex.Servers, platCodex)
	servers := make([]MCPServer, 0, len(stores))
	for _, server := range stores {
		server.EnabledInClaude = containsPlatform(server.EnablePlatform, platClaudeCode)
		server.EnabledInCodex = containsPlatform(server.EnablePlatform, platCodex)
		servers = append(servers, *server)
	}
	return servers
}

func appendMCPEntries(target map[string]*MCPServer, entries map[string]ccMCPServerEntry, platform string) {
	if len(entries) == 0 {
		return
	}
	for key, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = strings.TrimSpace(entry.ID)
		}
		if name == "" {
			name = strings.TrimSpace(key)
		}
		if name == "" {
			continue
		}
		serverCfg := entry.Server
		serverType := strings.TrimSpace(serverCfg.Type)
		command := strings.TrimSpace(serverCfg.Command)
		url := strings.TrimSpace(serverCfg.URL)
		if serverType == "" {
			if url != "" {
				serverType = "http"
			} else if command != "" {
				serverType = "stdio"
			}
		}
		if serverType == "" {
			continue
		}
		if serverType == "http" && url == "" {
			continue
		}
		if serverType == "stdio" && command == "" {
			continue
		}
		normalizedName := strings.ToLower(name)
		existing := target[normalizedName]
		if existing == nil {
			existing = &MCPServer{
				Name:           name,
				Type:           serverType,
				Command:        command,
				Args:           cloneStringSlice(serverCfg.Args),
				Env:            cloneStringMap(serverCfg.Env),
				URL:            url,
				Website:        strings.TrimSpace(entry.Homepage),
				Tips:           strings.TrimSpace(entry.Description),
				EnablePlatform: []string{},
			}
			target[normalizedName] = existing
		} else {
			if existing.Type == "http" && existing.URL == "" {
				existing.URL = url
			}
			if existing.Type == "stdio" && existing.Command == "" {
				existing.Command = command
			}
			if len(existing.Args) == 0 {
				existing.Args = cloneStringSlice(serverCfg.Args)
			}
			if len(existing.Env) == 0 {
				existing.Env = cloneStringMap(serverCfg.Env)
			}
			if existing.Website == "" {
				existing.Website = strings.TrimSpace(entry.Homepage)
			}
			if existing.Tips == "" {
				existing.Tips = strings.TrimSpace(entry.Description)
			}
		}
		if entry.Enabled {
			if !containsPlatform(existing.EnablePlatform, platform) {
				existing.EnablePlatform = append(existing.EnablePlatform, platform)
			}
		}
	}
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func containsPlatform(list []string, platform string) bool {
	platform = strings.TrimSpace(platform)
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), platform) {
			return true
		}
	}
	return false
}
