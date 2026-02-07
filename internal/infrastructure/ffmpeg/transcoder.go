package ffmpeg

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Transcoder handles real-time video transcoding using FFmpeg
type Transcoder struct {
	FFmpegPath  string
	FFprobePath string
	mu          sync.Mutex
}

// NewTranscoder creates a new transcoder
func NewTranscoder() *Transcoder {
	ffmpegPath := "ffmpeg"
	ffprobePath := "ffprobe"

	// Check if FFmpeg is available
	cmd := exec.Command(ffmpegPath, "-version")
	if err := cmd.Run(); err != nil {
		log.Printf("⚠️ FFmpeg not found in PATH. Transcoding will not work.")
		log.Printf("   Install FFmpeg: https://ffmpeg.org/download.html")
		return nil
	}

	// Check if FFprobe is available
	cmd2 := exec.Command(ffprobePath, "-version")
	if err := cmd2.Run(); err != nil {
		log.Printf("⚠️ FFprobe not found - duration detection disabled")
		ffprobePath = ""
	}

	log.Printf("✅ FFmpeg found, transcoding enabled")
	return &Transcoder{
		FFmpegPath:  ffmpegPath,
		FFprobePath: ffprobePath,
	}
}

// TranscodeStream transcodes a video stream to browser-compatible format
func (t *Transcoder) TranscodeStream(ctx context.Context, w http.ResponseWriter, inputURL string, fileSize int64, filename string, startTime float64) error {
	if t == nil {
		return fmt.Errorf("transcoder not available (FFmpeg not found)")
	}

	// Determine output format based on Accept header or default to MP4
	// Defaulting to MP4 for stability with explicit context control
	outputFormat := "mp4"
	contentType := "video/mp4"

	log.Printf("🎬 Starting transcode: %s -> %s (start: %.1fs)", filename, outputFormat, startTime)

	var args []string

	// optimize probe size for faster startup
	args = append(args,
		"-fflags", "+genpts+igndts", // FIX: Generate timestamps, ignore dts mismatch
		"-analyzeduration", "20000000",
		"-probesize", "20000000",
	)

	args = append(args,
		"-i", inputURL,
		"-v", "warning",
	)

	if startTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", startTime)) // SEEK AFTER INPUT for accuracy
		args = append(args, "-copyts")                             // Preserve original timestamps
		args = append(args, "-start_at_zero")                      // Start timeline from zero after seek
	}

	// Standard MP4 args
	// We use -c:v copy to avoid re-encoding overhead (Instant Seek)
	// We re-encode audio to AAC to ensure browser compatibility
	// We disable subtitles (-sn) to avoid codec incompatibilities in MP4 container
	args = append(args,
		"-c:v", "copy",
		"-vsync", "2", // FIX: Pass-through with timestamp correction
		"-c:a", "aac",
		"-b:a", "192k",
		"-af", "aresample=async=1:first_pts=0", // FIX: Better audio sync
		"-sn",
		"-movflags", "frag_keyframe+empty_moov+faststart",
		"-f", "mp4",
	)

	args = append(args, "pipe:1")

	// Use CommandContext to allow killing via context
	cmd := exec.CommandContext(ctx, t.FFmpegPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get FFmpeg stdout: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get FFmpeg stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Log FFmpeg errors in background
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				log.Printf("FFmpeg: %s", string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	// Set response headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Stream FFmpeg output to client
	buf := make([]byte, 64*1024)
	flusher, canFlush := w.(http.Flusher)

	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				log.Printf("Client disconnected: %v", writeErr)
				cmd.Process.Kill()
				break
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("FFmpeg read error: %v", err)
			}
			break
		}
	}

	cmd.Wait()

	log.Printf("✅ Transcode complete: %s", filename)
	return nil
}

// GetStreamDetails returns the video and audio codec names
func (t *Transcoder) GetStreamDetails(inputURL string) (videoCodec, audioCodec string, err error) {
	if t == nil || t.FFprobePath == "" {
		return "", "", fmt.Errorf("FFprobe not available")
	}

	args := []string{
		"-v", "error",
		"-show_entries", "stream=codec_name,codec_type",
		"-of", "json",
		inputURL,
	}

	cmd := exec.Command(t.FFprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("ffprobe error: %w", err)
	}

	type ProbeStream struct {
		CodecName string `json:"codec_name"`
		CodecType string `json:"codec_type"`
	}
	type ProbeResult struct {
		Streams []ProbeStream `json:"streams"`
	}

	var result ProbeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return "", "", fmt.Errorf("json parse error: %w", err)
	}

	for _, s := range result.Streams {
		if s.CodecType == "video" && videoCodec == "" {
			videoCodec = s.CodecName
		}
		if s.CodecType == "audio" && audioCodec == "" {
			audioCodec = s.CodecName
		}
	}

	return videoCodec, audioCodec, nil
}

// TranscodeSegment generates a single HLS segment
func (t *Transcoder) TranscodeSegment(ctx context.Context, w io.Writer, inputURL string, startTime float64, duration float64, srcVideoCodec, srcAudioCodec string) error {
	if t == nil {
		return fmt.Errorf("transcoder not available")
	}

	vCodec := "libx264"
	aCodec := "aac"

	// Smart Codec: Copy if compatible
	// DISABLED: HLS segments must start with a Keyframe (IDR).
	// Copying arbitrary chunks (10s) often results in segments starting with P-frames,
	// causing the player to freeze (hold previous frame) while audio continues.
	// We must re-encode to ensure every segment starts with an IDR frame.
	/*
		if srcVideoCodec == "h264" {
			vCodec = "copy"
		}
	*/
	if srcAudioCodec == "aac" {
		aCodec = "copy"
	}

	// ffmpeg -ss [START] -t [DURATION] -i [INPUT] ...
	args := []string{
		"-ss", fmt.Sprintf("%.6f", startTime),
		"-t", fmt.Sprintf("%.6f", duration),
		"-i", inputURL,
		"-map", "0:v:0", // Only map first video stream
		"-map", "0:a:0", // Only map first audio stream
		"-c:v", vCodec,
	}

	// Only add video encoding params if NOT copying
	if vCodec != "copy" {
		args = append(args,
			"-preset", "ultrafast",
			"-tune", "zerolatency",
		)
	}

	args = append(args, "-c:a", aCodec)

	// Only add audio encoding params if NOT copying
	if aCodec != "copy" {
		args = append(args,
			"-ar", "44100",
			"-ac", "2",
			"-b:a", "128k",
		)
	}

	args = append(args,
		"-f", "mpegts",
		"-copyts",
		"-start_at_zero",
		"-y",
		"pipe:1",
	)

	// Use CommandContext
	cmd := exec.CommandContext(ctx, t.FFmpegPath, args...)

	// Bind output
	cmd.Stdout = w

	// Capture stderr for debugging
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	log.Printf("🎞️ Generaing Segment (V:%s, A:%s): Start %.2f, Dur %.2f", vCodec, aCodec, startTime, duration)

	if err := cmd.Run(); err != nil {
		log.Printf("❌ FFmpeg Error Output:\n%s", stderr.String())
		return fmt.Errorf("ffmpeg segment error: %w", err)
	}

	return nil
}

// GetVideoDuration returns the duration of a video file in seconds using FFprobe
func (t *Transcoder) GetVideoDuration(reader io.Reader) (float64, error) {
	if t == nil || t.FFprobePath == "" {
		return 0, fmt.Errorf("FFprobe not available")
	}

	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		"pipe:0",
	}

	cmd := exec.Command(t.FFprobePath, args...)
	cmd.Stdin = reader

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe error: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration output: %s", durationStr)
	}

	return duration, nil
}

// GetVideoDurationFromURL returns video duration using URL input
func (t *Transcoder) GetVideoDurationFromURL(inputURL string) (float64, error) {
	if t == nil || t.FFprobePath == "" {
		return 0, fmt.Errorf("FFprobe not available")
	}

	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputURL,
	}

	cmd := exec.Command(t.FFprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe error: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration output: %s", durationStr)
	}

	return duration, nil
}

// SubtitleStream represents an embedded subtitle stream
type SubtitleStream struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Codec    string `json:"codec"`
}

// GetEmbeddedSubtitles returns a list of embedded subtitles
func (t *Transcoder) GetEmbeddedSubtitles(inputURL string) ([]SubtitleStream, error) {
	if t == nil || t.FFprobePath == "" {
		return nil, fmt.Errorf("FFprobe not available")
	}

	return t.getEmbeddedSubtitlesJSON(inputURL)
}

func (t *Transcoder) getEmbeddedSubtitlesJSON(inputURL string) ([]SubtitleStream, error) {
	args := []string{
		"-v", "error",
		"-analyzeduration", "10000000", // Limit analysis to 10MB/10s
		"-probesize", "10000000",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name:stream_tags=language,title",
		"-of", "json",
		inputURL,
	}

	cmd := exec.Command(t.FFprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Define struct for JSON parsing
	type ProbeResult struct {
		Streams []struct {
			Index     int    `json:"index"`
			CodecName string `json:"codec_name"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
	}

	var result ProbeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	var subs []SubtitleStream
	for _, s := range result.Streams {
		title := s.Tags.Title

		// Map common language codes to readable names if title is missing
		if title == "" && s.Tags.Language != "" {
			title = getLanguageName(s.Tags.Language)
		}

		subs = append(subs, SubtitleStream{
			Index:    s.Index,
			Codec:    s.CodecName,
			Language: s.Tags.Language,
			Title:    title,
		})
	}

	return subs, nil
}

// getLanguageName maps ISO 639-2/3 codes to readable names
func getLanguageName(code string) string {
	code = strings.ToLower(code)
	langMap := map[string]string{
		"eng": "English",
		"jpn": "Japanese",
		"ind": "Indonesian",
		"spa": "Spanish",
		"fre": "French",
		"fra": "French",
		"ger": "German",
		"deu": "German",
		"ita": "Italian",
		"rus": "Russian",
		"por": "Portuguese",
		"chi": "Chinese",
		"zho": "Chinese",
		"kor": "Korean",
		"ara": "Arabic",
		"hin": "Hindi",
		"ben": "Bengali",
		"tha": "Thai",
		"vie": "Vietnamese",
		"may": "Malay",
		"msa": "Malay",
		"dut": "Dutch",
		"nld": "Dutch",
		"pol": "Polish",
		"tur": "Turkish",
	}

	if name, ok := langMap[code]; ok {
		return name
	}
	return strings.ToUpper(code) // Fallback to uppercase code
}

// ExtractSubtitle extracts a subtitle stream as SRT
func (t *Transcoder) ExtractSubtitle(inputURL string, streamIndex int, w io.Writer) error {
	if t == nil {
		return fmt.Errorf("transcoder not available")
	}

	// ffmpeg -i <input> -map 0:<index> -f srt pipe:1
	args := []string{
		"-analyzeduration", "5000000", // Reduce to 5MB/5s
		"-probesize", "5000000",
		"-i", inputURL,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-vn", // Disable video
		"-an", // Disable audio
		"-dn", // Disable data
		"-f", "srt",
		"-v", "quiet",
		"pipe:1",
	}

	cmd := exec.Command(t.FFmpegPath, args...)
	cmd.Stdout = w

	// Optional: capture stderr
	// cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract subtitle: %w", err)
	}

	return nil
}

// ExtractAudioSignature extracts audio activity signature for auto-sync
func (t *Transcoder) ExtractAudioSignature(inputURL string, startTime float64, durationSec int, sampleRate int, windowMs int, threshold float64) ([]float64, error) {
	if t == nil {
		return nil, fmt.Errorf("no transcoder")
	}

	args := []string{
		"-ss", fmt.Sprintf("%.2f", startTime),
		"-i", inputURL,
		"-t", strconv.Itoa(durationSec),
		"-ac", "1",
		"-ar", strconv.Itoa(sampleRate),
		"-f", "f32le",
		"-v", "quiet",
		"pipe:1",
	}

	cmd := exec.Command(t.FFmpegPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Read floats
	var samples []float32
	bytesBuf := make([]byte, 4096)

	for {
		n, err := stdout.Read(bytesBuf)
		if n > 0 {
			for i := 0; i < n; i += 4 {
				if i+4 > n {
					break
				}
				bits := binary.LittleEndian.Uint32(bytesBuf[i : i+4])
				f := math.Float32frombits(bits)
				samples = append(samples, f)
			}
		}
		if err != nil {
			break
		}
	}
	cmd.Wait()

	log.Printf("DEBUG: Extracted %d float samples from FFmpeg", len(samples))

	// Normalize Audio
	maxVal := 0.0
	for _, s := range samples {
		abs := math.Abs(float64(s))
		if abs > maxVal {
			maxVal = abs
		}
	}

	scale := 1.0
	if maxVal > 0 {
		scale = 1.0 / maxVal
	}

	log.Printf("DEBUG: Audio Normalization. Max Peak: %.4f, Scale: %.2f", maxVal, scale)

	// Process VAD
	samplesPerWin := sampleRate * windowMs / 1000
	var activity []float64

	for i := 0; i < len(samples); i += samplesPerWin {
		end := i + samplesPerWin
		if end > len(samples) {
			end = len(samples)
		}

		sum := 0.0
		for j := i; j < end; j++ {
			val := float64(samples[j]) * scale
			sum += val * val
		}
		rms := math.Sqrt(sum / float64(end-i))

		if rms > threshold {
			activity = append(activity, 1.0)
		} else {
			activity = append(activity, 0.0)
		}
	}

	return activity, nil
}
