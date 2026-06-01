package domain

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Scenario struct {
	Name     string    `json:"name"`
	Modified time.Time `json:"modified"`
	Size     int64     `json:"size"`
	Tags     []string  `json:"tags"`
}

func ListScenarios(testsDir string) []Scenario {
	pattern := filepath.Join(testsDir, "tests", "*.spec.ts")
	files, _ := filepath.Glob(pattern)
	scenarios := make([]Scenario, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		scenarios = append(scenarios, Scenario{
			Name:     filepath.Base(f),
			Modified: info.ModTime(),
			Size:     info.Size(),
		})
	}
	sort.Slice(scenarios, func(i, j int) bool {
		return scenarios[i].Modified.After(scenarios[j].Modified)
	})
	return scenarios
}

var tagRe = regexp.MustCompile(`@([a-zA-Z][a-zA-Z0-9]*)`)

func ScanTags(testsDir string) []string {
	pattern := filepath.Join(testsDir, "tests", "*.spec.ts")
	files, _ := filepath.Glob(pattern)
	tagSet := map[string]struct{}{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, m := range tagRe.FindAllSubmatch(data, -1) {
			tagSet[string(m[1])] = struct{}{}
		}
	}
	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

func SanitizeName(name string) string {
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.ReplaceAll(name, "..", "")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "scenario-" + RandomID()
	}
	return name
}
