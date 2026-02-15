package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/khan-lab/EGAfetch/internal/auth"
)

const (
	// EGA API v2 base URLs (from pyEGA3 default_server_file.json).
	dataBaseURL     = "https://ega.ebi.ac.uk:8443/v2"
	metadataBaseURL = "https://ega.ebi.ac.uk:8443/v2/metadata"
	// EGA private metadata API.
	metadataAPIBaseURL = "https://metadata.ega-archive.org"
)

// Client provides methods to interact with the EGA REST APIs.
type Client struct {
	tokenProvider auth.TokenProvider
	httpClient    *http.Client
}

// NewClient creates an API client that uses the given TokenProvider for auth.
func NewClient(tp auth.TokenProvider) *Client {
	return &Client{
		tokenProvider: tp,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ListDatasets returns all datasets the authenticated user has access to.
func (c *Client) ListDatasets(ctx context.Context) ([]DatasetInfo, error) {
	url := fmt.Sprintf("%s/datasets", metadataBaseURL)

	body, err := c.doAuthenticatedGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("list datasets: %w", err)
	}

	var datasets []DatasetInfo
	if err := json.Unmarshal(body, &datasets); err != nil {
		return nil, fmt.Errorf("parse datasets response: %w", err)
	}
	return datasets, nil
}

// ListDatasetFiles returns all files belonging to the given dataset.
func (c *Client) ListDatasetFiles(ctx context.Context, datasetID string) ([]DatasetFile, error) {
	url := fmt.Sprintf("%s/datasets/%s/files", metadataBaseURL, datasetID)

	body, err := c.doAuthenticatedGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("list dataset files: %w", err)
	}

	var files []DatasetFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("parse dataset files response: %w", err)
	}
	return files, nil
}

// GetFileMetadata returns metadata for a single file.
func (c *Client) GetFileMetadata(ctx context.Context, fileID string) (*FileMetadata, error) {
	url := fmt.Sprintf("%s/files/%s", metadataBaseURL, fileID)

	body, err := c.doAuthenticatedGet(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("get file metadata: %w", err)
	}

	var meta FileMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("parse file metadata response: %w", err)
	}
	return &meta, nil
}

// FileDownloadURL returns the full URL for streaming a file download.
// The caller should use HTTP Range headers to download specific byte ranges.
func (c *Client) FileDownloadURL(fileID string) string {
	return fmt.Sprintf("%s/files/%s?destinationFormat=plain", dataBaseURL, fileID)
}

// FetchDatasetMappings fetches all mapping endpoints from the EGA private
// metadata API and returns the combined result. The token parameter is a
// metadata-specific Bearer token (from the metadata IdP, not the download IdP).
func (c *Client) FetchDatasetMappings(ctx context.Context, token, datasetID string) (*DatasetMetadata, error) {
	mappings := []struct {
		name string
		dest *[]map[string]interface{}
	}{
		{"study_experiment_run_sample", nil},
		{"run_sample", nil},
		{"study_analysis_sample", nil},
		{"analysis_sample", nil},
		{"sample_file", nil},
	}

	result := &DatasetMetadata{}
	mappings[0].dest = &result.StudyExperimentRunSample
	mappings[1].dest = &result.RunSample
	mappings[2].dest = &result.StudyAnalysisSample
	mappings[3].dest = &result.AnalysisSample
	mappings[4].dest = &result.SampleFile

	for _, m := range mappings {
		url := fmt.Sprintf("%s/datasets/%s/mappings/%s", metadataAPIBaseURL, datasetID, m.name)
		data, err := c.doGetWithToken(ctx, token, url)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", m.name, err)
		}

		var records []map[string]interface{}
		if err := json.Unmarshal(data, &records); err != nil {
			return nil, fmt.Errorf("parse %s response: %w", m.name, err)
		}
		*m.dest = records
	}

	return result, nil
}

// GetDatasetDetails fetches rich metadata for a dataset from the EGA public
// metadata API (no authentication required).
func (c *Client) GetDatasetDetails(ctx context.Context, datasetID string) (*DatasetDetails, error) {
	url := fmt.Sprintf("https://metadata.ega-archive.org/datasets/%s", datasetID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch dataset details: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var details DatasetDetails
	if err := json.Unmarshal(body, &details); err != nil {
		return nil, fmt.Errorf("parse dataset details: %w", err)
	}
	return &details, nil
}

// doGetWithToken performs a GET request using an explicit Bearer token
// (for APIs that use a different auth system than the download API).
func (c *Client) doGetWithToken(ctx context.Context, token, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	return body, nil
}

// NewAuthenticatedRequest creates an HTTP request with the Bearer token set.
// This is used by the chunk downloader for streaming downloads with Range headers.
func (c *Client) NewAuthenticatedRequest(ctx context.Context, method, url string) (*http.Request, error) {
	token, err := c.tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req, nil
}

// DoStreamRequest executes an HTTP request and returns the response without
// reading the body. The caller is responsible for closing resp.Body.
// This is used for streaming file downloads.
func (c *Client) DoStreamRequest(req *http.Request) (*http.Response, error) {
	// Use a separate client without the default timeout for streaming downloads,
	// since large chunks may take longer than 60 seconds.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	return resp, nil
}

// doAuthenticatedGet performs a GET request with an Authorization header
// and returns the response body.
func (c *Client) doAuthenticatedGet(ctx context.Context, url string) ([]byte, error) {
	token, err := c.tokenProvider.GetAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	return body, nil
}
