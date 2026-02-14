package ui

import (
	"fmt"
	"strings"

	"github.com/khan-lab/EGAfetch/internal/state"
)

// PrintFileStates prints a formatted table of file download states.
func PrintFileStates(states []*state.FileState) {
	if len(states) == 0 {
		fmt.Println("No downloads found.")
		return
	}

	fmt.Printf("\n%-20s %-15s %-12s %-10s %s\n",
		"File ID", "Status", "Size", "Progress", "File Name")
	fmt.Println(strings.Repeat("-", 80))

	for _, fs := range states {
		var progress string
		if fs.Status == state.StatusComplete {
			progress = "100%"
		} else if fs.Size > 0 {
			downloaded := int64(0)
			for _, c := range fs.Chunks {
				downloaded += c.BytesDownloaded
			}
			pct := float64(downloaded) / float64(fs.Size) * 100
			progress = fmt.Sprintf("%.1f%%", pct)
		} else {
			progress = "-"
		}

		fmt.Printf("%-20s %-15s %-12s %-10s %s\n",
			truncate(fs.FileID, 20),
			fs.Status,
			FormatBytes(fs.Size),
			progress,
			fs.FileName,
		)

		if fs.Error != "" {
			fmt.Printf("  Error: %s\n", fs.Error)
		}
	}
	fmt.Println()
}

// PrintDatasetFiles prints a formatted table of files in a dataset.
func PrintDatasetFiles(files []FileInfo) {
	if len(files) == 0 {
		fmt.Println("No files found.")
		return
	}

	fmt.Printf("\n%-20s %-12s %-6s %-34s %s\n",
		"File ID", "Size", "Check", "Checksum", "File Name")
	fmt.Println(strings.Repeat("-", 110))

	var totalSize int64
	for _, f := range files {
		fmt.Printf("%-20s %-12s %-6s %-34s %s\n",
			truncate(f.FileID, 20),
			FormatBytes(f.FileSize),
			f.ChecksumType,
			f.Checksum,
			f.FileName,
		)
		totalSize += f.FileSize
	}

	fmt.Printf("\n%d files, %s total\n\n", len(files), FormatBytes(totalSize))
}

// FileInfo holds display information for a file.
type FileInfo struct {
	FileID       string
	FileName     string
	FileSize     int64
	Checksum     string
	ChecksumType string
}

// PrintDatasets prints a list of authorized dataset IDs.
func PrintDatasets(datasets []string) {
	if len(datasets) == 0 {
		fmt.Println("No authorized datasets found.")
		return
	}

	fmt.Printf("\nAuthorized datasets (%d):\n\n", len(datasets))
	for _, id := range datasets {
		fmt.Printf("  %s\n", id)
	}
	fmt.Println()
}

// PrintAuthStatus prints the current authentication status.
func PrintAuthStatus(username string, expiresIn string, loggedIn bool) {
	if !loggedIn {
		fmt.Println("Not logged in. Run 'egafetch auth login' to authenticate.")
		return
	}
	fmt.Printf("Logged in as: %s\n", username)
	fmt.Printf("Token expires: %s\n", expiresIn)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
