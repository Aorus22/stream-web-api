import { useState, useRef, useEffect, useCallback } from "react";
import Hls from "hls.js";
import { useSearchParams } from "react-router-dom";
import { Play, Pause, Maximize, Minimize, Volume2, VolumeX, ArrowLeft, RotateCcw, RotateCw } from "lucide-react";
import { cn } from "../../lib/utils";
import { Toaster } from "../../components/ui/sonner";
import SubtitlePopover from "./SubtitlePopover";
import SettingsPopover from "./SettingsPopover";
import { useServer } from "../../contexts/ServerContext";

// Helper to format seconds to HH:MM:SS
const formatTime = (seconds: number) => {
    if (!seconds || isNaN(seconds)) return "00:00:00";
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    return `${h.toString().padStart(2, "0")}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
};

type FileInfo = {
    name: string;
    size: number;
};

type TorrentMeta = {
    title: string;
    background: string;
    logo: string;
};

type SubtitleCue = {
    start: number;
    end: number;
    text: string;
    position?: string;
};

type EmbeddedSubtitle = {
    index: number;
    language: string;
    title: string;
    codec: string;
};

type SSEStats = {
    downloadSpeed?: number;
    files?: Array<{
        name?: string;
        length: number;
        progress?: number;
        piecesReady?: number;
        piecesTotal?: number;
        bufferedRanges?: Array<{ start: number; end: number }>;
    }>;
};

export default function VideoPlayer() {
    const { serverUrl } = useServer();
    const [searchParams] = useSearchParams();
    const infoHash = searchParams.get('infoHash') || '';
    const fileIndex = parseInt(searchParams.get('fileIndex') || searchParams.get('file') || '0');
    const directIdParam = searchParams.get('directId');
    const directId = directIdParam ? parseInt(directIdParam, 10) : NaN;
    const isDirectDownload = Number.isFinite(directId);

    const videoRef = useRef<HTMLVideoElement>(null);
    const containerRef = useRef<HTMLDivElement>(null);
    const hlsRef = useRef<Hls | null>(null);

    const [playing, setPlaying] = useState(false);
    const [currentTime, setCurrentTime] = useState(0);
    const [duration, setDuration] = useState(0);
    const [volume, setVolume] = useState(1);
    const [muted, setMuted] = useState(false);
    const [fullscreen, setFullscreen] = useState(false);
    const [showControls, setShowControls] = useState(true);
    const [loading, setLoading] = useState(true);
    const [initialLoading, setInitialLoading] = useState(true);
    const [fileInfo, setFileInfo] = useState<FileInfo | null>(null);
    const [torrentMeta, setTorrentMeta] = useState<TorrentMeta | null>(null);
    const seekOffsetRef = useRef(0);
    const isDraggingRef = useRef(false);
    const hlsSessionIdRef = useRef<string | null>(null);
    const hlsStartTimeRef = useRef(0);
    const seekAmount = 10; // seconds to seek with arrow keys

    // Stream mode: "direct" = fMP4 remux (fast), "hls" = HLS transcoding (compatible)
    const [streamMode, setStreamMode] = useState<'direct' | 'hls' | 'static'>(() => {
        const saved = localStorage.getItem('streamMode');
        return (saved === 'hls' || saved === 'direct' || saved === 'static') ? saved : 'direct';
    });

    const [embeddedSubs, setEmbeddedSubs] = useState<EmbeddedSubtitle[]>([]);
    const [selectedSubId, setSelectedSubId] = useState<string | null>(null);
    const [currentSubLink, setCurrentSubLink] = useState<string | null>(null);
    const [feedback, setFeedback] = useState<{ type: 'play' | 'pause' | 'forward' | 'backward', text?: string, position?: 'left' | 'right' | 'center', id: number } | null>(null);
    const [bufferedRanges, setBufferedRanges] = useState<{ start: number; end: number }[]>([]);
    const [hlsBufferedRanges, setHlsBufferedRanges] = useState<{ start: number; end: number }[]>([]);
    const [hoverTime, setHoverTime] = useState<number | null>(null);
    const [hoverPosition, setHoverPosition] = useState<number>(0);
    const [staticProgress, setStaticProgress] = useState<number>(0);
    const [staticSpeed, setStaticSpeed] = useState<number>(0);
    const [staticReady, setStaticReady] = useState(false);

    // Client-Side Rendering State
    const [subtitleCues, setSubtitleCues] = useState<SubtitleCue[]>([]);
    const [currentActiveCues, setCurrentActiveCues] = useState<SubtitleCue[]>([]);

    // Video Rect for Subtitles
    const [videoRect, setVideoRect] = useState({ top: 0, left: 0, width: 0, height: 0 });

    const updateVideoRect = useCallback(() => {
        const video = videoRef.current;
        const container = containerRef.current;
        if (!video || !container) return;

        const containerRect = container.getBoundingClientRect();
        const videoWidth = video.videoWidth || 16;
        const videoHeight = video.videoHeight || 9;
        const videoRatio = videoWidth / videoHeight;
        const containerRatio = containerRect.width / containerRect.height;

        let width, height, top, left;
        if (containerRatio > videoRatio) {
            height = containerRect.height;
            width = height * videoRatio;
            top = 0;
            left = (containerRect.width - width) / 2;
        } else {
            width = containerRect.width;
            height = width / videoRatio;
            left = 0;
            top = (containerRect.height - height) / 2;
        }

        setVideoRect({ top, left, width, height });
    }, []);

    useEffect(() => {
        const video = videoRef.current;
        if (!video) return;

        video.addEventListener('loadedmetadata', updateVideoRect);
        window.addEventListener('resize', updateVideoRect);

        const observer = new ResizeObserver(updateVideoRect);
        if (containerRef.current) observer.observe(containerRef.current);

        return () => {
            video.removeEventListener('loadedmetadata', updateVideoRect);
            window.removeEventListener('resize', updateVideoRect);
            observer.disconnect();
        };
    }, [updateVideoRect]);

    // Style settings - Load from localStorage
    const [subOffset, setSubOffset] = useState(0); // in seconds
    const [subSize, setSubSize] = useState(() => {
        const saved = localStorage.getItem('subSize');
        return saved ? Number(saved) : 100;
    });
    const [subPos, setSubPos] = useState(() => {
        const saved = localStorage.getItem('subPos');
        return saved ? Number(saved) : 0; // Default to bottom area
    });

    // Save subtitle settings to localStorage
    useEffect(() => {
        localStorage.setItem('subSize', String(subSize));
    }, [subSize]);

    useEffect(() => {
        localStorage.setItem('subPos', String(subPos));
    }, [subPos]);

    useEffect(() => {
        localStorage.setItem('streamMode', streamMode);
    }, [streamMode]);

    const controlsTimeoutRef = useRef<number>(0);
    const feedbackTimeoutRef = useRef<number>(0);

    // Helper for feedback
    const triggerFeedback = (type: 'play' | 'pause' | 'forward' | 'backward', text?: string, position: 'left' | 'right' | 'center' = 'center') => {
        if (feedbackTimeoutRef.current) clearTimeout(feedbackTimeoutRef.current);
        setFeedback({ type, text, position, id: Date.now() });
        feedbackTimeoutRef.current = window.setTimeout(() => {
            setFeedback(null);
        }, 600);
    };

    // 1. Load Metadata
    useEffect(() => {
        const fetchData = async () => {
            if (!serverUrl) return;
            try {
                if (isDirectDownload) {
                    const res = await fetch(`${serverUrl}/api/direct/${directId}`);
                    const data = await res.json();
                    if (data?.filename) {
                        setFileInfo({ name: data.filename, size: data.totalBytes || 0 });
                    }
                    return;
                }

                const statsRes = await fetch(`${serverUrl}/api/stats/${infoHash}`);
                const statsData = await statsRes.json();
                const file = statsData.files && statsData.files[Number(fileIndex)];
                if (file) {
                    setFileInfo({
                        name: file.name || '',
                        size: file.length || 0
                    });
                }

                const metaRes = await fetch(`${serverUrl}/api/metadata/${infoHash}/${fileIndex}`);
                const metaData = await metaRes.json();
                if (metaData.duration > 0) setDuration((prev) => Math.max(prev, metaData.duration));
                if (metaData.subtitles) setEmbeddedSubs(metaData.subtitles);

                try {
                    const torrentMetaRes = await fetch(`${serverUrl}/api/torrent/metadata/${infoHash}`);
                    if (torrentMetaRes.ok) {
                        const torrentMetaData = await torrentMetaRes.json();
                        if (torrentMetaData.title) {
                            setTorrentMeta(torrentMetaData as TorrentMeta);
                        }
                    }
                } catch {}
            } catch (err) {
                console.error("Metadata error", err);
            }
        };
        fetchData();
    }, [infoHash, fileIndex, serverUrl, isDirectDownload, directId]);

    // SSE for stats
    useEffect(() => {
        if (isDirectDownload || !infoHash || !serverUrl) return;

        const eventSource = new EventSource(`${serverUrl}/api/stats/${infoHash}/stream`);

        eventSource.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data) as SSEStats;
                const file = data.files && data.files[Number(fileIndex)];

                if (data.downloadSpeed !== undefined) {
                    setStaticSpeed(data.downloadSpeed);
                }

                if (file) {
                    // Dynamic update file info from SSE
                    setFileInfo(prev => ({
                        name: file.name || prev?.name || '',
                        size: file.length || prev?.size || 0
                    }));

                    if (file.bufferedRanges && file.length) {
                        const ranges = file.bufferedRanges.map((r) => ({
                            start: (r.start / file.length) * duration,
                            end: (r.end / file.length) * duration
                        }));
                        setBufferedRanges(ranges);
                    }

                    if (file.progress !== undefined) {
                        setStaticProgress(file.progress);
                    }
                }
            } catch (e) {
                console.error("SSE Parse Error", e);
            }
        };

        eventSource.onerror = (e) => {
            console.error("SSE Error", e);
            eventSource.close();
        };

        return () => {
            eventSource.close();
        };
    }, [infoHash, fileIndex, duration, serverUrl, isDirectDownload]);

    const togglePlay = useCallback(() => {
        if (!videoRef.current) return;

        if (videoRef.current.paused) {
            videoRef.current.play();
            setPlaying(true);
            triggerFeedback('play');
        } else {
            videoRef.current.pause();
            setPlaying(false);
            triggerFeedback('pause');
        }
    }, []);

    const syncDurationFromVideo = useCallback(() => {
        const video = videoRef.current;
        if (!video) return;
        const d = video.duration;
        if (!Number.isFinite(d) || d <= 0) return;

        setDuration((prev) => Math.max(prev, d));
    }, []);

    const handleSeek = useCallback((time: number) => {
        const video = videoRef.current;
        const videoDuration = video?.duration;
        const observedDuration = (Number.isFinite(videoDuration) && (videoDuration as number) > 0) ? (videoDuration as number) : 0;
        const effectiveDuration = Math.max(duration, observedDuration);
        const clampMax = effectiveDuration > 0 ? effectiveDuration : time;
        const targetTime = Math.max(0, Math.min(time, clampMax));
        setCurrentTime(targetTime);

        if (videoRef.current && serverUrl) {
            if (streamMode === 'static' && !isDirectDownload) {
                videoRef.current.currentTime = targetTime;
                return;
            }

            if (streamMode === 'hls' && !isDirectDownload) {
                const vid = videoRef.current;
                if (vid) {
                    const absTarget = hlsStartTimeRef.current + targetTime;
                    for (let i = 0; i < vid.buffered.length; i++) {
                        if (absTarget >= vid.buffered.start(i) && absTarget <= vid.buffered.end(i)) {
                            vid.currentTime = absTarget;
                            return;
                        }
                    }
                }

                (async () => {
                    if (hlsSessionIdRef.current) {
                        fetch(`${serverUrl}/hls-live/${hlsSessionIdRef.current}`, { method: 'DELETE' }).catch(() => {});
                    }

                    try {
                        const startRes = await fetch(`${serverUrl}/hls-live/start`, {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ infoHash, fileIndex, startTime: targetTime }),
                        });
                        const { sessionId, playlistUrl, startTime } = await startRes.json();

                        hlsSessionIdRef.current = sessionId;
                        hlsStartTimeRef.current = startTime;

                        if (hlsRef.current) {
                            hlsRef.current.destroy();
                            hlsRef.current = null;
                        }

                        if (Hls.isSupported()) {
                            const hls = new Hls({
                                debug: false,
                                enableWorker: true,
                                maxBufferLength: 30,
                                maxMaxBufferLength: 60,
                                maxBufferSize: 60 * 1000 * 1000,
                                backBufferLength: 8,
                            });
                            hlsRef.current = hls;
                            hls.loadSource(`${serverUrl}${playlistUrl}`);
                            hls.attachMedia(videoRef.current!);
                            hls.on(Hls.Events.ERROR, (_, data) => {
                                if (data.fatal) {
                                    switch (data.type) {
                                        case Hls.ErrorTypes.NETWORK_ERROR:
                                            hls.startLoad();
                                            break;
                                        case Hls.ErrorTypes.MEDIA_ERROR:
                                            hls.recoverMediaError();
                                            break;
                                        default:
                                            hls.destroy();
                                            break;
                                    }
                                }
                            });
                        }
                    } catch (err) {
                        console.error("HLS live seek failed:", err);
                    }
                })();
            } else {
                if (isDirectDownload) {
                    videoRef.current.currentTime = targetTime;
                    videoRef.current.play();
                    setPlaying(true);
                } else {
                    seekOffsetRef.current = targetTime;
                    const dParam = effectiveDuration > 0 ? `&d=${encodeURIComponent(String(effectiveDuration))}` : "";
                    videoRef.current.src = `${serverUrl}/stream/${infoHash}/${fileIndex}?t=${encodeURIComponent(String(targetTime))}${dParam}`;
                    videoRef.current.play();
                    setPlaying(true);
                }
            }
        }
    }, [duration, streamMode, infoHash, fileIndex, serverUrl, isDirectDownload]);

    const toggleFullscreen = () => {
        if (!document.fullscreenElement) {
            containerRef.current?.requestFullscreen();
            setFullscreen(true);
        } else {
            document.exitFullscreen();
            setFullscreen(false);
        }
    };

    const handleDoubleClick = (e: React.MouseEvent) => {
        if (!containerRef.current) return;
        const rect = containerRef.current.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const width = rect.width;
        const percentage = x / width;

        if (percentage < 0.3) {
            handleSeek(currentTime - 10);
            triggerFeedback('backward', '10s', 'left');
        } else if (percentage > 0.7) {
            handleSeek(currentTime + 10);
            triggerFeedback('forward', '10s', 'right');
        } else {
            toggleFullscreen();
        }
    };

    const handleMouseMove = () => {
        setShowControls(true);
        if (controlsTimeoutRef.current) clearTimeout(controlsTimeoutRef.current);
        controlsTimeoutRef.current = setTimeout(() => {
            if (!videoRef.current?.paused) setShowControls(false);
        }, 4000);
    };

    // Keyboard Shortcuts
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if ((e.target as HTMLElement).tagName === 'INPUT') return;

            switch (e.key) {
                case 'ArrowRight':
                    e.preventDefault();
                    handleSeek(currentTime + seekAmount);
                    triggerFeedback('forward', `${seekAmount}s`, 'right');
                    break;
                case 'ArrowLeft':
                    e.preventDefault();
                    handleSeek(currentTime - seekAmount);
                    triggerFeedback('backward', `${seekAmount}s`, 'left');
                    break;
                case ' ':
                    e.preventDefault();
                    togglePlay();
                    break;
            }
        };

        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [currentTime, handleSeek, togglePlay]);

    // Initialize stream based on mode
    useEffect(() => {
        const video = videoRef.current;
        if (!video || !serverUrl) return;

        // Stop current playback and clear source
        video.pause();
        video.src = "";
        video.load();

        if (hlsRef.current) {
            hlsRef.current.destroy();
            hlsRef.current = null;
        }

        if (isDirectDownload) {
            video.src = `${serverUrl}/stream/direct/${directId}`;
        } else if (streamMode === 'static') {
            setStaticProgress(0);
            setStaticReady(false);
            setInitialLoading(true);

            fetch(`${serverUrl}/api/static/prepare`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ infoHash, fileIndex }),
            }).catch((err) => {
                console.error("Failed to prepare static download:", err);
            });
        } else if (streamMode === 'hls') {
            if (Hls.isSupported()) {
                (async () => {
                    try {
                        const startRes = await fetch(`${serverUrl}/hls-live/start`, {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ infoHash, fileIndex, startTime: 0 }),
                        });
                        const { sessionId, playlistUrl, startTime } = await startRes.json();

                        hlsSessionIdRef.current = sessionId;
                        hlsStartTimeRef.current = startTime;

                        const hls = new Hls({
                            debug: false,
                            enableWorker: true,
                            maxBufferLength: 30,
                            maxMaxBufferLength: 60,
                            maxBufferSize: 60 * 1000 * 1000,
                            backBufferLength: 8,
                        });
                        hlsRef.current = hls;
                        hls.loadSource(`${serverUrl}${playlistUrl}`);
                        hls.attachMedia(video);
                        hls.on(Hls.Events.ERROR, (_, data) => {
                            if (data.fatal) {
                                switch (data.type) {
                                    case Hls.ErrorTypes.NETWORK_ERROR:
                                        hls.startLoad();
                                        break;
                                    case Hls.ErrorTypes.MEDIA_ERROR:
                                        hls.recoverMediaError();
                                        break;
                                    default:
                                        hls.destroy();
                                        break;
                                }
                            }
                        });
                    } catch (err) {
                        console.error("Failed to start HLS live session:", err);
                    }
                })();
            } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
                video.src = `${serverUrl}/hls/${infoHash}/${fileIndex}/playlist.m3u8`;
            }
        } else {
            // Direct fMP4 stream — BE remuxes (copy video, re-encode audio only)
            video.src = `${serverUrl}/stream/${infoHash}/${fileIndex}`;
        }

        return () => {
            if (videoRef.current) {
                videoRef.current.pause();
                videoRef.current.src = "";
                videoRef.current.load();
            }
            if (hlsRef.current) {
                hlsRef.current.destroy();
                hlsRef.current = null;
            }
            if (hlsSessionIdRef.current && serverUrl) {
                fetch(`${serverUrl}/hls-live/${hlsSessionIdRef.current}`, { method: 'DELETE' }).catch(() => {});
                hlsSessionIdRef.current = null;
            }
        };
    }, [streamMode, serverUrl, infoHash, fileIndex, isDirectDownload, directId]);

    // Static mode: start playback when download reaches 100%
    useEffect(() => {
        if (streamMode !== 'static' || isDirectDownload || staticReady) return;
        if (staticProgress < 100) return;

        const video = videoRef.current;
        if (!video || !serverUrl) return;

        setStaticReady(true);
        setInitialLoading(false);
        video.src = `${serverUrl}/stream/static/${infoHash}/${fileIndex}`;
        video.play().catch(() => {});
    }, [staticProgress, staticReady, streamMode, infoHash, fileIndex, serverUrl, isDirectDownload]);

    // 2. Time Update & Subtitle Rendering Logic
    useEffect(() => {
        const video = videoRef.current;
        if (!video) return;

        const onTimeUpdate = () => {
            if (isDraggingRef.current) return;
            const absTime = isDirectDownload || streamMode === 'static'
                ? video.currentTime
                : streamMode === 'hls'
                    ? hlsStartTimeRef.current + video.currentTime
                    : seekOffsetRef.current + video.currentTime;
            setCurrentTime(absTime);

            if (subtitleCues.length > 0) {
                const targetTime = absTime - subOffset;
                const activeCues = subtitleCues.filter(c => targetTime >= c.start && targetTime <= c.end);
                setCurrentActiveCues(activeCues);
            } else {
                setCurrentActiveCues([]);
            }

            // Capture browser buffered ranges (useful for HLS mode)
            if (video.buffered.length > 0) {
                const ranges: { start: number; end: number }[] = [];
                for (let i = 0; i < video.buffered.length; i++) {
                    ranges.push({ start: video.buffered.start(i), end: video.buffered.end(i) });
                }
                setHlsBufferedRanges(ranges);
            }
        };

        video.addEventListener('timeupdate', onTimeUpdate);
        return () => {
            video.removeEventListener('timeupdate', onTimeUpdate);
        };
    }, [streamMode, subtitleCues, subOffset, isDirectDownload]);

    const handleContainerClick = (e: React.MouseEvent) => {
        const target = e.target as HTMLElement;
        if (target.closest('button') || target.closest('input') || target.closest('[data-radix-popper-content-wrapper]')) {
            return;
        }
        if (streamMode === 'static' && !staticReady) return;
        togglePlay();
    }

    const handleSliderHover = (e: React.MouseEvent<HTMLDivElement>) => {
        const rect = e.currentTarget.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const percentage = Math.max(0, Math.min(1, x / rect.width));
        const time = percentage * duration;
        
        setHoverTime(time);
        setHoverPosition(percentage * 100);
    };

    // Reset timeline state when switching sources (torrent <-> direct, or different file)
    useEffect(() => {
        const frame = window.requestAnimationFrame(() => {
            setCurrentTime(0);
            setDuration(0);
            seekOffsetRef.current = 0;
            hlsStartTimeRef.current = 0;
            setBufferedRanges([]);
            setHlsBufferedRanges([]);
            setHoverTime(null);
            setHoverPosition(0);
            setLoading(true);
            setStaticProgress(0);
            setStaticReady(false);
        });

        return () => {
            window.cancelAnimationFrame(frame);
        };
    }, [infoHash, fileIndex]);

    return (
        <div
            ref={containerRef}
            className="group relative w-full h-dvh bg-black overflow-hidden flex items-center justify-center font-sans select-none"
            onMouseMove={handleMouseMove}
            onDoubleClick={handleDoubleClick}
            onClick={handleContainerClick}
        >
            <Toaster position="top-center" />

            <video
                ref={videoRef}
                className="w-full h-full object-contain"
                autoPlay
                crossOrigin="anonymous"
                onPlay={() => { setPlaying(true); setLoading(false); setInitialLoading(false); }}
                onPause={() => setPlaying(false)}
                onLoadedMetadata={syncDurationFromVideo}
                onDurationChange={syncDurationFromVideo}
                onWaiting={() => setLoading(true)}
                onPlaying={() => setLoading(false)}
                onEnded={() => setPlaying(false)}
            />

            {/* Play/Pause/Seek Feedback */}
            {feedback && (
                <div
                    key={feedback.id}
                    className={cn(
                        "absolute inset-0 flex items-center pointer-events-none z-40 px-20",
                        feedback.position === 'left' ? "justify-start" :
                            feedback.position === 'right' ? "justify-end" :
                                "justify-center"
                    )}>
                    <div className="bg-black/50 rounded-full p-5 animate-[ping_0.5s_ease-out_forwards]">
                        {feedback.type === 'play' && <Play size={48} fill="white" className="text-white ml-1" />}
                        {feedback.type === 'pause' && <Pause size={48} fill="white" className="text-white" />}
                        {feedback.type === 'forward' && (
                            <div className="flex flex-col items-center justify-center w-12 h-12">
                                <RotateCw size={32} className="text-white" />
                                <span className="text-white text-[10px] font-bold mt-1 leading-none">+{feedback.text}</span>
                            </div>
                        )}
                        {feedback.type === 'backward' && (
                            <div className="flex flex-col items-center justify-center w-12 h-12">
                                <RotateCcw size={32} className="text-white" />
                                <span className="text-white text-[10px] font-bold mt-1 leading-none">-{feedback.text}</span>
                            </div>
                        )}
                    </div>
                </div>
            )}

            {/* Custom Subtitle Overlay - Relative to Video Rect */}
            {currentActiveCues.length > 0 && (
                <div
                    className="absolute pointer-events-none"
                    style={{
                        top: videoRect.top,
                        left: videoRect.left,
                        width: videoRect.width,
                        height: videoRect.height,
                        zIndex: 30
                    }}
                >
                    {(() => {
                        const topCues = currentActiveCues.filter(c => c.position === 'top');
                        const middleCues = currentActiveCues.filter(c => c.position === 'middle');
                        const bottomCues = currentActiveCues.filter(c => c.position !== 'top' && c.position !== 'middle');
                        const groups: { cues: SubtitleCue[]; style: React.CSSProperties }[] = [];
                        if (topCues.length > 0) groups.push({ cues: topCues, style: { top: '5%' } });
                        if (middleCues.length > 0) groups.push({ cues: middleCues, style: { top: '45%', transform: 'translateY(-50%)' } });
                        if (bottomCues.length > 0) groups.push({ cues: bottomCues, style: { bottom: `${10 + (subPos * 0.5)}%` } });

                        return groups.map((group, gi) => (
                            <div
                                key={gi}
                                className="absolute w-full px-4 flex flex-col items-center justify-center pointer-events-none"
                                style={group.style}
                            >
                                {group.cues.map((cue, ci) => (
                                    <span
                                        key={ci}
                                        className="bg-black/50 text-white px-2 py-1 rounded inline-block whitespace-pre-wrap"
                                        style={{
                                            fontSize: `${Math.max(12, (videoRect.height * 0.045) * (subSize / 100))}px`,
                                            textShadow: '0 1px 2px rgba(0,0,0,1)'
                                        }}
                                    >
                                        {cue.text}
                                    </span>
                                ))}
                            </div>
                        ));
                    })()}
                </div>
            )}

            {initialLoading && (
                <div className="absolute inset-0 flex items-center justify-center pointer-events-none z-50 bg-black">
                    {streamMode === 'static' && !isDirectDownload ? (
                        <>
                            {torrentMeta?.background && (
                                <>
                                    <img
                                        src={torrentMeta.background}
                                        alt=""
                                        className="absolute inset-0 w-full h-full object-cover"
                                    />
                                    <div className="absolute inset-0 bg-black/80" />
                                </>
                            )}
                            <div className="relative z-10 flex flex-col items-center gap-4">
                                <div className="relative w-32 h-32">
                                    <svg className="w-full h-full -rotate-90" viewBox="0 0 120 120">
                                        <circle
                                            cx="60" cy="60" r="52"
                                            fill="none"
                                            stroke="rgba(255,255,255,0.1)"
                                            strokeWidth="6"
                                        />
                                        <circle
                                            cx="60" cy="60" r="52"
                                            fill="none"
                                            stroke="currentColor"
                                            strokeWidth="6"
                                            strokeLinecap="round"
                                            className="text-primary"
                                            strokeDasharray={`${2 * Math.PI * 52}`}
                                            strokeDashoffset={`${2 * Math.PI * 52 * (1 - staticProgress / 100)}`}
                                            style={{ transition: 'stroke-dashoffset 0.5s ease' }}
                                        />
                                    </svg>
                                    <div className="absolute inset-0 flex items-center justify-center">
                                        <span className="text-white text-2xl font-bold tabular-nums">
                                            {Math.floor(staticProgress)}%
                                        </span>
                                    </div>
                                </div>
                                <div className="flex flex-col items-center gap-1">
                                    {torrentMeta?.logo ? (
                                        <img
                                            src={torrentMeta.logo}
                                            alt=""
                                            className="max-w-[200px] max-h-[60px] object-contain mb-2"
                                        />
                                    ) : fileInfo?.name ? (
                                        <span className="text-white/80 text-sm font-medium max-w-xs text-center truncate">
                                            {fileInfo.name}
                                        </span>
                                    ) : null}
                                    <span className="text-white/50 text-xs">
                                        {fileInfo?.size ? (
                                            fileInfo.size >= 1024 * 1024 * 1024 
                                                ? `${(fileInfo.size / 1024 / 1024 / 1024).toFixed(2)} GB`
                                                : `${(fileInfo.size / 1024 / 1024).toFixed(1)} MB`
                                        ) : ''}
                                        {staticSpeed > 0 ? ` • ${(staticSpeed / 1024 / 1024).toFixed(1)} MB/s` : ''}
                                    </span>
                                </div>
                            </div>
                        </>
                    ) : (
                        <>
                            {torrentMeta?.background ? (
                                <>
                                    <img
                                        src={torrentMeta.background}
                                        alt=""
                                        className="absolute inset-0 w-full h-full object-cover"
                                    />
                                    <div className="absolute inset-0 bg-black/70" />
                                    <div className="relative z-10 flex flex-col items-center gap-6">
                                        {torrentMeta.logo ? (
                                            <img
                                                src={torrentMeta.logo}
                                                alt=""
                                                className="max-w-[280px] max-h-[120px] object-contain animate-pulse"
                                            />
                                        ) : (
                                            <div className="text-3xl font-black text-white/80 tracking-[0.3em] animate-pulse">STREAM</div>
                                        )}
                                    </div>
                                </>
                            ) : (
                                <div className="animate-spin rounded-full h-16 w-16 border-t-2 border-b-2 border-primary"></div>
                            )}
                        </>
                    )}
                </div>
            )}

            {!initialLoading && loading && playing && (
                <div className="absolute inset-0 flex items-center justify-center pointer-events-none z-[5]">
                    {torrentMeta?.logo ? (
                        <img
                            src={torrentMeta.logo}
                            alt=""
                            className="w-[140px] h-[60px] object-contain animate-pulse mx-auto my-0"
                        />
                    ) : (
                        <div className="animate-spin rounded-full h-10 w-10 border-t-2 border-b-2 border-white/60 mx-auto"></div>
                    )}
                </div>
            )}

            {/* Controls Container */}
            <div className={cn(
                "absolute inset-0 bg-gradient-to-t from-black/90 via-transparent to-black/60 transition-opacity duration-300 pointer-events-none z-10",
                showControls ? "opacity-100" : "opacity-0"
            )} />

            {/* Top Bar */}
            <div className={cn(
                "absolute top-0 left-0 right-0 p-6 transition-all duration-300 z-20",
                showControls ? "opacity-100 translate-y-0" : "opacity-0 -translate-y-4"
            )}>
                <button onClick={() => window.location.href = '/'} className="absolute top-6 left-6 flex items-center justify-center p-2 bg-white/10 rounded-full text-white/80 hover:text-white hover:bg-white/20 transition-colors pointer-events-auto">
                    <ArrowLeft size={20} />
                </button>
                <div className="pl-12">
                    <h1 className="font-medium text-lg text-left drop-shadow-md text-white">{torrentMeta?.title || fileInfo?.name || "Loading..."}</h1>
                    <p className="text-xs text-white/50 text-left">
                        {streamMode === 'hls' ? 'HLS' : streamMode === 'static' ? 'Static' : 'Direct'} • {fileInfo ? (
                            fileInfo.size >= 1024 * 1024 * 1024
                                ? `${(fileInfo.size / 1024 / 1024 / 1024).toFixed(2)} GB`
                                : `${(fileInfo.size / 1024 / 1024).toFixed(1)} MB`
                        ) : "Loading..."}
                    </p>
                </div>
            </div>

            {/* Bottom Controls */}
            <div className={cn(
                "absolute bottom-0 left-0 right-0 px-8 pb-8 pt-20 transition-all duration-300 z-20 flex flex-col gap-2",
                showControls ? "translate-y-0 opacity-100" : "translate-y-8 opacity-0"
            )}
                onClick={(e) => e.stopPropagation()}
            >

                {/* Progress Slider */}
                <div
                    className="flex items-center gap-4 group/slider relative pointer-events-auto"
                    onClick={(e) => e.stopPropagation()}
                >
                    <span className="text-white/90 text-xs font-mono w-16 text-right">{formatTime(currentTime)}</span>
                    <div
                        className="relative flex-1 h-1.5 bg-white/20 rounded-full cursor-pointer group-hover/slider:h-2.5 transition-all duration-200"
                        onMouseEnter={handleSliderHover}
                        onMouseMove={handleSliderHover}
                        onMouseLeave={() => setHoverTime(null)}
                    >
                        {/* Torrent Buffered Ranges */}
                        {bufferedRanges.map((range, idx) => (
                            <div
                                key={idx}
                                className="absolute top-0 h-full bg-white/40 rounded-full transition-all duration-300"
                                style={{
                                    left: `${(range.start / (duration || 1)) * 100}%`,
                                    width: `${Math.max(0, ((range.end - range.start) / (duration || 1)) * 100)}%`
                                }}
                            />
                        ))}

                        {/* Browser/HLS Buffered Ranges */}
                        {hlsBufferedRanges.map((range, idx) => (
                            <div
                                key={`hls-${idx}`}
                                className="absolute top-0 h-full bg-cyan-500/50 rounded-full transition-all duration-300"
                                style={{
                                    left: `${(range.start / (duration || 1)) * 100}%`,
                                    width: `${Math.max(0, ((range.end - range.start) / (duration || 1)) * 100)}%`
                                }}
                            />
                        ))}

                        <div
                            className="absolute h-full bg-primary rounded-full shadow-[0_0_10px_currentColor] text-primary/50 z-10"
                            style={{ width: `${(currentTime / (duration || 1)) * 100}%` }}
                        />
                        <div
                            className="absolute top-1/2 -translate-y-1/2 w-3.5 h-3.5 bg-white rounded-full shadow-lg scale-0 group-hover/slider:scale-100 transition-transform duration-200"
                            style={{ left: `${(currentTime / (duration || 1)) * 100}%` }}
                        />
                        <input
                            type="range"
                            min={0}
                            max={duration || 100}
                            value={currentTime}
                            onMouseDown={() => { isDraggingRef.current = true; }}
                            onTouchStart={() => { isDraggingRef.current = true; }}
                            onChange={(e) => setCurrentTime(parseFloat(e.target.value))}
                            onMouseUp={(e) => {
                                isDraggingRef.current = false;
                                handleSeek(parseFloat((e.target as HTMLInputElement).value));
                            }}
                            onTouchEnd={(e) => {
                                isDraggingRef.current = false;
                                handleSeek(parseFloat((e.target as HTMLInputElement).value));
                            }}
                            className="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-20"
                        />
                        
                        {/* Hover Timestamp Tooltip */}
                        {hoverTime !== null && (
                            <div
                                className="absolute bottom-full mb-2 -translate-x-1/2 bg-black/90 text-white text-xs font-mono px-2 py-1 rounded pointer-events-none transition-opacity duration-150"
                                style={{ left: `${hoverPosition}%` }}
                            >
                                {formatTime(hoverTime)}
                            </div>
                        )}
                    </div>

                    <span className="text-white/90 text-xs font-mono w-16">{formatTime(duration)}</span>
                </div>

                {/* Buttons */}
                <div className="flex items-center justify-between mt-2 pointer-events-auto">
                    <div className="flex items-center gap-6">
                        <button
                            onClick={togglePlay}
                            className="text-white hover:text-primary transition-transform hover:scale-110 p-3 bg-white/5 rounded-full backdrop-blur-sm"
                        >
                            {playing ? <Pause size={32} fill="currentColor" /> : <Play size={32} fill="currentColor" className="ml-1" />}
                        </button>

                        <div className="flex items-center gap-2 group/vol">
                            <button onClick={() => setMuted(!muted)} className="text-white/70 hover:text-white">
                                {muted ? <VolumeX size={20} /> : <Volume2 size={20} />}
                            </button>
                            <div className="w-0 overflow-hidden group-hover/vol:w-20 transition-all duration-300">
                                <input
                                    type="range" min="0" max="1" step="0.1"
                                    value={muted ? 0 : volume}
                                    onChange={(e) => {
                                        const v = parseFloat(e.target.value);
                                        setVolume(v);
                                        if (videoRef.current) videoRef.current.volume = v;
                                        setMuted(v === 0);
                                    }}
                                    className="w-full h-1 bg-white/30 rounded-full accent-white cursor-pointer"
                                />
                            </div>
                        </div>
                    </div>

                    <div className="flex items-center gap-4">
                        <>
                            <SubtitlePopover
                                containerRef={containerRef}
                                subtitleCues={subtitleCues}
                                setSubtitleCues={setSubtitleCues}
                                embeddedSubs={embeddedSubs}
                                selectedSubId={selectedSubId}
                                setSelectedSubId={setSelectedSubId}
                                setCurrentSubLink={setCurrentSubLink}
                                infoHash={infoHash}
                                fileIndex={fileIndex}
                                serverUrl={serverUrl}
                                setSubOffset={setSubOffset}
                                initialQuery={(torrentMeta?.title ?? fileInfo?.name ?? "").replace(/\./g, " ")}
                            />

                            <SettingsPopover
                                containerRef={containerRef}
                                streamMode={streamMode}
                                setStreamMode={setStreamMode}
                                subOffset={subOffset}
                                setSubOffset={setSubOffset}
                                subSize={subSize}
                                setSubSize={setSubSize}
                                subPos={subPos}
                                setSubPos={setSubPos}
                                currentSubLink={currentSubLink}
                                infoHash={infoHash}
                                fileIndex={fileIndex}
                                serverUrl={serverUrl}
                                currentTime={currentTime}
                            />
                        </>

                        <button onClick={toggleFullscreen} className="text-white/70 hover:text-white pointer-events-auto">
                            {fullscreen ? <Minimize size={24} /> : <Maximize size={24} />}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}
