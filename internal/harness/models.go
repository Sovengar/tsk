package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
)

// ModelOptions is the model/thinking picker data for one harness.
type ModelOptions struct {
	Models   []string
	Thinking []string
}

// Options loads the local model registry used by the selected harness. A
// missing or malformed registry returns empty options so Ask AI remains usable
// with configured command templates and custom harnesses.
func Options(name string) ModelOptions {
	home, err := os.UserHomeDir()
	if err != nil {
		return ModelOptions{}
	}
	switch name {
	case "pi":
		return piOptions(filepath.Join(home, ".pi", "agent", "models-store.json"))
	case "codex":
		return codexOptions(filepath.Join(home, ".codex", "models_cache.json"))
	case "opencode":
		return opencodeOptions(filepath.Join(home, ".local", "state", "opencode", "model.json"))
	default:
		return ModelOptions{}
	}
}

func readJSON(path string, dst any) bool {
	data, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(data, dst) == nil
}

func piOptions(path string) ModelOptions {
	var root map[string]struct {
		Models []struct {
			ID               string         `json:"id"`
			Provider         string         `json:"provider"`
			ThinkingLevelMap map[string]any `json:"thinkingLevelMap"`
		} `json:"models"`
	}
	if !readJSON(path, &root) {
		return ModelOptions{}
	}
	var out ModelOptions
	for provider, catalog := range root {
		for _, model := range catalog.Models {
			if model.ID == "" {
				continue
			}
			modelProvider := model.Provider
			if modelProvider == "" {
				modelProvider = provider
			}
			out.Models = append(out.Models, modelProvider+"/"+model.ID)
			for level := range model.ThinkingLevelMap {
				out.Thinking = append(out.Thinking, level)
			}
		}
	}
	return normalizeOptions(out)
}

func codexOptions(path string) ModelOptions {
	var root struct {
		Models []struct {
			Slug                     string `json:"slug"`
			SupportedReasoningLevels []struct {
				Effort string `json:"effort"`
			} `json:"supported_reasoning_levels"`
		} `json:"models"`
	}
	if !readJSON(path, &root) {
		return ModelOptions{}
	}
	var out ModelOptions
	for _, model := range root.Models {
		if model.Slug == "" {
			continue
		}
		out.Models = append(out.Models, model.Slug)
		for _, level := range model.SupportedReasoningLevels {
			out.Thinking = append(out.Thinking, level.Effort)
		}
	}
	return normalizeOptions(out)
}

func opencodeOptions(path string) ModelOptions {
	var root struct {
		Recent []struct {
			ProviderID string `json:"providerID"`
			ModelID    string `json:"modelID"`
		} `json:"recent"`
		Favorite []struct {
			ProviderID string `json:"providerID"`
			ModelID    string `json:"modelID"`
		} `json:"favorite"`
		Variant map[string]string `json:"variant"`
	}
	if !readJSON(path, &root) {
		return ModelOptions{}
	}
	var out ModelOptions
	for _, model := range append(root.Recent, root.Favorite...) {
		if model.ProviderID != "" && model.ModelID != "" {
			out.Models = append(out.Models, model.ProviderID+"/"+model.ModelID)
		}
	}
	for _, variant := range root.Variant {
		if variant != "" {
			out.Thinking = append(out.Thinking, variant)
		}
	}
	return normalizeOptions(out)
}

func normalizeOptions(options ModelOptions) ModelOptions {
	options.Models = uniqueSorted(options.Models)
	options.Thinking = uniqueSorted(options.Thinking)
	return options
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}
