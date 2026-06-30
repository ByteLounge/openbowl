package watcher

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/openbowl/openbowl/packages/core/pkg/db"
)

type FileWatcher struct {
	DB        *db.DB
	ProjectID string
	DirPath   string
	Watcher   *fsnotify.Watcher
	StopChan  chan struct{}
}

func NewFileWatcher(database *db.DB, projectID string, dirPath string) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &FileWatcher{
		DB:        database,
		ProjectID: projectID,
		DirPath:   dirPath,
		Watcher:   watcher,
		StopChan:  make(chan struct{}),
	}, nil
}

// Start boots the filesystem monitoring loop
func (fw *FileWatcher) Start() {
	// 1. Run initial directory indexing scan
	log.Printf("[File Watcher] Indexing directory: %s", fw.DirPath)
	if err := fw.syncDirectory(); err != nil {
		log.Printf("[File Watcher] Initial sync error: %v", err)
	}

	// 2. Add directories recursively
	if err := fw.watchRecursive(fw.DirPath); err != nil {
		log.Printf("[File Watcher] Recursive watch error: %v", err)
	}

	// 3. Start monitoring events loop
	go func() {
		for {
			select {
			case event, ok := <-fw.Watcher.Events:
				if !ok {
					return
				}

				// Skip temporary editor files or hidden config folders
				if fw.shouldIgnore(event.Name) {
					continue
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					log.Printf("[File Watcher] File modified: %s", event.Name)
					fw.indexFile(event.Name)
				} else if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					log.Printf("[File Watcher] File removed: %s", event.Name)
					fw.removeFile(event.Name)
				}

			case err, ok := <-fw.Watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[File Watcher] Monitor error: %v", err)

			case <-fw.StopChan:
				fw.Watcher.Close()
				return
			}
		}
	}()
}

func (fw *FileWatcher) Stop() {
	close(fw.StopChan)
}

func (fw *FileWatcher) syncDirectory() error {
	return filepath.Walk(fw.DirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if fw.shouldIgnore(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !fw.shouldIgnore(path) {
			fw.indexFile(path)
		}
		return nil
	})
}

func (fw *FileWatcher) watchRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if fw.shouldIgnore(path) {
				return filepath.SkipDir
			}
			log.Printf("[File Watcher] Sub-directory watch added: %s", path)
			return fw.Watcher.Add(path)
		}
		return nil
	})
}

func (fw *FileWatcher) indexFile(path string) {
	relPath, err := filepath.Rel(fw.DirPath, path)
	if err != nil {
		relPath = path
	}

	hash, err := calculateHash(path)
	if err != nil {
		return
	}

	// Update database reference
	// Native upsert ON CONFLICT in SQLite
	query := `
	INSERT INTO file_references (id, project_id, relative_path, file_hash, indexed_at)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		file_hash = excluded.file_hash,
		indexed_at = excluded.indexed_at
	`
	id := uuid.NewMD5(uuid.NameSpaceDNS, []byte(relPath)).String() // Deterministic ID based on relative path
	_, err = fw.DB.Conn.Exec(query, id, fw.ProjectID, relPath, hash, time.Now())
	if err != nil {
		log.Printf("[File Watcher] Database write failed for %s: %v", relPath, err)
	}
}

func (fw *FileWatcher) removeFile(path string) {
	relPath, err := filepath.Rel(fw.DirPath, path)
	if err != nil {
		relPath = path
	}

	id := uuid.NewMD5(uuid.NameSpaceDNS, []byte(relPath)).String()
	_, err = fw.DB.Conn.Exec(`DELETE FROM file_references WHERE id = ?`, id)
	if err != nil {
		log.Printf("[File Watcher] Database delete failed for %s: %v", relPath, err)
	}
}

func (fw *FileWatcher) shouldIgnore(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return true
	}
	ignoredDirs := []string{
		"node_modules", "dist", "bin", "build", "target", "vendor",
		".git", ".gemini", ".agents", "brain", ".system_generated",
	}
	for _, dir := range ignoredDirs {
		if strings.Contains(path, string(filepath.Separator)+dir+string(filepath.Separator)) ||
			strings.HasSuffix(path, string(filepath.Separator)+dir) ||
			strings.HasPrefix(path, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func calculateHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
