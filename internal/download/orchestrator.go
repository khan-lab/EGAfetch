package download

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/khan-lab/EGAfetch/internal/api"
	"github.com/khan-lab/EGAfetch/internal/state"
)

// Orchestrator coordinates parallel file downloads.
type Orchestrator struct {
	apiClient    *api.Client
	stateManager *state.StateManager
	opts         DownloadOptions
	onProgress   ProgressCallback
	onFileStart  func(fileID, fileName string)
	onFileDone   func(fileID, fileName string, err error)
	onFileSkip   func(fileID, fileName string)
}

// NewOrchestrator creates a download orchestrator.
func NewOrchestrator(
	apiClient *api.Client,
	stateManager *state.StateManager,
	opts DownloadOptions,
) *Orchestrator {
	return &Orchestrator{
		apiClient:    apiClient,
		stateManager: stateManager,
		opts:         opts,
	}
}

// SetProgressCallback sets the progress callback for download updates.
func (o *Orchestrator) SetProgressCallback(cb ProgressCallback) {
	o.onProgress = cb
}

// SetFileCallbacks sets callbacks for file lifecycle events.
func (o *Orchestrator) SetFileCallbacks(
	onStart func(fileID, fileName string),
	onDone func(fileID, fileName string, err error),
	onSkip func(fileID, fileName string),
) {
	o.onFileStart = onStart
	o.onFileDone = onDone
	o.onFileSkip = onSkip
}

// Download downloads all files in the manifest using parallel workers.
func (o *Orchestrator) Download(ctx context.Context, manifest *state.Manifest) error {
	if len(manifest.Files) == 0 {
		return fmt.Errorf("no files to download")
	}

	// Save manifest.
	if err := o.stateManager.SaveManifest(manifest); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}

	g, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, o.opts.ParallelFiles)

	for _, fileSpec := range manifest.Files {
		fileSpec := fileSpec
		g.Go(func() error {
			// Check if already complete BEFORE acquiring the semaphore so
			// finished files don't occupy a download slot and can be marked
			// "skipped" immediately even when the context is cancelled.
			existing, err := o.stateManager.LoadFileState(fileSpec.FileID)
			if err != nil {
				return fmt.Errorf("load state for %s: %w", fileSpec.FileID, err)
			}
			if existing != nil && existing.IsComplete() {
				if o.onFileSkip != nil {
					o.onFileSkip(fileSpec.FileID, fileSpec.FileName)
				}
				return nil
			}

			// Acquire file semaphore.
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return ctx.Err()
			}

			return o.downloadFile(ctx, fileSpec)
		})
	}

	return g.Wait()
}

// downloadFile downloads a single file, checking if it's already complete.
func (o *Orchestrator) downloadFile(ctx context.Context, spec state.FileSpec) error {
	// Check if already complete.
	existing, err := o.stateManager.LoadFileState(spec.FileID)
	if err != nil {
		return fmt.Errorf("load state for %s: %w", spec.FileID, err)
	}
	if existing != nil && existing.IsComplete() {
		if o.onFileSkip != nil {
			o.onFileSkip(spec.FileID, spec.FileName)
		}
		return nil
	}

	if o.onFileStart != nil {
		o.onFileStart(spec.FileID, spec.FileName)
	}

	fd := NewFileDownload(spec, o.apiClient, o.stateManager, o.opts, o.onProgress)
	err = fd.Run(ctx)

	if o.onFileDone != nil {
		o.onFileDone(spec.FileID, spec.FileName, err)
	}

	return err
}
