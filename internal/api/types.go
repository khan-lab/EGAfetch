package api

import "fmt"

// FileMetadata represents a file as returned by the EGA metadata API.
type FileMetadata struct {
	FileID              string `json:"fileId"`
	FileName            string `json:"fileName"`
	FileSize            int64  `json:"fileSize"`
	Checksum            string `json:"checksum"`
	PlainChecksum       string `json:"plainChecksum"`
	UnencryptedChecksum string `json:"unencryptedChecksum"`
	ChecksumType        string `json:"checksumType"`
	FileStatus          string `json:"fileStatus"`
}

// GetChecksum returns the best available checksum value and its inferred type.
// The EGA API may return the checksum under different field names depending on
// the API version (plainChecksum for v2, unencryptedChecksum for v1).
func (f *FileMetadata) GetChecksum() (value string, checksumType string) {
	cs := f.PlainChecksum
	if cs == "" {
		cs = f.UnencryptedChecksum
	}
	if cs == "" {
		cs = f.Checksum
	}
	if cs == "" {
		return "", ""
	}
	if f.ChecksumType != "" {
		return cs, f.ChecksumType
	}
	switch len(cs) {
	case 32:
		return cs, "MD5"
	case 64:
		return cs, "SHA256"
	default:
		return cs, ""
	}
}

// DatasetFile represents a file entry within a dataset listing.
type DatasetFile struct {
	FileID              string `json:"fileId"`
	FileName            string `json:"fileName"`
	FileSize            int64  `json:"fileSize"`
	Checksum            string `json:"checksum"`
	PlainChecksum       string `json:"plainChecksum"`
	UnencryptedChecksum string `json:"unencryptedChecksum"`
	ChecksumType        string `json:"checksumType"`
	FileStatus          string `json:"fileStatus"`
}

// GetChecksum returns the best available checksum value and its inferred type.
func (f *DatasetFile) GetChecksum() (value string, checksumType string) {
	cs := f.PlainChecksum
	if cs == "" {
		cs = f.UnencryptedChecksum
	}
	if cs == "" {
		cs = f.Checksum
	}
	if cs == "" {
		return "", ""
	}
	if f.ChecksumType != "" {
		return cs, f.ChecksumType
	}
	switch len(cs) {
	case 32:
		return cs, "MD5"
	case 64:
		return cs, "SHA256"
	default:
		return cs, ""
	}
}

// DatasetInfo represents a dataset returned by the EGA metadata API.
type DatasetInfo struct {
	DatasetID string `json:"datasetId"`
}

// DatasetMetadata holds all mapping data fetched from the EGA metadata API.
type DatasetMetadata struct {
	StudyExperimentRunSample []map[string]interface{} `json:"study_experiment_run_sample"`
	RunSample                []map[string]interface{} `json:"run_sample"`
	StudyAnalysisSample      []map[string]interface{} `json:"study_analysis_sample"`
	AnalysisSample           []map[string]interface{} `json:"analysis_sample"`
	SampleFile               []map[string]interface{} `json:"sample_file"`
}

// APIError represents an error response from the EGA API.
type APIError struct {
	StatusCode int
	Body       string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("EGA API error (%d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("EGA API error (%d): %s", e.StatusCode, e.Body)
}

// IsRetryable returns true for server errors (5xx) and rate limiting (429).
func (e *APIError) IsRetryable() bool {
	return e.StatusCode == 429 || e.StatusCode >= 500
}
