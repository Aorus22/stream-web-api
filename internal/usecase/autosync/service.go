package autosync

import (
	"fmt"
	"log"

	"torrent-stream/internal/domain"
	"torrent-stream/internal/infrastructure/ffmpeg"
	"torrent-stream/pkg/srt"
)

// Config for AutoSync
const (
	SyncWindowSec = 120 // Analyze 2 minutes
	VADSampleRate = 8000
	VADWindowMs   = 20
	VADThreshold  = 0.05
	MaxOffsetSec  = 30.0
)

// Service provides auto-sync business logic
type Service struct {
	transcoder *ffmpeg.Transcoder
}

// NewService creates a new autosync service
func NewService(transcoder *ffmpeg.Transcoder) *Service {
	return &Service{
		transcoder: transcoder,
	}
}

// CalculateOffset calculates subtitle offset using audio correlation
func (s *Service) CalculateOffset(req domain.AutoSyncRequest, srtContent []byte, port int) (*domain.AutoSyncResponse, error) {
	if s.transcoder == nil {
		return nil, fmt.Errorf("transcoder not available")
	}

	// Parse subtitle cues
	cues := srt.Parse(srtContent)

	// Build input URL for FFmpeg
	portStr := "8080"
	if port > 0 {
		portStr = fmt.Sprintf("%d", port)
	}
	inputURL := fmt.Sprintf("http://127.0.0.1:%s/stream/%s/%d?raw=true", portStr, req.InfoHash, req.FileIndex)

	log.Printf("🤖 AutoSync Computing... URL: %s, Start: %.2fs", inputURL, req.CurrentTime)

	// Extract audio signature
	audioSig, err := s.transcoder.ExtractAudioSignature(inputURL, req.CurrentTime, SyncWindowSec, VADSampleRate, VADWindowMs, VADThreshold)
	if err != nil {
		log.Printf("AudioExtract Error: %v", err)
		return nil, err
	}

	// Rasterize subtitles
	subSig := s.rasterizeSubtitles(cues, req.CurrentTime, SyncWindowSec)

	// Calculate correlation
	log.Printf("DEBUG: AudioSig Len: %d, SubSig Len: %d", len(audioSig), len(subSig))

	// Check for silence
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

	offset, score := s.calculateBestOffset(audioSig, subSig)

	log.Printf("🤖 AutoSync Result: Offset %.2fs (Score: %.1f)", offset, score)

	return &domain.AutoSyncResponse{
		Offset: offset,
		Score:  score,
	}, nil
}

// rasterizeSubtitles converts subtitle cues to activity timeline
func (s *Service) rasterizeSubtitles(cues []domain.SubtitleCue, startTime float64, durationSec float64) []float64 {
	totalWins := int(durationSec * 1000 / VADWindowMs)
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

		startWin := int(relStart * 1000 / VADWindowMs)
		endWin := int(relEnd * 1000 / VADWindowMs)

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

// calculateBestOffset finds the best offset using cross-correlation
func (s *Service) calculateBestOffset(audioSig []float64, subSig []float64) (float64, float64) {
	if len(audioSig) == 0 || len(subSig) == 0 {
		return 0, 0
	}

	binsRange := int(MaxOffsetSec * 1000 / VADWindowMs)

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
			bestOffset = float64(shift) * VADWindowMs / 1000.0
		}
	}

	log.Printf("DEBUG: Best correlation score: %.4f at offset %.2fs", maxScore, bestOffset)

	return bestOffset, maxScore
}
