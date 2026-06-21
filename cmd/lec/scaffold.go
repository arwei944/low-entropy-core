package main

import (
	"fmt"
	"text/template"
)

// ─── Template data ───

type TemplateData struct {
	Project    string
	Module     string
	Tier       string
	TierLabel  string
	TierNum    int
	Desc       string
	Author     string
	RemoteURL  string
	Version    string
	Year       int
	CoreModule string
	CoreTag    string
}

type templateFile struct {
	name       string
	outputPath string
	content    string
	isTemplate bool
}

func tierLabel(t string) string {
	switch t {
	case "l0":
		return "Prototype"
	case "l1":
		return "Microservice"
	case "l3":
		return "Large Service"
	default:
		return "Prototype"
	}
}

func tierNum(t string) int {
	switch t {
	case "l0":
		return 0
	case "l1":
		return 1
	case "l3":
		return 3
	default:
		return 0
	}
}

func coreModuleRef(t string) string {
	// For now, always use the go-core replace directive
	return "low-entropy-core/go-core"
}

func coreBuildTag(t string) string {
	switch t {
	case "l0":
		return "lecore_tier0"
	case "l1":
		return "lecore_tier1"
	case "l3":
		return "lecore_tier3"
	default:
		return "lecore_tier0"
	}
}

func collectTemplateFiles(tier string) ([]templateFile, error) {
	var files []templateFile

	// Common files for all tiers
	common := []struct {
		name       string
		outputPath string
		tpl        bool
	}{
		{"go.mod", "go.mod", true},
		{"CLAUDE.md", "CLAUDE.md", true},
		{"README.md", "README.md", true},
		{".gitignore", ".gitignore", false},
	}

	for _, f := range common {
		content, err := readTemplate(f.name)
		if err != nil {
			return nil, err
		}
		files = append(files, templateFile{
			name:       f.name,
			outputPath: f.outputPath,
			content:    content,
			isTemplate: f.tpl,
		})
	}

	// Tier-specific main.go
	mainContent, err := readTemplate(tier + "/main.go")
	if err != nil {
		return nil, err
	}
	files = append(files, templateFile{
		name:       tier + "/main.go",
		outputPath: "main.go",
		content:    mainContent,
		isTemplate: true,
	})

	// L1+: add types.go
	if tier == "l1" || tier == "l3" {
		typesContent, err := readTemplate(tier + "/types.go")
		if err != nil {
			return nil, err
		}
		files = append(files, templateFile{
			name:       tier + "/types.go",
			outputPath: "types.go",
			content:    typesContent,
			isTemplate: true,
		})
	}

	// L3: add additional files
	if tier == "l3" {
		extraFiles := []string{"ports.go", "adapters.go", "atoms.go", "routes.go"}
		for _, ef := range extraFiles {
			content, err := readTemplate(tier + "/" + ef)
			if err != nil {
				return nil, err
			}
			files = append(files, templateFile{
				name:       tier + "/" + ef,
				outputPath: ef,
				content:    content,
				isTemplate: true,
			})
		}
	}

	return files, nil
}

func readTemplate(path string) (string, error) {
	data, err := templateFS.ReadFile("templates/" + path)
	if err != nil {
		return "", fmt.Errorf("template not found: templates/%s: %w", path, err)
	}
	return string(data), nil
}
