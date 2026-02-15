package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/spf13/cobra"

	"github.com/khan-lab/EGAfetch/internal/api"
	"github.com/khan-lab/EGAfetch/internal/auth"
	"github.com/khan-lab/EGAfetch/internal/download"
	"github.com/khan-lab/EGAfetch/internal/state"
	"github.com/khan-lab/EGAfetch/internal/ui"
	"github.com/khan-lab/EGAfetch/internal/verify"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "egafetch",
		Short: "Fast, parallel, resumable downloads from EGA",
		Long: `EGAfetch is a command-line tool for downloading data and metadata from the
European Genome-phenome Archive (EGA) with parallel chunked downloads,
automatic resume, and checksum verification.`,
		Version: version,
	}

	rootCmd.SetVersionTemplate(fmt.Sprintf("egafetch version %s\n", version))

	rootCmd.AddCommand(
		newAuthCmd(),
		newDownloadCmd(),
		newListCmd(),
		newInfoCmd(),
		newMetadataCmd(),
		newStatusCmd(),
		newVerifyCmd(),
		newCleanCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// signalContext returns a context that is cancelled on SIGINT/SIGTERM.
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigs:
			fmt.Fprintln(os.Stderr, "\nInterrupted. Saving state...")
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// --- Auth commands ---

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage EGA authentication",
	}

	cmd.AddCommand(newAuthLoginCmd(), newAuthStatusCmd(), newAuthLogoutCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to EGA",
		RunE: func(cmd *cobra.Command, args []string) error {
			var username, password string

			if configFile != "" {
				var err error
				username, password, err = loadConfigFile(configFile)
				if err != nil {
					return err
				}
			} else {
				fmt.Print("EGA Username (email): ")
				fmt.Scanln(&username)
				fmt.Print("EGA Password: ")
				passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Println() // newline after hidden input
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
				password = string(passwordBytes)
			}

			if username == "" || password == "" {
				return fmt.Errorf("username and password are required")
			}

			mgr, err := auth.NewManager()
			if err != nil {
				return err
			}

			ctx, cancel := signalContext()
			defer cancel()

			fmt.Println("Authenticating...")
			if err := mgr.Login(ctx, username, password); err != nil {
				return err
			}

			fmt.Println("Login successful!")
			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "cf", "", "JSON config file with credentials ({\"username\":\"...\",\"password\":\"...\"})")
	cmd.Flags().StringVar(&configFile, "config-file", "", "JSON config file with credentials (alias for --cf)")

	return cmd
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := auth.NewManager()
			if err != nil {
				return err
			}

			creds := mgr.Status()
			if creds == nil {
				ui.PrintAuthStatus("", "", false)
				return nil
			}

			expiresIn := time.Until(creds.ExpiresAt).Round(time.Second).String()
			if creds.IsExpired(0) {
				expiresIn = "expired"
			}
			ui.PrintAuthStatus(creds.Username, expiresIn, true)
			return nil
		},
	}
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := auth.NewManager()
			if err != nil {
				return err
			}
			if err := mgr.Logout(); err != nil {
				return err
			}
			fmt.Println("Logged out.")
			return nil
		},
	}
}

// --- Download command ---

func newDownloadCmd() *cobra.Command {
	var output string
	var parallelFiles int
	var parallelChunks int
	var chunkSize string
	var configFile string
	var restart bool
	var format string

	cmd := &cobra.Command{
		Use:   "download [EGAD.../EGAF...]",
		Short: "Download datasets or files from EGA",
		Long: `Download datasets or files from EGA. Re-running the same command
automatically resumes incomplete downloads. Use --restart to force a
fresh download from scratch.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chunkBytes, err := parseSize(chunkSize)
			if err != nil {
				return fmt.Errorf("invalid chunk-size: %w", err)
			}

			opts := download.DownloadOptions{
				ParallelFiles:  parallelFiles,
				ParallelChunks: parallelChunks,
				ChunkSize:      chunkBytes,
			}

			mgr, err := auth.NewManager()
			if err != nil {
				return err
			}

			ctx, cancel := signalContext()
			defer cancel()

			if err := ensureAuth(ctx, mgr, configFile); err != nil {
				return err
			}

			apiClient := api.NewClient(mgr)
			sm := state.NewStateManager(output)

			// If --restart is set, wipe all existing state for a fresh download.
			if restart {
				fmt.Println("Restarting: clearing previous download state...")
				if err := sm.Reset(); err != nil {
					return fmt.Errorf("reset state: %w", err)
				}
			}

			// Resolve args into a manifest.
			manifest, err := resolveManifest(ctx, apiClient, args, format)
			if err != nil {
				return err
			}

			fmt.Printf("Downloading %d file(s) to %s\n", len(manifest.Files), output)

			// Set up progress tracking.
			tracker := ui.NewProgressTracker()
			for _, f := range manifest.Files {
				tracker.RegisterFile(f.FileID, f.FileName, f.Size)
			}

			orch := download.NewOrchestrator(apiClient, sm, opts)
			orch.SetProgressCallback(func(fileID string, bytesDownloaded, totalBytes int64) {
				tracker.UpdateProgress(fileID, bytesDownloaded, totalBytes)
			})
			orch.SetFileCallbacks(
				func(fileID, fileName string) { tracker.FileStarted(fileID, fileName) },
				func(fileID, fileName string, err error) {
					if err != nil {
						tracker.FileFailed(fileID, fileName, err)
					} else {
						tracker.FileCompleted(fileID, fileName)
					}
				},
				func(fileID, fileName string) { tracker.FileSkipped(fileID, fileName) },
			)

			if err := orch.Download(ctx, manifest); err != nil {
				tracker.Stop()
				return err
			}
			tracker.Stop()

			fmt.Println("\nDownload complete!")
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", ".", "Output directory")
	cmd.Flags().IntVar(&parallelFiles, "parallel-files", 4, "Number of files to download in parallel")
	cmd.Flags().IntVar(&parallelChunks, "parallel-chunks", 8, "Number of chunks per file to download in parallel")
	cmd.Flags().StringVar(&chunkSize, "chunk-size", "64M", "Size of each chunk (e.g., 64M, 128M)")
	cmd.Flags().BoolVar(&restart, "restart", false, "Force fresh download, removing any existing progress")
	cmd.Flags().StringVarP(&format, "format", "f", "", "Download only files of this type (e.g., BAM, CRAM, VCF, BCF)")
	cmd.Flags().StringVar(&configFile, "cf", "", "JSON config file with credentials")
	cmd.Flags().StringVar(&configFile, "config-file", "", "JSON config file with credentials (alias for --cf)")

	return cmd
}

// resolveManifest takes CLI args (dataset IDs or file IDs) and builds a manifest.
func resolveManifest(ctx context.Context, apiClient *api.Client, args []string, format string) (*state.Manifest, error) {
	manifest := &state.Manifest{
		CreatedAt: time.Now(),
	}

	for _, arg := range args {
		if strings.HasPrefix(arg, "EGAD") {
			// Dataset ID — fetch file list.
			manifest.DatasetID = arg
			fmt.Printf("Fetching file list for dataset %s...\n", arg)
			files, err := apiClient.ListDatasetFiles(ctx, arg)
			if err != nil {
				return nil, fmt.Errorf("list dataset %s: %w", arg, err)
			}
			for i := range files {
				checksum, checksumType := files[i].GetChecksum()
				manifest.Files = append(manifest.Files, state.FileSpec{
					FileID:       files[i].FileID,
					FileName:     files[i].FileName,
					Size:         files[i].FileSize - 16, // IV stripped in plain mode
					Checksum:     checksum,
					ChecksumType: checksumType,
				})
			}
		} else if strings.HasPrefix(arg, "EGAF") {
			// Individual file ID — fetch metadata.
			fmt.Printf("Fetching metadata for %s...\n", arg)
			meta, err := apiClient.GetFileMetadata(ctx, arg)
			if err != nil {
				return nil, fmt.Errorf("get metadata for %s: %w", arg, err)
			}
			checksum, checksumType := meta.GetChecksum()
			manifest.Files = append(manifest.Files, state.FileSpec{
				FileID:       meta.FileID,
				FileName:     meta.FileName,
				Size:         meta.FileSize - 16, // IV stripped in plain mode
				Checksum:     checksum,
				ChecksumType: checksumType,
			})
		} else {
			return nil, fmt.Errorf("unrecognized identifier %q: expected EGAD... or EGAF...", arg)
		}
	}

	if len(manifest.Files) == 0 {
		return nil, fmt.Errorf("no files found for the given identifiers")
	}

	// Filter by file format if --format is specified.
	if format != "" {
		suffix := "." + strings.ToLower(format)
		totalBefore := len(manifest.Files)
		var filtered []state.FileSpec
		for _, f := range manifest.Files {
			if strings.HasSuffix(strings.ToLower(f.FileName), suffix) {
				filtered = append(filtered, f)
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("no files matching format %q found (out of %d total)", strings.ToUpper(format), totalBefore)
		}
		fmt.Printf("Filtered to %d of %d files matching format %q\n", len(filtered), totalBefore, strings.ToUpper(format))
		manifest.Files = filtered
	}

	return manifest, nil
}

// --- List command ---

func newListCmd() *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "list [EGAD...]",
		Short: "List authorized datasets, or files in a dataset",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := auth.NewManager()
			if err != nil {
				return err
			}

			ctx, cancel := signalContext()
			defer cancel()

			if err := ensureAuth(ctx, mgr, configFile); err != nil {
				return err
			}

			apiClient := api.NewClient(mgr)

			if len(args) == 0 {
				// No dataset ID — list all authorized datasets.
				fmt.Println("Fetching authorized datasets...")
				datasets, err := apiClient.ListDatasets(ctx)
				if err != nil {
					return err
				}
				ids := make([]string, len(datasets))
				for i, d := range datasets {
					ids[i] = d.DatasetID
				}
				ui.PrintDatasets(ids)
				return nil
			}

			// Dataset ID provided — list files in that dataset.
			datasetID := args[0]
			if !strings.HasPrefix(datasetID, "EGAD") {
				return fmt.Errorf("expected dataset ID (EGAD...)")
			}

			fmt.Printf("Fetching files for dataset %s...\n", datasetID)
			files, err := apiClient.ListDatasetFiles(ctx, datasetID)
			if err != nil {
				return err
			}

			var displayFiles []ui.FileInfo
			for i := range files {
				checksum, checksumType := files[i].GetChecksum()
				displayFiles = append(displayFiles, ui.FileInfo{
					FileID:       files[i].FileID,
					FileName:     files[i].FileName,
					FileSize:     files[i].FileSize,
					Checksum:     checksum,
					ChecksumType: checksumType,
				})
			}
			ui.PrintDatasetFiles(displayFiles)
			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "cf", "", "JSON config file with credentials")
	cmd.Flags().StringVar(&configFile, "config-file", "", "JSON config file with credentials (alias for --cf)")

	return cmd
}

// --- Info command ---

func newInfoCmd() *cobra.Command {
	var configFile string

	cmd := &cobra.Command{
		Use:   "info EGAF...",
		Short: "Show file metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fileID := args[0]
			if !strings.HasPrefix(fileID, "EGAF") {
				return fmt.Errorf("expected file ID (EGAF...)")
			}

			mgr, err := auth.NewManager()
			if err != nil {
				return err
			}

			ctx, cancel := signalContext()
			defer cancel()

			if err := ensureAuth(ctx, mgr, configFile); err != nil {
				return err
			}

			apiClient := api.NewClient(mgr)

			meta, err := apiClient.GetFileMetadata(ctx, fileID)
			if err != nil {
				return err
			}

			checksum, checksumType := meta.GetChecksum()
			fmt.Printf("File ID:       %s\n", meta.FileID)
			fmt.Printf("File Name:     %s\n", meta.FileName)
			fmt.Printf("File Size:     %s (%d bytes)\n", ui.FormatBytes(meta.FileSize), meta.FileSize)
			fmt.Printf("Checksum:      %s\n", checksum)
			fmt.Printf("Checksum Type: %s\n", checksumType)
			fmt.Printf("Status:        %s\n", meta.FileStatus)
			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "cf", "", "JSON config file with credentials")
	cmd.Flags().StringVar(&configFile, "config-file", "", "JSON config file with credentials (alias for --cf)")

	return cmd
}

// --- Metadata command ---

func newMetadataCmd() *cobra.Command {
	var format string
	var output string
	var configFile string

	cmd := &cobra.Command{
		Use:   "metadata EGAD...",
		Short: "Download dataset metadata (TSV, CSV, or JSON)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			datasetID := args[0]
			if !strings.HasPrefix(datasetID, "EGAD") {
				return fmt.Errorf("expected dataset ID (EGAD...)")
			}

			switch format {
			case "tsv", "csv", "json":
			default:
				return fmt.Errorf("unsupported format %q (use tsv, csv, or json)", format)
			}

			if output == "" {
				output = datasetID + "-metadata"
			}

			mgr, err := auth.NewManager()
			if err != nil {
				return err
			}

			ctx, cancel := signalContext()
			defer cancel()

			// If config file provided, login to download API and read password for metadata API.
			var metaPassword string
			if configFile != "" {
				username, password, err := loadConfigFile(configFile)
				if err != nil {
					return err
				}
				if err := mgr.Login(ctx, username, password); err != nil {
					return fmt.Errorf("login from config file: %w", err)
				}
				metaPassword = password
			} else {
				if mgr.Username() == "" {
					return fmt.Errorf("not authenticated; run 'egafetch auth login' first")
				}
				// Metadata API uses a separate IdP — need password.
				fmt.Print("EGA Password: ")
				passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
				metaPassword = string(passwordBytes)
			}

			fmt.Printf("Authenticating with metadata API...\n")
			metaToken, err := mgr.GetMetadataToken(ctx, metaPassword)
			if err != nil {
				return err
			}

			apiClient := api.NewClient(mgr)

			fmt.Printf("Fetching metadata for %s...\n", datasetID)
			meta, err := apiClient.FetchDatasetMappings(ctx, metaToken, datasetID)
			if err != nil {
				return err
			}

			// Create output directory.
			if err := os.MkdirAll(output, 0755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}

			// Write individual mapping files.
			mappings := []struct {
				name    string
				records []map[string]interface{}
			}{
				{"study_experiment_run_sample", meta.StudyExperimentRunSample},
				{"run_sample", meta.RunSample},
				{"study_analysis_sample", meta.StudyAnalysisSample},
				{"analysis_sample", meta.AnalysisSample},
				{"sample_file", meta.SampleFile},
			}

			for _, m := range mappings {
				ext := format
				if ext == "tsv" {
					ext = "tsv"
				}
				fileName := m.name + "." + ext
				outPath := filepath.Join(output, fileName)

				if err := writeRecords(outPath, format, m.records); err != nil {
					return fmt.Errorf("write %s: %w", fileName, err)
				}
				fmt.Printf("  %s (%d records)\n", fileName, len(m.records))
			}

			// Generate merged metadata file by merging
			// study_experiment_run_sample + sample_file on sample_accession_id.
			mergedRecords := buildMergedMetadata(meta)
			mergedName := datasetID + "_merged_metadata." + format
			mergedPath := filepath.Join(output, mergedName)
			if err := writeRecords(mergedPath, format, mergedRecords); err != nil {
				return fmt.Errorf("write merged file: %w", err)
			}
			fmt.Printf("  %s (%d records)\n", mergedName, len(mergedRecords))

			fmt.Printf("\nMetadata saved to %s/\n", output)
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "tsv", "Output format (tsv, csv, json)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output directory (default: {datasetID}-metadata)")
	cmd.Flags().StringVar(&configFile, "cf", "", "JSON config file with credentials")
	cmd.Flags().StringVar(&configFile, "config-file", "", "JSON config file with credentials (alias for --cf)")

	return cmd
}

// writeRecords writes a slice of maps to a file in the given format.
func writeRecords(path, format string, records []map[string]interface{}) error {
	if format == "json" {
		return writeJSON(path, records)
	}
	return writeDelimited(path, format, records)
}

// writeJSON writes records as a JSON array.
func writeJSON(path string, records []map[string]interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(records)
}

// writeDelimited writes records as TSV or CSV.
func writeDelimited(path, format string, records []map[string]interface{}) error {
	if len(records) == 0 {
		// Write empty file.
		return os.WriteFile(path, nil, 0644)
	}

	// Collect all column names in stable order.
	colSet := make(map[string]struct{})
	for _, rec := range records {
		for k := range rec {
			colSet[k] = struct{}{}
		}
	}
	columns := make([]string, 0, len(colSet))
	for k := range colSet {
		columns = append(columns, k)
	}
	sort.Strings(columns)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if format == "tsv" {
		w.Comma = '\t'
	}

	// Header.
	if err := w.Write(columns); err != nil {
		return err
	}

	// Rows.
	row := make([]string, len(columns))
	for _, rec := range records {
		for i, col := range columns {
			row[i] = formatValue(rec[col])
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	w.Flush()
	return w.Error()
}

// formatValue converts an interface{} value to a string for TSV/CSV output.
func formatValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		// For nested objects/arrays, marshal to JSON string.
		b, _ := json.Marshal(val)
		return string(b)
	}
}

// buildMergedMetadata merges study_experiment_run_sample with sample_file
// on sample_accession_id to produce a single wide table.
func buildMergedMetadata(meta *api.DatasetMetadata) []map[string]interface{} {
	// Build a lookup from sample_accession_id → sample_file record.
	sampleFileMap := make(map[string]map[string]interface{})
	for _, rec := range meta.SampleFile {
		key, _ := rec["sample_accession_id"].(string)
		if key != "" {
			sampleFileMap[key] = rec
		}
	}

	// Pick the first non-empty base table. EGA datasets can follow
	// the sequencing path (study→experiment→run→sample) or the
	// analysis path (study→analysis→sample), or both.
	var base []map[string]interface{}
	switch {
	case len(meta.StudyExperimentRunSample) > 0:
		base = meta.StudyExperimentRunSample
	case len(meta.StudyAnalysisSample) > 0:
		base = meta.StudyAnalysisSample
	case len(meta.AnalysisSample) > 0:
		base = meta.AnalysisSample
	case len(meta.SampleFile) > 0:
		return meta.SampleFile // nothing to merge with
	default:
		return nil
	}

	// Merge base with sample_file on sample_accession_id.
	var result []map[string]interface{}
	for _, baseRec := range base {
		merged := make(map[string]interface{})
		for k, v := range baseRec {
			merged[k] = v
		}

		sampleID, _ := baseRec["sample_accession_id"].(string)
		if sf, ok := sampleFileMap[sampleID]; ok {
			for k, v := range sf {
				// Prefix to avoid collisions with base columns.
				if _, exists := merged[k]; exists {
					merged["file_"+k] = v
				} else {
					merged[k] = v
				}
			}
		}

		result = append(result, merged)
	}

	return result
}

// --- Status command ---

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [directory]",
		Short: "Show download progress",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			sm := state.NewStateManager(dir)
			states, err := sm.ListFileStates()
			if err != nil {
				return err
			}

			ui.PrintFileStates(states)
			return nil
		},
	}
}

// --- Verify command ---

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify [directory]",
		Short: "Re-verify checksums of downloaded files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			sm := state.NewStateManager(dir)
			states, err := sm.ListFileStates()
			if err != nil {
				return err
			}

			if len(states) == 0 {
				fmt.Println("No downloads found to verify.")
				return nil
			}

			var passed, failed, skipped int
			for _, fs := range states {
				if fs.Status != state.StatusComplete {
					fmt.Printf("  SKIP  %s (status: %s)\n", fs.FileName, fs.Status)
					skipped++
					continue
				}

				filePath := fmt.Sprintf("%s/%s", dir, fs.FileName)
				if fs.ChecksumExpected == "" {
					fmt.Printf("  SKIP  %s (no checksum)\n", fs.FileName)
					skipped++
					continue
				}

				err := verify.Verify(filePath, fs.ChecksumExpected, fs.ChecksumType)
				if err != nil {
					fmt.Printf("  FAIL  %s: %v\n", fs.FileName, err)
					failed++
				} else {
					fmt.Printf("  OK    %s\n", fs.FileName)
					passed++
				}
			}

			fmt.Printf("\n%d passed, %d failed, %d skipped\n", passed, failed, skipped)
			if failed > 0 {
				return fmt.Errorf("%d file(s) failed verification", failed)
			}
			return nil
		},
	}
}

// --- Clean command ---

func newCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean [directory]",
		Short: "Remove temp files, keep completed downloads",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			sm := state.NewStateManager(dir)

			// Remove all chunk directories.
			chunksDir := sm.ChunksPath()
			if _, err := os.Stat(chunksDir); err == nil {
				fmt.Printf("Removing chunk files from %s...\n", chunksDir)
				if err := os.RemoveAll(chunksDir); err != nil {
					return fmt.Errorf("remove chunks: %w", err)
				}
			}

			// Remove state files for completed downloads.
			states, err := sm.ListFileStates()
			if err != nil {
				return err
			}

			var cleaned int
			for _, fs := range states {
				if fs.Status == state.StatusComplete {
					if err := sm.DeleteFileState(fs.FileID); err != nil {
						fmt.Printf("  Warning: could not remove state for %s: %v\n", fs.FileID, err)
					} else {
						cleaned++
					}
				}
			}

			fmt.Printf("Cleaned %d completed state file(s).\n", cleaned)
			return nil
		},
	}
}

// --- Config file helpers ---

// configFileCredentials represents the JSON config file format (pyEGA3 compatible).
type configFileCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loadConfigFile reads username and password from a JSON config file.
func loadConfigFile(path string) (username, password string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read config file: %w", err)
	}

	var creds configFileCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", "", fmt.Errorf("parse config file: %w", err)
	}

	if creds.Username == "" || creds.Password == "" {
		return "", "", fmt.Errorf("config file must contain non-empty \"username\" and \"password\" fields")
	}

	return creds.Username, creds.Password, nil
}

// ensureAuth ensures the auth manager has a valid session. If configFile is
// provided, it reads credentials from the file and performs a fresh login.
// This is used by commands that accept --cf to transparently refresh auth.
func ensureAuth(ctx context.Context, mgr *auth.Manager, configFile string) error {
	if configFile == "" {
		return nil
	}

	username, password, err := loadConfigFile(configFile)
	if err != nil {
		return err
	}

	if err := mgr.Login(ctx, username, password); err != nil {
		return fmt.Errorf("login from config file: %w", err)
	}

	return nil
}

// --- Helpers ---

// parseSize parses a human-readable size string (e.g., "64M", "1G") to bytes.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size string")
	}

	multiplier := int64(1)
	suffix := s[len(s)-1]

	switch suffix {
	case 'K', 'k':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'M', 'm':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'G', 'g':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}

	var value int64
	if _, err := fmt.Sscanf(s, "%d", &value); err != nil {
		return 0, fmt.Errorf("invalid size %q", s)
	}

	if value <= 0 {
		return 0, fmt.Errorf("size must be positive")
	}

	return value * multiplier, nil
}
