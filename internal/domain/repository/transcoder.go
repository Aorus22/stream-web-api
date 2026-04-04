package repository

import (
	"context"
	"io"

	"stream-web-api/internal/domain/model"
)

type Transcoder interface {
	Acquire(ctx context.Context) (release func(), err error)
	TranscodeStream(ctx context.Context, w io.Writer, inputURL string, fileSize int64, filename string, startTime float64) error
	ReencodeToFile(ctx context.Context, inputURL string, outputPath string, resolution string, bitrate string, onProgress func(model.ReencodeProgress)) error
	TranscodeSegment(ctx context.Context, w io.Writer, inputURL string, startTime float64, duration float64, videoCodec string, audioCodec string) error
	GetVideoDurationFromURL(inputURL string) (float64, error)
	GetEmbeddedSubtitles(inputURL string) ([]model.SubtitleStream, error)
	ExtractSubtitle(inputURL string, streamIndex int, w io.Writer) error
	GetStreamDetails(inputURL string) (string, string, error)
	ExtractAudioSignature(inputURL string, startTime float64, durationSec int, sampleRate int, windowMs int, threshold float64) ([]float64, error)
}
