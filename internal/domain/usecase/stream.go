package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"stream-web-api/internal/domain/model"
	domainrepo "stream-web-api/internal/domain/repository"
	"stream-web-api/pkg/ranger"
	"stream-web-api/pkg/srt"
	"stream-web-api/pkg/streamio"
)

const StreamSegmentDuration = 10.0

type streamReencodeJob struct {
	ID         string
	Filename   string
	Resolution string
	Bitrate    string
	Progress   model.ReencodeProgress
	Status     string
	cancel     context.CancelFunc
}

type StreamUsecase struct {
	torrentService *TorrentUsecase
	directService  *DirectDownloadUsecase
	transcoder     domainrepo.Transcoder
	cacheDir       string
	port           int

	durationMu      sync.RWMutex
	cachedDurations map[string]float64

	reencodeJobs sync.Map
}

func NewStreamUsecase(
	torrentService *TorrentUsecase,
	directService *DirectDownloadUsecase,
	transcoder domainrepo.Transcoder,
	cacheDir string,
	port int,
) *StreamUsecase {
	s := &StreamUsecase{
		torrentService:  torrentService,
		directService:   directService,
		transcoder:      transcoder,
		cacheDir:        cacheDir,
		port:            port,
		cachedDurations: make(map[string]float64),
	}

	torrentService.OnSeek(func(infoHash string, fileIndex int, segmentIdx int, timestamp float64) {
		s.PrefetchSegments(infoHash, fileIndex, segmentIdx)
	})

	return s
}

func (s *StreamUsecase) HandleKillStream() bool {
	return s.torrentService.KillActiveStream()
}

func (s *StreamUsecase) TranscoderNotAvailable() bool {
	return s.transcoder == nil
}

func (s *StreamUsecase) UpdatePlayback(infoHash string, fileIndex int, timestamp float64, duration float64, segmentIdx int) bool {
	return s.torrentService.UpdatePlayback(infoHash, fileIndex, timestamp, duration, segmentIdx)
}

func (s *StreamUsecase) EnsureFileHeader(infoHash string, fileIndex int) {
	_ = s.torrentService.EnsureFileHeader(infoHash, fileIndex)
}

func (s *StreamUsecase) TranscodeHLSSegment(ctx context.Context, w io.Writer, inputURL string, startTime float64, duration float64, cachePath string) error {
	videoCodec, audioCodec, _ := s.transcoder.GetStreamDetails(inputURL)
	return s.transcoder.TranscodeSegment(ctx, w, inputURL, startTime, duration, videoCodec, audioCodec)
}

func (s *StreamUsecase) GetReencodeJobs() []model.ReencodeJobStatus {
	var result []model.ReencodeJobStatus
	s.reencodeJobs.Range(func(key, value interface{}) bool {
		if job, ok := value.(*streamReencodeJob); ok {
			result = append(result, model.ReencodeJobStatus{
				ID:         job.ID,
				Filename:   job.Filename,
				Resolution: job.Resolution,
				Bitrate:    job.Bitrate,
				Progress: model.ReencodeProgress{
					Percent: job.Progress.Percent,
					Speed:   job.Progress.Speed,
					Time:    job.Progress.Time,
				},
				Status:     job.Status,
			})
		}
		return true
	})
	return result
}

func (s *StreamUsecase) HandleReencode(ctx context.Context, infoHash string, fileIndex int, downloadID int, resolution string, bitrate string) (*model.ReencodeJobResult, error) {
	if s.transcoder == nil {
		return nil, fmt.Errorf("Transcoder not available (FFmpeg not found)")
	}

	if resolution == "" {
		resolution = "1280:720"
	} else {
		resolution = streamReplaceX(resolution)
	}
	if bitrate == "" {
		bitrate = "2000k"
	}

	var inputURL string
	var filename string
	var baseDir string
	var jobID string

	if infoHash != "" {
		handle, err := s.torrentService.GetFileHandle(infoHash, fileIndex)
		if err != nil {
			if err.Error() == "invalid file index" {
				return nil, fmt.Errorf("invalid_file_index")
			}
			return nil, fmt.Errorf("torrent_not_found")
		}

		filename = handle.DisplayPath
		inputURL = s.GetLoopbackURL(infoHash, fileIndex)
		baseDir = filepath.Join(s.cacheDir, "exports", infoHash)
		jobID = fmt.Sprintf("torrent_%s_%d", infoHash, fileIndex)

		s.torrentService.StartFileDownload(infoHash, fileIndex)
	} else if downloadID != 0 {
		dl, err := s.directService.GetDownload(downloadID)
		if err != nil {
			return nil, fmt.Errorf("download_not_found")
		}

		filename = dl.Filename
		if dl.Status == "on_demand" {
			inputURL = dl.URL
		} else {
			inputURL = s.GetDirectLoopbackURL(downloadID)
		}
		baseDir = filepath.Join(s.cacheDir, "exports", fmt.Sprintf("direct_%d", downloadID))
		jobID = fmt.Sprintf("direct_%d", downloadID)
	} else {
		return nil, fmt.Errorf("infoHash or downloadId required")
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create export directory")
	}

	cleanName := filepath.Base(filename)
	ext := filepath.Ext(cleanName)
	nameWithoutExt := strings.TrimSuffix(cleanName, ext)
	outName := fmt.Sprintf("%s_%s.mp4", nameWithoutExt, streamReplaceColon(resolution))
	outputPath := filepath.Join(baseDir, outName)

	jobCtx, cancel := context.WithCancel(context.Background())
	job := &streamReencodeJob{
		ID:         jobID,
		Filename:   filename,
		Resolution: resolution,
		Bitrate:    bitrate,
		Status:     "processing",
		cancel:     cancel,
	}
	s.reencodeJobs.Store(jobID, job)

	go func() {
		log.Printf("🚀 [Reencode] Job started for %s (Res: %s, ID: %s)", filename, resolution, jobID)

		err := s.transcoder.ReencodeToFile(jobCtx, inputURL, outputPath, resolution, bitrate, func(p model.ReencodeProgress) {
			job.Progress = p
		})

		if err != nil {
			if jobCtx.Err() == context.Canceled {
				log.Printf("🛑 [Reencode] Job canceled by user: %s", filename)
				job.Status = "canceled"
			} else {
				log.Printf("❌ [Reencode] Job failed for %s: %v", filename, err)
				job.Status = "failed"
			}
			time.AfterFunc(10*time.Minute, func() { s.reencodeJobs.Delete(jobID) })
			os.Remove(outputPath)
		} else {
			log.Printf("✅ [Reencode] Job completed: %s -> %s", filename, outputPath)
			job.Status = "completed"
			job.Progress.Percent = 100
			time.AfterFunc(5*time.Minute, func() { s.reencodeJobs.Delete(jobID) })
		}
	}()

	return &model.ReencodeJobResult{
		Message:    "Reencoding started in background",
		OutputPath: outputPath,
	}, nil
}

func (s *StreamUsecase) HandleCancelReencode(id string) error {
	if val, ok := s.reencodeJobs.Load(id); ok {
		job := val.(*streamReencodeJob)
		if job.cancel != nil {
			job.cancel()
			return nil
		}
	}
	return fmt.Errorf("Job not found or already finished")
}

func (s *StreamUsecase) GetLoopbackURL(infoHash string, fileIndex int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", s.port, infoHash, fileIndex)
}

func (s *StreamUsecase) GetDirectLoopbackURL(downloadID int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/stream/direct/%d?raw=true", s.port, downloadID)
}

func (s *StreamUsecase) durationKey(infoHash string, fileIndex int) string {
	return fmt.Sprintf("%s/%d", infoHash, fileIndex)
}

func (s *StreamUsecase) GetCachedDuration(infoHash string, fileIndex int) (float64, bool) {
	s.durationMu.RLock()
	defer s.durationMu.RUnlock()
	d, ok := s.cachedDurations[s.durationKey(infoHash, fileIndex)]
	return d, ok
}

func (s *StreamUsecase) SetCachedDuration(infoHash string, fileIndex int, duration float64) {
	s.durationMu.Lock()
	defer s.durationMu.Unlock()
	s.cachedDurations[s.durationKey(infoHash, fileIndex)] = duration
	s.torrentService.UpdatePlaybackDuration(infoHash, fileIndex, duration)
}

func (s *StreamUsecase) PrefetchSegments(infoHash string, fileIndex int, fromSegment int) {
	if s.transcoder == nil {
		return
	}

	duration, hasDuration := s.GetCachedDuration(infoHash, fileIndex)
	if !hasDuration {
		log.Printf("⚠️ Prefetch skipped: duration unknown for %s/%d", infoHash, fileIndex)
		return
	}

	totalSegments := int(math.Ceil(duration / StreamSegmentDuration))

	for i := 1; i <= 3; i++ {
		segIdx := fromSegment + i
		if segIdx >= totalSegments {
			break
		}

		cacheSubDir := filepath.Join(s.cacheDir, infoHash, fmt.Sprintf("file_%d", fileIndex))
		cachePath := filepath.Join(cacheSubDir, fmt.Sprintf("segment_%d.ts", segIdx))

		if info, err := os.Stat(cachePath); err == nil && info.Size() > 1024 {
			continue
		}

		go s.generateSegmentInBackground(infoHash, fileIndex, segIdx, cachePath)
	}
}

func (s *StreamUsecase) generateSegmentInBackground(infoHash string, fileIndex int, segmentIdx int, cachePath string) {
	startTime := float64(segmentIdx) * StreamSegmentDuration

	log.Printf("🔮 Prefetching segment %d (time: %.1fs) for %s/%d", segmentIdx, startTime, infoHash, fileIndex)

	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("⚠️ Prefetch: failed to create cache dir: %v", err)
		return
	}

	inputURL := s.GetLoopbackURL(infoHash, fileIndex)

	s.torrentService.EnsureFileHeader(infoHash, fileIndex)

	videoCodec, audioCodec, err := s.transcoder.GetStreamDetails(inputURL)
	if err != nil {
		log.Printf("⚠️ Prefetch: codec detection failed: %v", err)
	}

	cacheFile, err := os.Create(cachePath)
	if err != nil {
		log.Printf("⚠️ Prefetch: failed to create cache file: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	err = s.transcoder.TranscodeSegment(ctx, cacheFile, inputURL, startTime, StreamSegmentDuration, videoCodec, audioCodec)
	cacheFile.Close()

	if err != nil {
		log.Printf("⚠️ Prefetch: segment %d transcode failed: %v", segmentIdx, err)
		os.Remove(cachePath)
		return
	}

	log.Printf("✅ Prefetched segment %d for %s/%d", segmentIdx, infoHash, fileIndex)
}

func (s *StreamUsecase) GetMediaInfo(ctx context.Context, infoHash string, fileIndex int) (*model.MediaInfo, error) {
	if s.transcoder == nil {
		return nil, fmt.Errorf("FFprobe not available")
	}

	s.torrentService.EnsureFileHeader(infoHash, fileIndex)

	handle, err := s.torrentService.GetFileHandle(infoHash, fileIndex)
	if err == nil && handle.PieceLength > 0 {
		startPiece := int(handle.Offset / handle.PieceLength)
		s.torrentService.WaitForPieces(infoHash, startPiece, startPiece, 10)
	} else {
		time.Sleep(500 * time.Millisecond)
	}

	inputURL := s.GetLoopbackURL(infoHash, fileIndex)

	duration, err := s.transcoder.GetVideoDurationFromURL(inputURL)
	if err != nil {
		log.Printf("Metadata error (duration): %v", err)
	}
	if duration > 0 {
		s.SetCachedDuration(infoHash, fileIndex, duration)
	}

	subs, err := s.transcoder.GetEmbeddedSubtitles(inputURL)
	if err != nil {
		log.Printf("Metadata error (subtitles): %v", err)
		subs = []model.SubtitleStream{}
	}

	return &model.MediaInfo{
		Duration:  duration,
		Subtitles: subs,
	}, nil
}

func (s *StreamUsecase) GetDuration(ctx context.Context, infoHash string, fileIndex int) (float64, error) {
	if s.transcoder == nil {
		return 0, fmt.Errorf("FFprobe not available")
	}

	s.torrentService.EnsureFileHeader(infoHash, fileIndex)

	inputURL := s.GetLoopbackURL(infoHash, fileIndex)
	duration, err := s.transcoder.GetVideoDurationFromURL(inputURL)
	if err != nil {
		return 0, fmt.Errorf("Failed to get duration: %w", err)
	}

	return duration, nil
}

func (s *StreamUsecase) GetHLSPlaylist(ctx context.Context, infoHash string, fileIndex int) (*model.HLSPlaylistResult, error) {
	if s.transcoder == nil {
		return nil, fmt.Errorf("Transcoder not available")
	}

	s.torrentService.EnsureFileHeader(infoHash, fileIndex)
	if err := s.torrentService.StartFileDownload(infoHash, fileIndex); err != nil {
		log.Printf("Failed to start file download: %v", err)
	}

	inputURL := s.GetLoopbackURL(infoHash, fileIndex)

	duration, err := s.transcoder.GetVideoDurationFromURL(inputURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get duration: %w", err)
	}

	s.SetCachedDuration(infoHash, fileIndex, duration)
	s.torrentService.UpdatePlaybackDuration(infoHash, fileIndex, duration)

	totalSegments := int(math.Ceil(duration / StreamSegmentDuration))

	var playlist strings.Builder
	playlist.WriteString("#EXTM3U\n")
	playlist.WriteString("#EXT-X-VERSION:3\n")
	playlist.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", int(StreamSegmentDuration)))
	playlist.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	playlist.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")

	for i := 0; i < totalSegments; i++ {
		segDur := StreamSegmentDuration
		if i == totalSegments-1 {
			segDur = duration - (float64(i) * StreamSegmentDuration)
		}
		playlist.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", segDur))
		playlist.WriteString(fmt.Sprintf("segment/segment_%d.ts\n", i))
	}

	playlist.WriteString("#EXT-X-ENDLIST\n")

	return &model.HLSPlaylistResult{
		Duration:      duration,
		TotalSegments: totalSegments,
		Playlist:      playlist.String(),
	}, nil
}

func (s *StreamUsecase) ServeHLSegment(ctx context.Context, w io.Writer, infoHash string, fileIndex int, segmentIdx int) error {
	cacheSubDir := filepath.Join(s.cacheDir, infoHash, fmt.Sprintf("file_%d", fileIndex))
	cachePath := filepath.Join(cacheSubDir, fmt.Sprintf("segment_%d.ts", segmentIdx))

	if info, err := os.Stat(cachePath); err == nil {
		if info.Size() > 1024 {
			log.Printf("Serving cached segment: %s", cachePath)
			f, err := os.Open(cachePath)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(w, f)
			return err
		}
		log.Printf("Found invalid cache file (too small), removing: %s", cachePath)
		os.Remove(cachePath)
	}

	if s.transcoder == nil {
		return fmt.Errorf("Transcoder not available")
	}

	startTime := float64(segmentIdx) * StreamSegmentDuration
	duration, _ := s.GetCachedDuration(infoHash, fileIndex)
	s.torrentService.UpdatePlayback(infoHash, fileIndex, startTime, duration, segmentIdx)

	if err := os.MkdirAll(cacheSubDir, 0755); err != nil {
		log.Printf("Failed to create cache dir: %v", err)
	}

	s.torrentService.EnsureFileHeader(infoHash, fileIndex)

	inputURL := s.GetLoopbackURL(infoHash, fileIndex)
	videoCodec, audioCodec, _ := s.transcoder.GetStreamDetails(inputURL)

	cacheFile, err := os.Create(cachePath)
	if err != nil {
		log.Printf("Failed to create cache file: %v", err)
		return s.transcoder.TranscodeSegment(ctx, w, inputURL, startTime, StreamSegmentDuration, videoCodec, audioCodec)
	}
	defer cacheFile.Close()

	multiWriter := io.MultiWriter(w, cacheFile)

	err = s.transcoder.TranscodeSegment(ctx, multiWriter, inputURL, startTime, StreamSegmentDuration, videoCodec, audioCodec)

	if err != nil {
		log.Printf("Segment transcode failed: %v", err)
		os.Remove(cachePath)
		return err
	}

	return nil
}

func (s *StreamUsecase) ExtractSubtitle(ctx context.Context, w io.Writer, infoHash string, fileIndex int, streamIndex int) error {
	if s.transcoder == nil {
		return fmt.Errorf("Transcoding not available")
	}

	s.torrentService.EnsureFileHeader(infoHash, fileIndex)

	handle, err := s.torrentService.GetFileHandle(infoHash, fileIndex)
	if err == nil && handle.PieceLength > 0 {
		startPiece := int(handle.Offset / handle.PieceLength)
		s.torrentService.WaitForPieces(infoHash, startPiece, startPiece, 10)
	} else {
		time.Sleep(500 * time.Millisecond)
	}

	inputURL := s.GetLoopbackURL(infoHash, fileIndex)

	var buf bytes.Buffer
	if err := s.transcoder.ExtractSubtitle(inputURL, streamIndex, &buf); err != nil {
		return err
	}

	cues := srt.Parse(buf.Bytes())

	b, err := json.Marshal(cues)
	if err != nil {
		return err
	}

	log.Printf("Subtitle extraction: Stream %d, Bytes %d, Cues %d", streamIndex, buf.Len(), len(cues))

	_, err = w.Write(b)
	return err
}

func (s *StreamUsecase) TranscodeToWriter(ctx context.Context, w io.Writer, infoHash string, fileIndex int, startTime float64, duration float64) error {
	if s.transcoder == nil {
		return fmt.Errorf("Transcoding not available (FFmpeg not found)")
	}

	handle, err := s.torrentService.GetFileHandle(infoHash, fileIndex)
	if err != nil {
		return fmt.Errorf("torrent not found: %w", err)
	}

	inputURL := s.GetLoopbackURL(infoHash, fileIndex)

	if startTime > 0 {
		if duration > 0 {
			if err := s.torrentService.SeekFileDownload(infoHash, fileIndex, startTime, duration); err != nil {
				log.Printf("Failed to seek file download: %v", err)
			}
			s.SetCachedDuration(infoHash, fileIndex, duration)
		} else {
			d, ok := s.GetCachedDuration(infoHash, fileIndex)
			if ok && d > 0 {
				if err := s.torrentService.SeekFileDownload(infoHash, fileIndex, startTime, d); err != nil {
					log.Printf("Failed to seek file download: %v", err)
				}
			}
		}
	} else {
		if err := s.torrentService.StartFileDownload(infoHash, fileIndex); err != nil {
			log.Printf("Failed to start file download: %v", err)
		}
	}

	s.torrentService.EnsureFileHeader(infoHash, fileIndex)

	pieceLength := handle.PieceLength
	startPiece := int(handle.Offset / pieceLength)
	if err := s.torrentService.WaitForPieces(infoHash, startPiece, startPiece, 10); err != nil {
		log.Printf("Warning: Timeout waiting for header, starting FFmpeg anyway: %v", err)
	}

	return s.transcoder.TranscodeStream(ctx, w, inputURL, handle.Length, handle.DisplayPath, startTime)
}

func (s *StreamUsecase) GetTorrentFileInfo(infoHash string, fileIndex int) (*model.File, error) {
	stats, err := s.torrentService.GetStats(infoHash)
	if err != nil {
		return nil, err
	}

	if len(stats.Files) <= fileIndex {
		return nil, fmt.Errorf("invalid file index")
	}

	return &stats.Files[fileIndex], nil
}

func (s *StreamUsecase) GetDirectDownloadInfo(id int) (*model.DirectDownload, error) {
	return s.directService.GetDownload(id)
}

func (s *StreamUsecase) StreamDirectFile(ctx context.Context, w io.Writer, id int, rangeHeader string) (contentType string, statusCode int, err error) {
	download, err := s.directService.GetDownload(id)
	if err != nil {
		return "", http.StatusNotFound, fmt.Errorf("download not found")
	}

	if download.Status == "on_demand" {
		return s.streamOnDemandDirect(ctx, w, id, download, rangeHeader)
	}

	if download.Status != "completed" {
		return "", http.StatusServiceUnavailable, fmt.Errorf("download not complete")
	}

	filePath := download.FilePath
	if filePath == "" {
		return "", http.StatusNotFound, fmt.Errorf("file not found")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", http.StatusNotFound, fmt.Errorf("file not found")
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("failed to stat file")
	}

	fileSize := info.Size()
	filename := info.Name()
	contentType = s.torrentService.GetMimeType(filename)

	start, end := int64(0), fileSize-1

	if rangeHeader != "" {
		var parseStart int64
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &parseStart)
		if err != nil {
			parseStart = 0
		}
		start = parseStart

		if strings.Contains(rangeHeader, "-") {
			parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
			if len(parts) == 2 && parts[1] != "" {
				end, _ = strconv.ParseInt(parts[1], 10, 64)
			}
		}

		if end > fileSize-1 {
			end = fileSize - 1
		}
		if start < 0 {
			start = 0
		}
		if start > end {
			start = 0
			end = fileSize - 1
		}
	}

	contentLength := end - start + 1

	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("failed to seek")
	}

	_, err = io.CopyN(w, f, contentLength)
	if err != nil {
		return contentType, http.StatusOK, err
	}
	return contentType, http.StatusOK, nil
}

func (s *StreamUsecase) streamOnDemandDirect(ctx context.Context, w io.Writer, id int, download *model.DirectDownload, rangeHeader string) (contentType string, statusCode int, err error) {
	if download.URL == "" {
		return "", http.StatusBadRequest, fmt.Errorf("missing url")
	}
	if download.FilePath == "" {
		return "", http.StatusNotFound, fmt.Errorf("missing cache file path")
	}

	s.directService.StartBackgroundPrefetch(id)

	start, end, hasRange := ranger.ParseByteRange(rangeHeader)
	if hasRange && end >= start && download.TotalBytes > 0 && s.directService.OnDemandIsCached(id, start, end+1) {
		return s.serveLocalRange(ctx, w, download.FilePath, download.Filename, start, end, download.TotalBytes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, download.URL, nil)
	if err != nil {
		return "", http.StatusBadRequest, fmt.Errorf("invalid url")
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", http.StatusBadGateway, fmt.Errorf("failed to fetch source")
	}
	defer resp.Body.Close()

	contentType = resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = s.torrentService.GetMimeType(download.Filename)
	}

	var writeStart int64 = 0
	total := download.TotalBytes

	if resp.StatusCode == http.StatusPartialContent {
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			if parseStart, parseEnd, t, ok := ranger.ParseContentRange(cr); ok {
				writeStart = parseStart
				if t > 0 {
					total = t
				}
				if !hasRange {
					start, end, hasRange = parseStart, parseEnd, true
				}
			}
		}
	} else if resp.StatusCode == http.StatusOK {
		writeStart = 0
		if resp.ContentLength > 0 {
			total = resp.ContentLength
		}
	}

	release := s.directService.OnDemandAcquireFileLock(id)
	defer release()

	cacheFile, err := os.OpenFile(download.FilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		_, _ = io.Copy(w, resp.Body)
		return contentType, resp.StatusCode, nil
	}
	defer cacheFile.Close()

	buf := make([]byte, 1024*256)
	var written int64
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			_, _ = cacheFile.WriteAt(buf[:n], writeStart+written)
			written += int64(n)
		}
		if readErr != nil {
			break
		}
	}

	if written > 0 {
		s.directService.OnDemandRecordRange(id, writeStart, writeStart+written, total, contentType)
	}

	return contentType, resp.StatusCode, nil
}

func (s *StreamUsecase) serveLocalRange(ctx context.Context, w io.Writer, filePath string, filename string, start int64, end int64, total int64) (contentType string, statusCode int, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", http.StatusNotFound, fmt.Errorf("file not found")
	}
	defer f.Close()

	contentType = s.torrentService.GetMimeType(filename)

	if start > 0 {
		_, err = f.Seek(start, io.SeekStart)
		if err != nil {
			return "", http.StatusInternalServerError, fmt.Errorf("seek failed")
		}
	}

	return s.serveFileRange(ctx, w, f, contentType, total, start, end)
}

func (s *StreamUsecase) serveFileRange(ctx context.Context, w io.Writer, f *os.File, contentType string, fileSize int64, start int64, end int64) (string, int, error) {
	contentLength := end - start + 1

	if start > 0 {
		_, err := f.Seek(start, io.SeekStart)
		if err != nil {
			return contentType, http.StatusInternalServerError, fmt.Errorf("seek failed")
		}
	}

	streamio.CopyWithTimeout(w, f, contentLength, ctx)
	return contentType, http.StatusPartialContent, nil
}

func (s *StreamUsecase) StreamTorrentFile(ctx context.Context, w io.Writer, infoHash string, fileIndex int, rangeHeader string, duration string, startTime float64, raw bool, download bool) (*model.StreamResult, io.Reader, error) {
	if !s.torrentService.IsTorrentReady(infoHash) {
		return nil, nil, fmt.Errorf("torrent_not_ready")
	}

	fileInfo, err := s.torrentService.GetFileInfo(infoHash, fileIndex)
	if err != nil {
		if err.Error() == "invalid file index" {
			return nil, nil, fmt.Errorf("invalid_file_index")
		}
		return nil, nil, fmt.Errorf("torrent_not_found")
	}

	fileSize := fileInfo.Length
	filename := fileInfo.Name
	contentType := s.torrentService.GetMimeType(filename)

	raw = raw || download

	if !raw && strings.HasPrefix(contentType, "video/") {
		description := fmt.Sprintf("stream %s/%d", infoHash, fileIndex)
		streamCtx, cancel := s.torrentService.AcquireStream(ctx, description)
		defer cancel()

		d, _ := strconv.ParseFloat(duration, 64)
		if d <= 0 {
			d = 0
		}
		err := s.TranscodeToWriter(streamCtx, w, infoHash, fileIndex, startTime, d)
		if err != nil {
			return nil, nil, err
		}
		return &model.StreamResult{
			ContentType: contentType,
			StatusCode:  http.StatusOK,
		}, nil, nil
	}

	streamCtx := ctx
	if !raw {
		description := fmt.Sprintf("stream %s/%d", infoHash, fileIndex)
		var cancel context.CancelFunc
		streamCtx, cancel = s.torrentService.AcquireStream(ctx, description)
		defer cancel()

		if s.torrentService.NeedsTranscoding(filename) {
			d, _ := strconv.ParseFloat(duration, 64)
			if d <= 0 {
				d = 0
			}
			err := s.TranscodeToWriter(streamCtx, w, infoHash, fileIndex, startTime, d)
			if err != nil {
				return nil, nil, err
			}
			return &model.StreamResult{
				ContentType: contentType,
				StatusCode:  http.StatusOK,
			}, nil, nil
		}
	}

	var start, end int64 = 0, fileSize-1
	if rangeHeader != "" {
		var parseStart int64
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &parseStart)
		if err != nil {
			parseStart = 0
		}
		start = parseStart

		if strings.Contains(rangeHeader, "-") {
			parts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
			if len(parts) == 2 && parts[1] != "" {
				end, _ = strconv.ParseInt(parts[1], 10, 64)
			}
		}

		if end > fileSize-1 {
			end = fileSize - 1
		}
		if start < 0 {
			start = 0
		}
		if start > end {
			start = 0
			end = fileSize - 1
		}
	}

	reader, err := s.torrentService.GetFileReader(infoHash, fileIndex, start, end)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to get file reader")
	}

	contentLength := end - start + 1
	isRangeRequest := rangeHeader != ""

	log.Printf("Streaming %s [%d-%d] (%d bytes)", filename, start, end, contentLength)

	return &model.StreamResult{
		ContentType:     contentType,
		StatusCode:      http.StatusOK,
		FileSize:        fileSize,
		ContentStart:    start,
		ContentEnd:      end,
		ContentLength:   contentLength,
		IsRangeRequest:  isRangeRequest,
		Filename:        filename,
	}, reader, nil
}

func (s *StreamUsecase) streamTorrentRange(ctx context.Context, w io.Writer, infoHash string, fileIndex int, contentType string, start int64, end int64, fileSize int64) (string, int, error) {
	reader, err := s.torrentService.GetFileReader(infoHash, fileIndex, start, end)
	if err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("failed to get reader")
	}

	contentLength := end - start + 1

	log.Printf("Streaming %s [%d-%d] (%d bytes)", infoHash, start, end, contentLength)

	streamio.CopyWithTimeout(w, reader, contentLength, ctx)
	return contentType, http.StatusPartialContent, nil
}

func (s *StreamUsecase) ParseSegmentID(segmentStr string) (int, error) {
	segmentStr = strings.TrimPrefix(segmentStr, "segment_")
	segmentStr = strings.TrimSuffix(segmentStr, ".ts")
	segmentIdx, err := strconv.Atoi(segmentStr)
	if err != nil {
		return 0, fmt.Errorf("invalid segment format")
	}
	return segmentIdx, nil
}

func streamReplaceX(s string) string {
	return strings.Replace(s, "x", ":", 1)
}

func streamReplaceColon(s string) string {
	return strings.ReplaceAll(s, ":", "p")
}
