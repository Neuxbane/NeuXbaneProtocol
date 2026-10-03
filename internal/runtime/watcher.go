package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/Neuxbane/NeuXbaneProtocol/internal/telemetry"
)

// Watcher monitors define/ directory for Go file changes and triggers reload callbacks with debounce.
type Watcher struct {
	watchDir  string
	debounce  time.Duration
	onReload  func(changedFiles []string)
	fsWatcher *fsnotify.Watcher
	mu        sync.Mutex
	closed    bool
}

// NewWatcher constructs an initialized Watcher.
func NewWatcher(watchDir string, debounce time.Duration, onReload func([]string)) (*Watcher, error) {
	if debounce <= 0 {
		debounce = 250 * time.Millisecond
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		watchDir:  watchDir,
		debounce:  debounce,
		onReload:  onReload,
		fsWatcher: fsw,
	}, nil
}

// Start recursively watches watchDir and processes filesystem events.
func (w *Watcher) Start(ctx context.Context) error {
	// If watchDir doesn't exist yet, create or skip
	if _, err := os.Stat(w.watchDir); os.IsNotExist(err) {
		_ = os.MkdirAll(w.watchDir, 0755)
	}

	if err := w.watchRecursive(w.watchDir); err != nil {
		return err
	}

	go w.eventLoop(ctx)
	return nil
}

func (w *Watcher) watchRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return w.fsWatcher.Add(path)
		}
		return nil
	})
}

func (w *Watcher) eventLoop(ctx context.Context) {
	var (
		timer        *time.Timer
		changedFiles = make(map[string]struct{})
		mu           sync.Mutex
	)

	trigger := func() {
		mu.Lock()
		defer mu.Unlock()

		if len(changedFiles) == 0 {
			return
		}

		files := make([]string, 0, len(changedFiles))
		for f := range changedFiles {
			files = append(files, f)
		}
		changedFiles = make(map[string]struct{})

		telemetry.LogService("watcher", "filesystem changes detected under define/ (%d files)", len(files))
		if w.onReload != nil {
			w.onReload(files)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// If new directory created, watch it and track any contained .go files
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = w.watchRecursive(event.Name)
					_ = filepath.Walk(event.Name, func(p string, fi os.FileInfo, err error) error {
						if err == nil && !fi.IsDir() && strings.HasSuffix(p, ".go") {
							mu.Lock()
							changedFiles[p] = struct{}{}
							if timer != nil {
								timer.Stop()
							}
							timer = time.AfterFunc(w.debounce, trigger)
							mu.Unlock()
						}
						return nil
					})
					continue
				}
			}

			// Only trigger on .go files
			if !strings.HasSuffix(event.Name, ".go") {
				continue
			}

			mu.Lock()
			changedFiles[event.Name] = struct{}{}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, trigger)
			mu.Unlock()

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			telemetry.Logger().Error("watcher error", "err", err)
		}
	}
}

// Close terminates the filesystem watcher.
func (w *Watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		w.closed = true
		return w.fsWatcher.Close()
	}
	return nil
}
