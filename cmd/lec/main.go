package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

//go:embed templates/*
var templateFS embed.FS

var (
	version   = "0.1.0"
	tier      string
	module    string
	project   string
	desc      string
	author    string
	remoteURL string
)

func init() {
	flag.StringVar(&tier, "tier", "l0", "Target tier: l0 (prototype), l1 (microservice), l3 (large-service)")
	flag.StringVar(&module, "module", "", "Go module name (e.g. github.com/you/myproject)")
	flag.StringVar(&project, "project", "", "Project directory name")
	flag.StringVar(&desc, "desc", "", "Project description")
	flag.StringVar(&author, "author", "", "Author name")
	flag.StringVar(&remoteURL, "remote", "", "Git remote URL (optional)")
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n  lec — Low-Entropy Core project scaffolder\n")
		fmt.Fprintf(os.Stderr, "  Usage: lec init [options] <project-name>\n\n")
		fmt.Fprintf(os.Stderr, "  Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n  Tiers:\n")
		fmt.Fprintf(os.Stderr, "    l0  Prototype      (<10 files, scripts, PoC)\n")
		fmt.Fprintf(os.Stderr, "    l1  Microservice    (10-100 files, single service)\n")
		fmt.Fprintf(os.Stderr, "    l3  Large Service   (100+ files, distributed)\n\n")
		fmt.Fprintf(os.Stderr, "  Examples:\n")
		fmt.Fprintf(os.Stderr, "    lec init myproject\n")
		fmt.Fprintf(os.Stderr, "    lec init -tier l1 -module github.com/you/myproject myproject\n")
		fmt.Fprintf(os.Stderr, "    lec init -tier l3 -module github.com/org/service -remote git@github.com:org/service.git myservice\n\n")
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 || args[0] != "init" {
		flag.Usage()
		os.Exit(1)
	}
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "  Error: project name is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	name := args[1]
	if project == "" {
		project = name
	}
	if module == "" {
		module = name
	}
	if desc == "" {
		desc = "A Low-Entropy Core " + tierLabel(tier) + " project"
	}
	if author == "" {
		author = "Low-Entropy Developer"
	}

	dir := filepath.Join(".", project)
	if _, err := os.Stat(dir); err == nil {
		fmt.Fprintf(os.Stderr, "  Error: directory '%s' already exists\n", dir)
		os.Exit(1)
	}

	data := TemplateData{
		Project:    project,
		Module:     module,
		Tier:       tier,
		TierLabel:  tierLabel(tier),
		TierNum:    tierNum(tier),
		Desc:       desc,
		Author:     author,
		RemoteURL:  remoteURL,
		Version:    version,
		Year:       time.Now().Year(),
		CoreModule: coreModuleRef(tier),
		CoreTag:    coreBuildTag(tier),
	}

	fmt.Printf("\n  ⚡ Low-Entropy Core Scaffolder v%s\n\n", version)
	fmt.Printf("  Tier:      %s (%s)\n", tier, data.TierLabel)
	fmt.Printf("  Project:   %s\n", project)
	fmt.Printf("  Module:    %s\n", module)
	fmt.Printf("  Directory: %s\n\n", dir)

	// Collect template files for the selected tier
	files, err := collectTemplateFiles(tier)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error collecting templates: %v\n", err)
		os.Exit(1)
	}

	// Process and write files
	for _, tplFile := range files {
		outputPath := filepath.Join(dir, tplFile.outputPath)

		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "  Error creating directory: %v\n", err)
			os.Exit(1)
		}

		tmpl, err := template.New(tplFile.name).Parse(tplFile.content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error parsing template %s: %v\n", tplFile.name, err)
			os.Exit(1)
		}

		f, err := os.Create(outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error creating file %s: %v\n", outputPath, err)
			os.Exit(1)
		}
		defer f.Close()

		if err := tmpl.Execute(f, data); err != nil {
			fmt.Fprintf(os.Stderr, "  Error executing template %s: %v\n", tplFile.name, err)
			os.Exit(1)
		}

		fmt.Printf("  ✓ %s\n", tplFile.outputPath)
	}

	// Initialize git repo
	fmt.Printf("\n  Initializing git repository...\n")
	runCmd(dir, "git", "init")
	runCmd(dir, "git", "add", ".")
	runCmd(dir, "git", "commit", "-m", "init: Low-Entropy Core "+tierLabel(tier)+" project")

	if remoteURL != "" {
		runCmd(dir, "git", "remote", "add", "origin", remoteURL)
		fmt.Printf("  ✓ Remote: %s\n", remoteURL)
	}

	fmt.Printf("\n  🎉 Project '%s' created successfully!\n\n", project)
	fmt.Printf("  Quick start:\n")
	fmt.Printf("    cd %s\n", project)
	if tier == "l0" {
		fmt.Printf("    go run main.go\n")
	} else {
		fmt.Printf("    go build -tags %s -o server .\n", data.CoreTag)
		fmt.Printf("    ./server\n")
	}
	fmt.Printf("\n")
}

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
	name        string
	outputPath  string
	content     string
	isTemplate  bool
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

func runCmd(dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Run()
}
