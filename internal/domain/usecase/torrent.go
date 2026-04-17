package usecase

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"time"

	"stream-web-api/internal/domain/model"
	domainrepo "stream-web-api/internal/domain/repository"
)

type TorrentUsecase struct {
	client         domainrepo.TorrentClient
	port           int
	cpRepo         domainrepo.CustomProviderRepository
	scriptExecutor *ScriptExecutorUsecase
}

func NewTorrentUsecase(client domainrepo.TorrentClient, port int) *TorrentUsecase {
	return &TorrentUsecase{
		client: client,
		port:   port,
	}
}

func (s *TorrentUsecase) SetCustomProviderRepo(repo domainrepo.CustomProviderRepository) {
	s.cpRepo = repo
}

func (s *TorrentUsecase) SetScriptExecutor(se *ScriptExecutorUsecase) {
	s.scriptExecutor = se
}

func (s *TorrentUsecase) AddMagnet(magnetURI string) (*model.Torrent, error) {
	infoHash, err := s.client.AddMagnet(magnetURI)
	if err != nil {
		return nil, err
	}
	return s.client.GetStats(infoHash, "", s.port)
}

func (s *TorrentUsecase) GetStats(infoHash string) (*model.Torrent, error) {
	return s.client.GetStats(infoHash, "", s.port)
}

func (s *TorrentUsecase) ListTorrents() []*model.Torrent {
	return s.client.ListTorrents(s.port)
}

func (s *TorrentUsecase) RemoveTorrent(infoHash string) error {
	return s.client.RemoveTorrent(infoHash)
}

func (s *TorrentUsecase) RemoveAllTorrents() error {
	return s.client.RemoveAll()
}

func (s *TorrentUsecase) GetFileReader(infoHash string, fileIndex int, start, end int64) (io.ReadSeeker, error) {
	return s.client.GetFileReader(infoHash, fileIndex, start, end)
}

func (s *TorrentUsecase) WaitForPieces(infoHash string, startPiece, endPiece int, timeoutSeconds float64) error {
	return s.client.WaitForPieces(infoHash, startPiece, endPiece, time.Duration(timeoutSeconds)*time.Second)
}

func (s *TorrentUsecase) GetPieceInfo(infoHash string, fileIndex int) (map[string]interface{}, error) {
	return s.client.GetPieceInfo(infoHash, fileIndex)
}

func (s *TorrentUsecase) GetFileHandle(infoHash string, fileIndex int) (*model.FileHandle, error) {
	return s.client.GetFileHandle(infoHash, fileIndex)
}

func (s *TorrentUsecase) IsTorrentReady(infoHash string) bool {
	return s.client.IsTorrentReady(infoHash)
}

func (s *TorrentUsecase) GetFileDiskPath(infoHash string, fileIndex int) (string, error) {
	return s.client.GetFileDiskPath(infoHash, fileIndex)
}

func (s *TorrentUsecase) NeedsTranscoding(filename string) bool {
	return s.client.NeedsTranscoding(filename)
}

func (s *TorrentUsecase) GetMimeType(filename string) string {
	return s.client.GetMimeType(filename)
}

func (s *TorrentUsecase) GetFileInfo(infoHash string, fileIndex int) (*model.File, error) {
	stats, err := s.client.GetStats(infoHash, "", s.port)
	if err != nil {
		return nil, err
	}
	if fileIndex < 0 || fileIndex >= len(stats.Files) {
		return nil, fmt.Errorf("invalid file index")
	}
	return &stats.Files[fileIndex], nil
}

func (s *TorrentUsecase) EnsureFileHeader(infoHash string, fileIndex int) error {
	return s.client.EnsureFileHeader(infoHash, fileIndex)
}

func (s *TorrentUsecase) StartFileDownload(infoHash string, fileIndex int) error {
	return s.client.StartFileDownload(infoHash, fileIndex)
}

func (s *TorrentUsecase) SeekFileDownload(infoHash string, fileIndex int, timestamp float64, duration float64) error {
	return s.client.SeekFileDownload(infoHash, fileIndex, timestamp, duration)
}

func (s *TorrentUsecase) GetPort() int {
	return s.port
}

func (s *TorrentUsecase) SearchTorrents(provider, query string, page int) ([]*model.SearchResult, error) {
	return s.client.Search(provider, query, page)
}

func (s *TorrentUsecase) SearchWithDefaults(provider, query string, page int) ([]*model.SearchResult, error) {
	if page < 1 {
		page = 1
	}
	return s.SearchTorrents(provider, query, page)
}

func (s *TorrentUsecase) GetHardcodedProviders() []string {
	return s.client.GetProviders()
}

func (s *TorrentUsecase) ListAllProviders() []model.ProviderInfo {
	providers := s.client.GetProviders()
	var result []model.ProviderInfo
	for _, p := range providers {
		if p == "all" {
			result = append(result, model.ProviderInfo{ID: "all", Name: "all", Type: "embedded", PageType: "list"})
		} else {
			result = append(result, model.ProviderInfo{ID: p, Name: p, Type: "embedded", PageType: "list"})
		}
	}

	if s.cpRepo != nil {
		customProviders, err := s.cpRepo.GetAll()
		if err == nil {
			for _, cp := range customProviders {
				result = append(result, model.ProviderInfo{
					ID:       cp.ID,
					Name:     cp.Name,
					Type:     "custom",
					PageType: cp.PageTypeDefault,
				})
			}
		}
	}

	return result
}

func (s *TorrentUsecase) SearchCustom(ctx context.Context, id, query, detailURL string) (interface{}, error) {
	if s.cpRepo == nil || s.scriptExecutor == nil {
		return nil, fmt.Errorf("custom search not configured")
	}

	cp, err := s.cpRepo.GetByID(id)
	if err != nil || cp == nil {
		return nil, fmt.Errorf("custom provider not found")
	}

	var fullURL string
	var pageType string

	if detailURL != "" {
		fullURL = detailURL
		pageType = "detail"
	} else {
		if query == "" {
			return nil, fmt.Errorf("query required for list search")
		}
		fullURL = cp.BaseURL
		fullURL = torrentReplacePlaceholders(fullURL, query)
		pageType = cp.PageTypeDefault
		if pageType == "" {
			pageType = "list"
		}
	}

	code := cp.Code
	decoded, decodeErr := base64.StdEncoding.DecodeString(code)
	if decodeErr == nil {
		code = string(decoded)
	}

	language := cp.Language
	if language == "" {
		language = "javascript"
	}

	result, err := s.scriptExecutor.Execute(ctx, code, fullURL, pageType, language)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func torrentReplacePlaceholders(template, query string) string {
	return torrentReplaceAll(template, "{q}", query,
		"{query}", query,
		"{search}", query,
		"{raw_q}", query,
		"{raw_query}", query,
		"{raw_search}", query)
}

func torrentReplaceAll(s string, pairs ...string) string {
	for i := 0; i < len(pairs); i += 2 {
		if i+1 < len(pairs) {
			s = torrentReplaceString(s, pairs[i], pairs[i+1])
		}
	}
	return s
}

func torrentReplaceString(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}

func (s *TorrentUsecase) SaveMetadata(infoHash, metadataJSON string) error {
	return s.client.SaveMetadata(infoHash, metadataJSON)
}

func (s *TorrentUsecase) AddMagnetWithMetadata(magnetURI string, metadataJSON string) (*model.Torrent, error) {
	torrent, err := s.AddMagnet(magnetURI)
	if err != nil {
		return nil, err
	}

	if metadataJSON != "" {
		if err := s.SaveMetadata(torrent.InfoHash, metadataJSON); err != nil {
			log.Printf("Failed to save metadata for %s: %v", torrent.InfoHash, err)
		}
	}

	return torrent, nil
}

func (s *TorrentUsecase) GetMetadata(infoHash string) (string, error) {
	return s.client.GetMetadata(infoHash)
}

func (s *TorrentUsecase) AcquireStream(parentCtx context.Context, description string) (context.Context, context.CancelFunc) {
	return s.client.AcquireStream(parentCtx, description)
}

func (s *TorrentUsecase) KillActiveStream() bool {
	return s.client.KillActiveStream()
}

func (s *TorrentUsecase) HasActiveStream() bool {
	return s.client.HasActiveStream()
}

func (s *TorrentUsecase) TorrentCount() int {
	return s.client.TorrentCount()
}

func (s *TorrentUsecase) UpdatePlayback(infoHash string, fileIndex int, timestamp float64, duration float64, segmentIdx int) bool {
	return s.client.UpdatePlayback(infoHash, fileIndex, timestamp, duration, segmentIdx)
}

func (s *TorrentUsecase) UpdatePlaybackDuration(infoHash string, fileIndex int, duration float64) {
	s.client.UpdatePlaybackDuration(infoHash, fileIndex, duration)
}

func (s *TorrentUsecase) OnSeek(cb func(infoHash string, fileIndex int, segmentIdx int, timestamp float64)) {
	s.client.OnSeek(cb)
}
