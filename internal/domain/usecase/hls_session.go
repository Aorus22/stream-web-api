package usecase

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type HLSSession struct {
	ID           string
	InfoHash     string
	FileIndex    int
	StartTime    float64
	Cmd          *exec.Cmd
	Cancel       context.CancelFunc
	SegmentDir   string
	PlaylistPath string
	LastAccess   time.Time
}

type HLSSessionManager struct {
	sessions     sync.Map
	torrentSvc   *TorrentUsecase
	streamSvc    *StreamUsecase
	port         int
	baseDir      string
	ffmpegPath   string
}

func NewHLSSessionManager(torrentSvc *TorrentUsecase, streamSvc *StreamUsecase, port int, baseDir string) *HLSSessionManager {
	if baseDir == "" {
		baseDir = "/dev/shm/hls"
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		log.Printf("Warning: failed to create HLS live dir %s: %v, falling back to ./hls_live", baseDir, err)
		baseDir = "./hls_live"
		os.MkdirAll(baseDir, 0755)
	}

	ffmpegPath := "ffmpeg"
	if _, err := exec.LookPath(ffmpegPath); err != nil {
		log.Printf("Warning: ffmpeg not found, HLS live streaming will not work")
		ffmpegPath = ""
	}

	return &HLSSessionManager{
		torrentSvc: torrentSvc,
		streamSvc:  streamSvc,
		port:       port,
		baseDir:    baseDir,
		ffmpegPath: ffmpegPath,
	}
}

type CreateSessionParams struct {
	InfoHash  string
	FileIndex int
	StartTime float64
}

func (m *HLSSessionManager) CreateSession(ctx context.Context, params CreateSessionParams) (*HLSSession, error) {
	if m.ffmpegPath == "" {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	sessionID := uuid.New().String()
	segmentDir := filepath.Join(m.baseDir, sessionID)
	if err := os.MkdirAll(segmentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session dir: %w", err)
	}

	playlistPath := filepath.Join(segmentDir, "playlist.m3u8")

	inputURL := fmt.Sprintf("http://127.0.0.1:%d/stream/%s/%d?raw=true", m.port, params.InfoHash, params.FileIndex)

	if params.StartTime > 0 {
		duration, hasDuration := m.streamSvc.GetCachedDuration(params.InfoHash, params.FileIndex)
		if hasDuration && duration > 0 {
			_ = m.torrentSvc.SeekFileDownload(params.InfoHash, params.FileIndex, params.StartTime, duration)
		} else {
			_ = m.torrentSvc.StartFileDownload(params.InfoHash, params.FileIndex)
		}
	} else {
		_ = m.torrentSvc.StartFileDownload(params.InfoHash, params.FileIndex)
	}

	_ = m.torrentSvc.EnsureFileHeader(params.InfoHash, params.FileIndex)

	args := m.buildFFmpegArgs(inputURL, segmentDir, playlistPath, params.StartTime)

	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, m.ffmpegPath, args...)

	stderrPipe, _ := cmd.StderrPipe()
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			log.Printf("[HLS-Live/ffmpeg] %s: %s", sessionID, scanner.Text())
		}
	}()

	log.Printf("[HLS-Live] Starting session %s for %s/%d (startTime=%.2f)", sessionID, params.InfoHash, params.FileIndex, params.StartTime)

	if err := cmd.Start(); err != nil {
		cancel()
		os.RemoveAll(segmentDir)
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	session := &HLSSession{
		ID:           sessionID,
		InfoHash:     params.InfoHash,
		FileIndex:    params.FileIndex,
		StartTime:    params.StartTime,
		Cmd:          cmd,
		Cancel:       cancel,
		SegmentDir:   segmentDir,
		PlaylistPath: playlistPath,
		LastAccess:   time.Now(),
	}

	m.sessions.Store(sessionID, session)

	go m.watchProcess(session)

	return session, nil
}

func (m *HLSSessionManager) buildFFmpegArgs(inputURL, segmentDir, playlistPath string, startTime float64) []string {
	args := []string{}

	if startTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", startTime))
	}

	args = append(args,
		"-fflags", "+genpts+igndts",
		"-analyzeduration", "2000000",
		"-probesize", "2000000",
	)

	args = append(args,
		"-reconnect", "1",
		"-reconnect_at_eof", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "2",
	)

	args = append(args,
		"-re",
		"-i", inputURL,
		"-c:v", "copy",
		"-c:a", "aac",
		"-b:a", "192k",
		"-af", "aresample=async=1:first_pts=0",
		"-vsync", "2",
		"-sn",
		"-f", "hls",
		"-hls_time", "4",
		"-hls_list_size", "10",
		"-hls_flags", "delete_segments+append_list+omit_endlist",
		"-hls_segment_type", "fmp4",
		"-hls_segment_filename", filepath.Join(segmentDir, "seg_%d.m4s"),
		"-y",
		playlistPath,
	)

	return args
}

func (m *HLSSessionManager) watchProcess(session *HLSSession) {
	err := session.Cmd.Wait()
	if err != nil {
		log.Printf("[HLS-Live] Session %s ffmpeg exited: %v", session.ID, err)
	} else {
		log.Printf("[HLS-Live] Session %s ffmpeg stopped", session.ID)
	}
}

func (m *HLSSessionManager) GetSession(id string) (*HLSSession, bool) {
	val, ok := m.sessions.Load(id)
	if !ok {
		return nil, false
	}
	session := val.(*HLSSession)
	session.LastAccess = time.Now()
	return session, true
}

func (m *HLSSessionManager) StopSession(id string) {
	val, ok := m.sessions.LoadAndDelete(id)
	if !ok {
		return
	}
	session := val.(*HLSSession)
	m.killSession(session)
}

func (m *HLSSessionManager) killSession(session *HLSSession) {
	if session.Cancel != nil {
		session.Cancel()
	}
	if session.Cmd != nil && session.Cmd.Process != nil {
		session.Cmd.Process.Kill()
	}
	os.RemoveAll(session.SegmentDir)
	log.Printf("[HLS-Live] Session %s stopped and cleaned up", session.ID)
}

func (m *HLSSessionManager) StartCleanup(idleTimeout time.Duration, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		m.sessions.Range(func(key, value any) bool {
			session := value.(*HLSSession)
			if now.Sub(session.LastAccess) > idleTimeout {
				log.Printf("[HLS-Live] Cleaning up idle session %s", session.ID)
				m.sessions.Delete(key)
				m.killSession(session)
			}
			return true
		})
	}
}

func (m *HLSSessionManager) Cleanup() {
	m.sessions.Range(func(key, value any) bool {
		session := value.(*HLSSession)
		m.killSession(session)
		m.sessions.Delete(key)
		return true
	})
}
