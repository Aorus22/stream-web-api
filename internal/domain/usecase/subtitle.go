package usecase

import (
	"fmt"
	"strings"

	"stream-web-api/internal/domain/model"
	domainrepo "stream-web-api/internal/domain/repository"
	"stream-web-api/pkg/srt"
)

type SubtitleUsecase struct {
	osClient   domainrepo.OpenSubtitlesClient
	downloader domainrepo.SubtitleDownloader
}

func NewSubtitleUsecase(osClient domainrepo.OpenSubtitlesClient, downloader domainrepo.SubtitleDownloader) *SubtitleUsecase {
	return &SubtitleUsecase{
		osClient:   osClient,
		downloader: downloader,
	}
}

func (s *SubtitleUsecase) Search(query, lang string) ([]model.Subtitle, error) {
	return s.osClient.Search(query, lang)
}

func (s *SubtitleUsecase) Download(link string) ([]model.SubtitleCue, error) {
	bodyBytes, err := s.downloader.Download(link)
	if err != nil {
		return nil, err
	}
	cues := srt.Parse(bodyBytes)
	if len(cues) == 0 {
		if len(bodyBytes) > 10 && strings.Contains(strings.ToLower(string(bodyBytes[:100])), "<html") {
			return nil, fmt.Errorf("received HTML instead of subtitle (blocked or bot detection)")
		}
		return nil, fmt.Errorf("no cues parsed from subtitle file (size: %d bytes)", len(bodyBytes))
	}
	return cues, nil
}

func (s *SubtitleUsecase) DownloadRaw(link string) ([]byte, error) {
	return s.downloader.Download(link)
}
