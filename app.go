package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Rule struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`
	Pattern    string   `json:"pattern"`
	TargetDir  string   `json:"targetDir"`
}

type Config struct {
	ViewMode     string   `json:"viewMode"`
	WatchPaths   []string `json:"watchPaths"`
	ConflictMode string   `json:"conflictMode"`
}

type Schema struct {
	Rules []Rule `json:"rules"`
}

type FileInfo struct {
	Name      string `json:"name"`
	Extension string `json:"extension"`
	Size      int64  `json:"size"`
	FullPath  string `json:"fullPath"`
}

type MoveOperation struct {
	FileName string `json:"fileName"`
	OldPath  string `json:"oldPath"`
	NewPath  string `json:"newPath"`
}

type Transaction struct {
	ID         string          `json:"id"`
	Timestamp  int64           `json:"timestamp"`
	Operations []MoveOperation `json:"operations"`
}

type App struct {
	ctx context.Context
}

func NewApp() *App                         { return &App{} }
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) GetOSInfo() string {
	return fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)
}

func (a *App) GetAppDir() string {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".superd")
	os.MkdirAll(path, 0755)
	return path
}

func (a *App) GetSystemPaths() map[string]string {
	home, _ := os.UserHomeDir()
	return map[string]string{
		"Downloads": filepath.Join(home, "Downloads"),
		"Documents": filepath.Join(home, "Documents"),
		"Pictures":  filepath.Join(home, "Pictures"),
		"Videos":    filepath.Join(home, "Videos"),
	}
}

func (a *App) LoadConfig() Config {
	path := filepath.Join(a.GetAppDir(), "config.json")
	d, err := os.ReadFile(path)
	if err != nil {
		home, _ := os.UserHomeDir()
		return Config{ViewMode: "list", ConflictMode: "rename", WatchPaths: []string{filepath.Join(home, "Downloads")}}
	}
	var c Config
	json.Unmarshal(d, &c)
	return c
}

func (a *App) SaveConfig(c Config) {
	path := filepath.Join(a.GetAppDir(), "config.json")
	d, _ := json.Marshal(c)
	os.WriteFile(path, d, 0644)
}

func (a *App) LoadSchema() Schema {
	path := filepath.Join(a.GetAppDir(), "schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Schema{Rules: []Rule{
			{ID: "1", Name: "Images", Extensions: []string{".jpg", ".png", ".webp"}, TargetDir: "Pictures"},
			{ID: "2", Name: "Docs", Extensions: []string{".pdf", ".docx", ".txt"}, TargetDir: "Documents"},
		}}
	}
	var s Schema
	json.Unmarshal(data, &s)
	return s
}

func (a *App) SaveSchema(s Schema) error {
	path := filepath.Join(a.GetAppDir(), "schema.json")
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func (a *App) ExportRules() (bool, error) {
	schema := a.LoadSchema()
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export Rules",
		DefaultFilename: "superd_rules.json",
	})
	if err != nil || path == "" {
		return false, err
	}
	data, _ := json.MarshalIndent(schema, "", "  ")
	err = os.WriteFile(path, data, 0644)
	return err == nil, err
}

func (a *App) ImportRulesDialog() (Schema, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Import Rules",
	})
	if err != nil || path == "" {
		return Schema{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Schema{}, err
	}
	var s Schema
	err = json.Unmarshal(data, &s)
	return s, err
}

func (a *App) SelectFolder() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "Select Folder"})
}

func (a *App) ScanFolder(targetPath string) ([]FileInfo, error) {
	var files = []FileInfo{}
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return files, nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			info, _ := entry.Info()
			files = append(files, FileInfo{
				Name:      entry.Name(),
				Extension: strings.ToLower(filepath.Ext(entry.Name())),
				Size:      info.Size(),
				FullPath:  filepath.Join(targetPath, entry.Name()),
			})
		}
	}
	return files, nil
}

func (a *App) ScanMultipleFolders(paths []string) ([]FileInfo, error) {
	var allFiles []FileInfo
	for _, p := range paths {
		f, _ := a.ScanFolder(p)
		allFiles = append(allFiles, f...)
	}
	return allFiles, nil
}

func getSpecificityScore(r Rule) int {
	score := 0
	if r.Pattern != "" {
		cleanPattern := strings.ReplaceAll(strings.ReplaceAll(r.Pattern, "*", ""), "?", "")
		score += 1000 + len(cleanPattern)
	}
	if len(r.Extensions) > 0 {
		score += 100
	}
	return score
}

func (a *App) OrganizeFiles(targetPaths []string, schema Schema, conflictAction string) (int, error) {
	var allOps []MoveOperation
	sortedRules := make([]Rule, len(schema.Rules))
	copy(sortedRules, schema.Rules)
	sort.Slice(sortedRules, func(i, j int) bool {
		return getSpecificityScore(sortedRules[i]) > getSpecificityScore(sortedRules[j])
	})

	for _, p := range targetPaths {
		files, _ := os.ReadDir(p)
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			fileName := f.Name()
			ext := strings.ToLower(filepath.Ext(fileName))
			src := filepath.Join(p, fileName)
			destDir := ""
			for _, r := range sortedRules {
				if r.Pattern != "" {
					matched, _ := filepath.Match(r.Pattern, fileName)
					if matched {
						destDir = r.TargetDir
						break
					}
				}
				if destDir == "" && contains(r.Extensions, ext) {
					destDir = r.TargetDir
					break
				}
			}
			if destDir == "" {
				destDir = filepath.Join(p, "!Uncategorized")
			}
			if !filepath.IsAbs(destDir) {
				destDir = filepath.Join(p, destDir)
			}
			os.MkdirAll(destDir, 0755)
			dest := filepath.Join(destDir, fileName)
			if _, err := os.Stat(dest); err == nil {
				if conflictAction == "skip" {
					continue
				}
				if conflictAction == "rename" {
					dest = filepath.Join(destDir, fmt.Sprintf("%d_%s", time.Now().Unix(), fileName))
				}
			}
			if err := os.Rename(src, dest); err != nil {
				in, _ := os.Open(src)
				out, _ := os.Create(dest)
				io.Copy(out, in)
				in.Close()
				out.Close()
				os.Remove(src)
			}
			allOps = append(allOps, MoveOperation{FileName: fileName, OldPath: src, NewPath: dest})
		}
	}
	if len(allOps) > 0 {
		a.SaveTransaction(allOps)
	}
	return len(allOps), nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
