package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ProgressTracker tracks and renders live download progress for multiple files.
type ProgressTracker struct {
	mu       sync.Mutex
	files    map[string]*fileProgress
	order    []string // insertion order for stable rendering
	rendered int      // number of lines currently rendered on screen
	done     chan struct{}
}

type fileProgress struct {
	fileName string
	total    int64
	current  int64
	status   string // "downloading", "complete", "failed", "skipped", "merging", "verifying"
}

// NewProgressTracker creates a new progress tracker and starts a background
// goroutine that redraws the terminal every 200ms.
func NewProgressTracker() *ProgressTracker {
	pt := &ProgressTracker{
		files: make(map[string]*fileProgress),
		done:  make(chan struct{}),
	}
	go pt.renderLoop()
	return pt
}

// Stop stops the background render loop and prints the final state.
func (pt *ProgressTracker) Stop() {
	close(pt.done)
	// Small sleep to let the final render happen.
	time.Sleep(50 * time.Millisecond)
}

// RegisterFile registers a file for progress tracking.
func (pt *ProgressTracker) RegisterFile(fileID, fileName string, totalBytes int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.files[fileID] = &fileProgress{
		fileName: fileName,
		total:    totalBytes,
		current:  0,
		status:   "waiting",
	}
	pt.order = append(pt.order, fileID)
}

// UpdateProgress updates the download progress for a file.
func (pt *ProgressTracker) UpdateProgress(fileID string, bytesDownloaded, totalBytes int64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if fp, ok := pt.files[fileID]; ok {
		fp.current = bytesDownloaded
		fp.total = totalBytes
		if fp.status == "waiting" {
			fp.status = "downloading"
		}
	}
}

// FileStarted marks a file as actively downloading.
func (pt *ProgressTracker) FileStarted(fileID, fileName string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if fp, ok := pt.files[fileID]; ok {
		fp.status = "downloading"
	}
}

// FileCompleted marks a file as complete.
func (pt *ProgressTracker) FileCompleted(fileID, fileName string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if fp, ok := pt.files[fileID]; ok {
		fp.status = "complete"
		fp.current = fp.total
	}
}

// FileFailed marks a file as failed.
func (pt *ProgressTracker) FileFailed(fileID, fileName string, err error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if fp, ok := pt.files[fileID]; ok {
		fp.status = "failed"
	}
}

// FileSkipped marks a file as already complete.
func (pt *ProgressTracker) FileSkipped(fileID, fileName string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if fp, ok := pt.files[fileID]; ok {
		fp.status = "skipped"
		fp.current = fp.total
	}
}

// renderLoop redraws the progress display periodically.
func (pt *ProgressTracker) renderLoop() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-pt.done:
			pt.render()
			return
		case <-ticker.C:
			pt.render()
		}
	}
}

// render draws the current progress state to stderr.
func (pt *ProgressTracker) render() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Move cursor up to overwrite previous output.
	if pt.rendered > 0 {
		fmt.Fprintf(os.Stderr, "\033[%dA", pt.rendered)
	}

	lines := 0
	for _, fileID := range pt.order {
		fp := pt.files[fileID]

		// Truncate filename to fit.
		name := fp.fileName
		if len(name) > 30 {
			name = "..." + name[len(name)-27:]
		}

		var line string
		switch fp.status {
		case "complete":
			line = fmt.Sprintf("  %-30s %s  %s\n",
				name,
				formatBar(fp.total, fp.total, 25),
				FormatBytes(fp.total))
		case "skipped":
			line = fmt.Sprintf("  %-30s [---- skipped ----]  %s\n",
				name,
				FormatBytes(fp.total))
		case "failed":
			line = fmt.Sprintf("  %-30s [---- FAILED  ----]\n", name)
		case "waiting":
			line = fmt.Sprintf("  %-30s [waiting...]\n", name)
		default:
			line = fmt.Sprintf("  %-30s %s  %s / %s\n",
				name,
				formatBar(fp.current, fp.total, 25),
				FormatBytes(fp.current),
				FormatBytes(fp.total))
		}

		// Clear rest of line to handle shrinking text.
		fmt.Fprintf(os.Stderr, "\033[K%s", line)
		lines++
	}

	pt.rendered = lines
}

// formatBar builds a progress bar like [========>         ] 45%
func formatBar(current, total int64, width int) string {
	if total <= 0 {
		return fmt.Sprintf("[%s]   0%%", strings.Repeat(" ", width))
	}

	pct := float64(current) / float64(total)
	if pct > 1.0 {
		pct = 1.0
	}

	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("=", filled)
	if filled < width {
		bar += ">"
		bar += strings.Repeat(" ", width-filled-1)
	}

	return fmt.Sprintf("[%s] %3.0f%%", bar, pct*100)
}

// FormatBytes formats a byte count as a human-readable string.
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatProgressBar returns a text progress bar string (for non-live use).
func FormatProgressBar(current, total int64, width int) string {
	return formatBar(current, total, width)
}
