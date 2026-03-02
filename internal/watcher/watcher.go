package watcher

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

var supportedExts = map[string]bool{
	".py": true,
	".go": true,
	".rs": true,
}

// Watcher monitors a directory tree for changes to supported source files,
// emitting debounced batches of changed paths.
type Watcher struct {
	fsw      *fsnotify.Watcher
	events   chan []string
	done     chan struct{}
	debounce time.Duration
}

// New creates a Watcher that recursively watches directories under root,
// skipping hidden dirs and .treelines.
func New(root string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".treelines" {
				return filepath.SkipDir
			}
			if name != "." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return fsw.Add(path)
		}
		return nil
	})
	if err != nil {
		_ = fsw.Close()
		return nil, err
	}

	w := &Watcher{
		fsw:      fsw,
		events:   make(chan []string, 16),
		done:     make(chan struct{}),
		debounce: 500 * time.Millisecond,
	}
	go w.loop()
	return w, nil
}

// Events returns a channel that emits deduplicated batches of changed file paths.
func (w *Watcher) Events() <-chan []string {
	return w.events
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	err := w.fsw.Close()
	<-w.done
	return err
}

func (w *Watcher) loop() {
	defer close(w.done)
	defer close(w.events)

	var timer *time.Timer
	pending := make(map[string]struct{})

	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				if len(pending) > 0 {
					w.flush(pending)
				}
				return
			}
			if !isSupportedFile(ev.Name) {
				continue
			}
			pending[ev.Name] = struct{}{}
			if timer == nil {
				timer = time.NewTimer(w.debounce)
			} else {
				timer.Reset(w.debounce)
			}

		case <-timerChan(timer):
			timer = nil
			if len(pending) > 0 {
				w.flush(pending)
				pending = make(map[string]struct{})
			}

		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) flush(pending map[string]struct{}) {
	batch := make([]string, 0, len(pending))
	for p := range pending {
		batch = append(batch, p)
	}
	w.events <- batch
}

func timerChan(t *time.Timer) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

func isSupportedFile(path string) bool {
	ext := filepath.Ext(path)
	return supportedExts[ext]
}
