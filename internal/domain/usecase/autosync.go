package usecase

import (
	"fmt"
	"log"

	"stream-web-api/internal/domain/model"
	domainrepo "stream-web-api/internal/domain/repository"
	"stream-web-api/pkg/srt"
)

const (
	AutoSyncWindowSec = 120
	AutoVADSampleRate = 8000
	AutoVADWindowMs   = 20
	AutoVADThreshold  = 0.05
	AutoMaxOffsetSec  = 30.0
)

type AutoSyncUsecase struct {
	transcoder domainrepo.Transcoder
}

func NewAutoSyncUsecase(transcoder domainrepo.Transcoder) *AutoSyncUsecase {
	return &AutoSyncUsecase{
		transcoder: transcoder,
	}
}

func (s *AutoSyncUsecase) CalculateOffset(req model.AutoSyncRequest, srtContent []byte, port int) (*model.AutoSyncResponse, error) {
	if s.transcoder == nil {
		return nil, fmt.Errorf("transcoder not available")
	}

	cues := srt.Parse(srtContent)

	portStr := "8080"
	if port > 0 {
		portStr = fmt.Sprintf("%d", port)
	}
	inputURL := fmt.Sprintf("http://127.0.0.1:%s/stream/%s/%d?raw=true", portStr, req.InfoHash, req.FileIndex)

	log.Printf("AutoSync Computing... URL: %s, Start: %.2fs", inputURL, req.CurrentTime)

	audioSig, err := s.transcoder.ExtractAudioSignature(inputURL, req.CurrentTime, AutoSyncWindowSec, AutoVADSampleRate, AutoVADWindowMs, AutoVADThreshold)
	if err != nil {
		log.Printf("AudioExtract Error: %v", err)
		return nil, err
	}

	subSig := s.autoSyncRasterizeSubtitles(cues, req.CurrentTime, AutoSyncWindowSec)

	log.Printf("DEBUG: AudioSig Len: %d, SubSig Len: %d", len(audioSig), len(subSig))

	audioSum := 0.0
	for _, v := range audioSig {
		audioSum += v
	}
	subSum := 0.0
	for _, v := range subSig {
		subSum += v
	}
	log.Printf("DEBUG: Audio Activity: %.1f (%.1f%%), Sub Activity: %.1f (%.1f%%)",
		audioSum, audioSum/float64(len(audioSig))*100,
		subSum, subSum/float64(len(subSig))*100)
	log.Printf("DEBUG: Cues parsed: %d", len(cues))

	offset, score := s.autoSyncCalculateBestOffset(audioSig, subSig)

	log.Printf("AutoSync Result: Offset %.2fs (Score: %.1f)", offset, score)

	return &model.AutoSyncResponse{
		Offset: offset,
		Score:  score,
	}, nil
}

func (s *AutoSyncUsecase) AutoSyncWithDownload(subtitleService *SubtitleUsecase, req model.AutoSyncRequest, link string, port int) (*model.AutoSyncResponse, error) {
	srtContent, err := subtitleService.DownloadRaw(link)
	if err != nil {
		return nil, fmt.Errorf("failed to download subtitle: %w", err)
	}

	return s.CalculateOffset(req, srtContent, port)
}

func (s *AutoSyncUsecase) autoSyncRasterizeSubtitles(cues []model.SubtitleCue, startTime float64, durationSec float64) []float64 {
	totalWins := int(durationSec * 1000 / AutoVADWindowMs)
	timeline := make([]float64, totalWins)

	for _, cue := range cues {
		relStart := cue.Start - startTime
		relEnd := cue.End - startTime

		if relEnd < 0 {
			continue
		}
		if relStart > durationSec {
			continue
		}

		startWin := int(relStart * 1000 / AutoVADWindowMs)
		endWin := int(relEnd * 1000 / AutoVADWindowMs)

		if startWin < 0 {
			startWin = 0
		}
		if endWin >= totalWins {
			endWin = totalWins - 1
		}

		for i := startWin; i <= endWin; i++ {
			timeline[i] = 1.0
		}
	}
	return timeline
}

func (s *AutoSyncUsecase) autoSyncCalculateBestOffset(audioSig []float64, subSig []float64) (float64, float64) {
	if len(audioSig) == 0 || len(subSig) == 0 {
		return 0, 0
	}

	binsRange := int(AutoMaxOffsetSec * 1000 / AutoVADWindowMs)

	bestOffset := 0.0
	maxScore := -999999.0

	minLen := len(audioSig)
	if len(subSig) < minLen {
		minLen = len(subSig)
	}

	if binsRange > minLen/2 {
		binsRange = minLen / 2
	}

	log.Printf("DEBUG: Correlation binsRange: %d, minLen: %d", binsRange, minLen)

	for shift := -binsRange; shift <= binsRange; shift++ {
		score := 0.0
		matchCount := 0

		for i := 0; i < minLen; i++ {
			audioIdx := i
			subIdx := i - shift

			if subIdx < 0 || subIdx >= len(subSig) {
				continue
			}
			if audioIdx >= len(audioSig) {
				continue
			}

			score += audioSig[audioIdx] * subSig[subIdx]
			matchCount++
		}

		if matchCount > 0 {
			score = score / float64(matchCount)
		}

		if score > maxScore {
			maxScore = score
			bestOffset = float64(shift) * AutoVADWindowMs / 1000.0
		}
	}

	log.Printf("DEBUG: Best correlation score: %.4f at offset %.2fs", maxScore, bestOffset)

	return bestOffset, maxScore
}
