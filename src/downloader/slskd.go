package downloader

import (
	"bytes"
	"encoding/json"
	"fmt"

	cfg "explo/src/config"
	"explo/src/models"
	"explo/src/util"
)

type Slskd struct {
	DownloadDir  string
	HttpClient   *util.HttpClient
	Cfg          cfg.Slskd
	lastSearchID string // stores the most recent search ID
}

type SearchConfirmation struct {
	ID         string `json:"id"`
	IsComplete bool   `json:"isComplete"`
}

type SearchResult struct {
	Files       []SearchFile `json:"files"`
	QueueLength string       `json:"queueLength"`
	Username    string       `json:"username"`
}

type SearchFile struct {
	Filename string `json:"filename"`
}

func NewSlskd(cfg cfg.Slskd, discovery, downloadDir string, httpClient *util.HttpClient) *Slskd { // init downloader cfg for slskd
	return &Slskd{
		DownloadDir: downloadDir,
		Cfg:         cfg,
		HttpClient:  httpClient}
}

func (c *Slskd) QueryTrack(track *models.Track) error {
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "*/*",
		"X-API-Key":    c.Cfg.APIKey,
	}

	searchText := fmt.Sprintf("%s - %s", track.Title, track.Artist)
	jsonBody := map[string]string{
		"searchText": searchText,
	}
	body, err := json.Marshal(jsonBody)
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	queryURL := fmt.Sprintf("%s://%s/api/v0/searches", c.Cfg.Scheme, c.Cfg.URL)
	resp, err := c.HttpClient.MakeRequest("POST", queryURL, bytes.NewReader(body), headers)
	if err != nil {
		return err
	}

	var search SearchConfirmation
	if err = util.ParseResp(resp, &search); err != nil {
		return fmt.Errorf("failed to parse search confirmation: %w", err)
	}

	c.lastSearchID = search.ID // store the search ID for later
	return nil
}

func (c *Slskd) GetTrack(track *models.Track) error {
	if c.lastSearchID == "" {
		return fmt.Errorf("no search ID stored; call QueryTrack first")
	}

	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "*/*",
		"X-API-Key":    c.Cfg.APIKey,
	}

	queryURL := fmt.Sprintf("%s://%s/api/v0/searches/%s/responses", c.Cfg.Scheme, c.Cfg.URL, c.lastSearchID)
	resp, err := c.HttpClient.MakeRequest("GET", queryURL, nil, headers)
	if err != nil {
		return err
	}

	var results []SearchResult
	if err = util.ParseResp(resp, &results); err != nil {
		return fmt.Errorf("failed to parse search results: %w", err)
	}

	if len(results) == 0 || len(results[0].Files) == 0 {
		return fmt.Errorf("no search results found")
	}

	topMatch := results[0]
	filename := topMatch.Files[0].Filename

	downloadBody := map[string]string{
		"filename": filename,
	}
	body, err := json.Marshal(downloadBody)
	if err != nil {
		return fmt.Errorf("failed to encode download JSON: %w", err)
	}

	downloadURL := fmt.Sprintf("%s://%s/api/v0/transfers/downloads/%s", c.Cfg.Scheme, c.Cfg.URL, topMatch.Username)
	_, err = c.HttpClient.MakeRequest("POST", downloadURL, bytes.NewReader(body), headers)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}

	return nil
}
