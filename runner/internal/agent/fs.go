package agent

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type DirectoryEntry struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	IsDir   bool     `json:"is_dir"`
	Markers []string `json:"markers,omitempty"`
}

type DirectoryListing struct {
	Path    string           `json:"path"`
	Parent  *string          `json:"parent"`
	Entries []DirectoryEntry `json:"entries"`
	Error   *string          `json:"error,omitempty"`
}

type ProjectFileEntry struct {
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	IsDir      bool       `json:"is_dir"`
	Size       int64      `json:"size"`
	ModifiedAt *time.Time `json:"modified_at,omitempty"`
	Markers    []string   `json:"markers,omitempty"`
}

type ProjectFileListing struct {
	Root    string             `json:"root"`
	Path    string             `json:"path"`
	Parent  *string            `json:"parent"`
	Entries []ProjectFileEntry `json:"entries"`
	Error   *string            `json:"error,omitempty"`
}

type ProjectFileContent struct {
	Root       string     `json:"root"`
	Path       string     `json:"path"`
	Name       string     `json:"name"`
	Size       int64      `json:"size"`
	ModifiedAt *time.Time `json:"modified_at,omitempty"`
	Content    string     `json:"content"`
	Encoding   string     `json:"encoding"`
	Error      *string    `json:"error,omitempty"`
}

type ProjectFileActionResult struct {
	Root       string     `json:"root"`
	Path       string     `json:"path"`
	TargetPath string     `json:"target_path,omitempty"`
	IsDir      bool       `json:"is_dir,omitempty"`
	Size       int64      `json:"size,omitempty"`
	ModifiedAt *time.Time `json:"modified_at,omitempty"`
	Error      *string    `json:"error,omitempty"`
}

const maxProjectFileReadBytes int64 = 2 * 1024 * 1024

func listDirectories(path string) DirectoryListing {
	path = strings.TrimSpace(path)
	if path == "" {
		return listDirectoryRoots()
	}

	cleaned, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		msg := err.Error()
		return DirectoryListing{Path: path, Error: &msg}
	}

	info, err := os.Stat(cleaned)
	if err != nil {
		msg := err.Error()
		return DirectoryListing{Path: cleaned, Error: &msg}
	}
	if !info.IsDir() {
		msg := "path is not a directory"
		return DirectoryListing{Path: cleaned, Error: &msg}
	}

	entries, err := os.ReadDir(cleaned)
	if err != nil {
		msg := err.Error()
		return DirectoryListing{Path: cleaned, Error: &msg}
	}

	out := DirectoryListing{
		Path:    cleaned,
		Parent:  parentPath(cleaned),
		Entries: make([]DirectoryEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		childPath := filepath.Join(cleaned, name)
		out.Entries = append(out.Entries, DirectoryEntry{
			Name:    name,
			Path:    childPath,
			IsDir:   true,
			Markers: projectMarkers(childPath),
		})
		if len(out.Entries) >= 250 {
			break
		}
	}
	sortDirectoryEntries(out.Entries)
	return out
}

func listProjectFiles(root, path string) ProjectFileListing {
	cleanRoot, target, relPath, err := resolveProjectPath(root, path)
	if err != nil {
		msg := err.Error()
		return ProjectFileListing{Root: root, Path: path, Error: &msg}
	}

	info, err := os.Stat(target)
	if err != nil {
		msg := err.Error()
		return ProjectFileListing{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	if !info.IsDir() {
		msg := "path is not a directory"
		return ProjectFileListing{Root: cleanRoot, Path: relPath, Error: &msg}
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		msg := err.Error()
		return ProjectFileListing{Root: cleanRoot, Path: relPath, Error: &msg}
	}

	out := ProjectFileListing{
		Root:    cleanRoot,
		Path:    relPath,
		Parent:  projectParentPath(relPath),
		Entries: make([]ProjectFileEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := entry.Name()
		childAbs := filepath.Join(target, name)
		childRel, err := filepath.Rel(cleanRoot, childAbs)
		if err != nil {
			continue
		}
		childRel = filepath.ToSlash(childRel)
		item := ProjectFileEntry{
			Name:       name,
			Path:       childRel,
			IsDir:      entry.IsDir(),
			Size:       info.Size(),
			ModifiedAt: ptrTime(info.ModTime()),
		}
		if entry.IsDir() {
			item.Markers = projectMarkers(childAbs)
		}
		out.Entries = append(out.Entries, item)
		if len(out.Entries) >= 500 {
			break
		}
	}
	sortProjectFileEntries(out.Entries)
	return out
}

func readProjectFile(root, path string, maxBytes int64) ProjectFileContent {
	if maxBytes <= 0 || maxBytes > maxProjectFileReadBytes {
		maxBytes = maxProjectFileReadBytes
	}
	cleanRoot, target, relPath, err := resolveProjectPath(root, path)
	if err != nil {
		msg := err.Error()
		return ProjectFileContent{Root: root, Path: path, Error: &msg}
	}

	info, err := os.Stat(target)
	if err != nil {
		msg := err.Error()
		return ProjectFileContent{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	if info.IsDir() {
		msg := "path is a directory"
		return ProjectFileContent{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	if info.Size() > maxBytes {
		msg := "file is too large to open in the workbench"
		return ProjectFileContent{Root: cleanRoot, Path: relPath, Name: info.Name(), Size: info.Size(), ModifiedAt: ptrTime(info.ModTime()), Error: &msg}
	}

	file, err := os.Open(target)
	if err != nil {
		msg := err.Error()
		return ProjectFileContent{Root: cleanRoot, Path: relPath, Name: info.Name(), Size: info.Size(), ModifiedAt: ptrTime(info.ModTime()), Error: &msg}
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		msg := err.Error()
		return ProjectFileContent{Root: cleanRoot, Path: relPath, Name: info.Name(), Size: info.Size(), ModifiedAt: ptrTime(info.ModTime()), Error: &msg}
	}
	if int64(len(data)) > maxBytes {
		msg := "file is too large to open in the workbench"
		return ProjectFileContent{Root: cleanRoot, Path: relPath, Name: info.Name(), Size: info.Size(), ModifiedAt: ptrTime(info.ModTime()), Error: &msg}
	}

	return ProjectFileContent{
		Root:       cleanRoot,
		Path:       relPath,
		Name:       info.Name(),
		Size:       info.Size(),
		ModifiedAt: ptrTime(info.ModTime()),
		Content:    string(data),
		Encoding:   "utf-8",
	}
}

func writeProjectFile(root, path, content string, createDirs bool) ProjectFileActionResult {
	cleanRoot, target, relPath, err := resolveProjectWritablePath(root, path)
	if err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: root, Path: path, Error: &msg}
	}
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		msg := "path is a directory"
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	if createDirs {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			msg := err.Error()
			return ProjectFileActionResult{Root: cleanRoot, Path: relPath, Error: &msg}
		}
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	return projectFileActionInfo(cleanRoot, target, relPath, "")
}

func createProjectFileEntry(root, path string, isDir bool) ProjectFileActionResult {
	cleanRoot, target, relPath, err := resolveProjectWritablePath(root, path)
	if err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: root, Path: path, IsDir: isDir, Error: &msg}
	}
	if isDir {
		if err := os.MkdirAll(target, 0o755); err != nil {
			msg := err.Error()
			return ProjectFileActionResult{Root: cleanRoot, Path: relPath, IsDir: true, Error: &msg}
		}
		return projectFileActionInfo(cleanRoot, target, relPath, "")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	if err := file.Close(); err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	return projectFileActionInfo(cleanRoot, target, relPath, "")
}

func renameProjectFileEntry(root, path, targetPath string) ProjectFileActionResult {
	cleanRoot, source, relPath, err := resolveProjectPath(root, path)
	if err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: root, Path: path, TargetPath: targetPath, Error: &msg}
	}
	_, target, relTarget, err := resolveProjectWritablePath(cleanRoot, targetPath)
	if err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, TargetPath: targetPath, Error: &msg}
	}
	if err := os.Rename(source, target); err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, TargetPath: relTarget, Error: &msg}
	}
	return projectFileActionInfo(cleanRoot, target, relTarget, relPath)
}

func deleteProjectFileEntry(root, path string) ProjectFileActionResult {
	cleanRoot, target, relPath, err := resolveProjectPath(root, path)
	if err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: root, Path: path, Error: &msg}
	}
	if relPath == "" {
		msg := "cannot delete the project root"
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	info, err := os.Stat(target)
	if err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	if info.IsDir() {
		if err := os.RemoveAll(target); err != nil {
			msg := err.Error()
			return ProjectFileActionResult{Root: cleanRoot, Path: relPath, IsDir: true, Error: &msg}
		}
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, IsDir: true}
	}
	if err := os.Remove(target); err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: cleanRoot, Path: relPath, Error: &msg}
	}
	return ProjectFileActionResult{Root: cleanRoot, Path: relPath, Size: info.Size(), ModifiedAt: ptrTime(info.ModTime())}
}

func projectFileActionInfo(root, target, relPath, previousPath string) ProjectFileActionResult {
	info, err := os.Stat(target)
	if err != nil {
		msg := err.Error()
		return ProjectFileActionResult{Root: root, Path: relPath, TargetPath: previousPath, Error: &msg}
	}
	return ProjectFileActionResult{
		Root:       root,
		Path:       relPath,
		TargetPath: previousPath,
		IsDir:      info.IsDir(),
		Size:       info.Size(),
		ModifiedAt: ptrTime(info.ModTime()),
	}
}

func listDirectoryRoots() DirectoryListing {
	home, _ := os.UserHomeDir()
	wd, _ := os.Getwd()
	entries := make([]DirectoryEntry, 0, 30)
	seen := map[string]bool{}

	add := func(name, path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		cleaned, err := filepath.Abs(filepath.Clean(path))
		if err != nil {
			return
		}
		info, err := os.Stat(cleaned)
		if err != nil || !info.IsDir() || seen[strings.ToLower(cleaned)] {
			return
		}
		seen[strings.ToLower(cleaned)] = true
		entries = append(entries, DirectoryEntry{Name: name, Path: cleaned, IsDir: true, Markers: projectMarkers(cleaned)})
	}

	add("Home", home)
	add("Current runner directory", wd)
	if runtime.GOOS == "windows" {
		for drive := 'A'; drive <= 'Z'; drive++ {
			root := string(drive) + `:\`
			add(root, root)
		}
	} else {
		add("/", "/")
	}
	sortDirectoryEntries(entries)
	return DirectoryListing{Path: "", Parent: nil, Entries: entries}
}

func parentPath(path string) *string {
	parent := filepath.Dir(path)
	if parent == "." || parent == path {
		return nil
	}
	if runtime.GOOS == "windows" && filepath.VolumeName(path) == path {
		return nil
	}
	return &parent
}

func projectMarkers(path string) []string {
	candidates := []string{".git", "AGENTS.md", "go.mod", "package.json", "pyproject.toml", "Cargo.toml"}
	markers := make([]string, 0, 3)
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(path, candidate)); err == nil {
			markers = append(markers, candidate)
		}
	}
	return markers
}

func sortDirectoryEntries(entries []DirectoryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		leftScore := len(entries[i].Markers)
		rightScore := len(entries[j].Markers)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

func sortProjectFileEntries(entries []ProjectFileEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		leftScore := len(entries[i].Markers)
		rightScore := len(entries[j].Markers)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

func resolveProjectPath(root, path string) (string, string, string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", "", "", os.ErrInvalid
	}
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", "", "", err
	}
	rootInfo, err := os.Stat(cleanRoot)
	if err != nil {
		return "", "", "", err
	}
	if !rootInfo.IsDir() {
		return "", "", "", os.ErrInvalid
	}
	if evaluatedRoot, err := filepath.EvalSymlinks(cleanRoot); err == nil {
		cleanRoot, err = filepath.Abs(evaluatedRoot)
		if err != nil {
			return "", "", "", err
		}
	}

	path = strings.TrimSpace(path)
	var target string
	if path == "" || path == "." {
		target = cleanRoot
	} else if filepath.IsAbs(path) {
		target = filepath.Clean(path)
	} else {
		target = filepath.Join(cleanRoot, filepath.Clean(filepath.FromSlash(path)))
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", "", "", err
	}
	if evaluatedTarget, err := filepath.EvalSymlinks(target); err == nil {
		target, err = filepath.Abs(evaluatedTarget)
		if err != nil {
			return "", "", "", err
		}
	}
	rel, err := filepath.Rel(cleanRoot, target)
	if err != nil {
		return "", "", "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", "", "", os.ErrPermission
	}
	if rel == "." {
		rel = ""
	}
	return cleanRoot, target, filepath.ToSlash(rel), nil
}

func resolveProjectWritablePath(root, path string) (string, string, string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", "", "", os.ErrInvalid
	}
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", "", "", err
	}
	rootInfo, err := os.Stat(cleanRoot)
	if err != nil {
		return "", "", "", err
	}
	if !rootInfo.IsDir() {
		return "", "", "", os.ErrInvalid
	}
	if evaluatedRoot, err := filepath.EvalSymlinks(cleanRoot); err == nil {
		cleanRoot, err = filepath.Abs(evaluatedRoot)
		if err != nil {
			return "", "", "", err
		}
	}
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return "", "", "", os.ErrInvalid
	}
	var target string
	if filepath.IsAbs(path) {
		target = filepath.Clean(path)
	} else {
		target = filepath.Join(cleanRoot, filepath.Clean(filepath.FromSlash(path)))
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", "", "", err
	}
	parent := filepath.Dir(target)
	if evaluatedParent, err := filepath.EvalSymlinks(parent); err == nil {
		target = filepath.Join(evaluatedParent, filepath.Base(target))
		target, err = filepath.Abs(target)
		if err != nil {
			return "", "", "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", "", err
	}
	rel, err := filepath.Rel(cleanRoot, target)
	if err != nil {
		return "", "", "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", "", "", os.ErrPermission
	}
	if rel == "." {
		return "", "", "", os.ErrInvalid
	}
	return cleanRoot, target, filepath.ToSlash(rel), nil
}

func projectParentPath(path string) *string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" || path == "." {
		return nil
	}
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(path)))
	if parent == "." {
		parent = ""
	}
	return &parent
}

func ptrTime(t time.Time) *time.Time {
	u := t.UTC()
	return &u
}
