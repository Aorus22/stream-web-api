import { useState } from "react";
import { Settings, Type, ArrowUp, Loader2, Copy } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

type SettingsPopoverProps = {
    containerRef: React.RefObject<HTMLDivElement | null>;
    streamMode: 'direct' | 'hls' | 'static';
    setStreamMode: (mode: 'direct' | 'hls' | 'static') => void;
    subOffset: number;
    setSubOffset: (offset: number) => void;
    subSize: number;
    setSubSize: (size: number) => void;
    subPos: number;
    setSubPos: (pos: number) => void;
    currentSubLink: string | null;
    infoHash: string;
    fileIndex: number;
    serverUrl: string | null;
    currentTime: number;
};

export default function SettingsPopover({
    containerRef,
    streamMode,
    setStreamMode,
    subOffset,
    setSubOffset,
    subSize,
    setSubSize,
    subPos,
    setSubPos,
    currentSubLink,
    infoHash,
    fileIndex,
    serverUrl,
    currentTime
}: SettingsPopoverProps) {
    const [isAutoSyncing, setIsAutoSyncing] = useState(false);

    const handleAutoSync = async () => {
        if (!currentSubLink || !infoHash || !serverUrl) return;
        setIsAutoSyncing(true);
        try {
            const res = await fetch(`${serverUrl}/api/subtitles/autosync?link=${encodeURIComponent(currentSubLink)}&infoHash=${infoHash}&fileIndex=${fileIndex}&currentTime=${currentTime}`);
            const data = await res.json();
            if (data && typeof data.offset === 'number') {
                setSubOffset(data.offset);
                toast.success(`Synced: ${data.offset >= 0 ? '+' : ''}${data.offset.toFixed(2)}s`);
            }
        } catch (e) {
            console.error(e);
            toast.error('Auto Sync Failed');
        } finally {
            setIsAutoSyncing(false);
        }
    };

    return (
        <Popover modal={true}>
            <PopoverTrigger asChild>
                <button
                    className="text-white/70 hover:text-white transition-transform hover:scale-110"
                    title="Settings"
                >
                    <Settings size={24} />
                </button>
            </PopoverTrigger>
            <PopoverContent container={containerRef.current || undefined} className="w-64 p-4 bg-black/90 border-white/10 backdrop-blur-md text-white" side="top" align="end">
                <div className="flex flex-col gap-3">
                    <div className="flex items-center justify-between">
                        <h3 className="font-bold flex items-center gap-2"><Settings size={18} /> Settings</h3>
                        {streamMode === 'static' && (
                            <button
                                onClick={() => {
                                    const url = `${serverUrl}/stream/static/${infoHash}/${fileIndex}`;
                                    navigator.clipboard.writeText(url);
                                    toast.success("Static link copied to clipboard");
                                }}
                                className="p-1 hover:bg-white/10 rounded-md transition-colors group"
                                title="Copy Direct Link"
                            >
                                <Copy size={16} className="text-white/70 group-hover:text-white" />
                            </button>
                        )}
                    </div>

                    <div className="space-y-2">
                        <span className="text-xs text-white/70">Stream Mode</span>
                        <div className="flex rounded-lg overflow-hidden border border-white/10">
                            <button
                                onClick={() => setStreamMode('direct')}
                                className={cn(
                                    "flex-1 px-3 py-2 text-xs font-medium transition-colors",
                                    streamMode === 'direct'
                                        ? "bg-primary text-primary-foreground"
                                        : "bg-white/5 text-white/60 hover:bg-white/10"
                                )}
                            >
                                Direct
                            </button>
                            <button
                                onClick={() => setStreamMode('hls')}
                                className={cn(
                                    "flex-1 px-3 py-2 text-xs font-medium transition-colors",
                                    streamMode === 'hls'
                                        ? "bg-primary text-primary-foreground"
                                        : "bg-white/5 text-white/60 hover:bg-white/10"
                                )}
                            >
                                HLS
                            </button>
                            <button
                                onClick={() => setStreamMode('static')}
                                className={cn(
                                    "flex-1 px-3 py-2 text-xs font-medium transition-colors",
                                    streamMode === 'static'
                                        ? "bg-primary text-primary-foreground"
                                        : "bg-white/5 text-white/60 hover:bg-white/10"
                                )}
                            >
                                Static
                            </button>
                        </div>
                        <p className="text-[10px] text-white/40 leading-tight">
                            {streamMode === 'direct'
                                ? "Fast startup, no video re-encoding. Seek creates new stream."
                                : streamMode === 'hls'
                                ? "Slower startup (transcodes segments), but seeking within buffered range is instant."
                                : "Downloads entire file first, then plays natively without transcoding."
                            }
                        </p>
                    </div>

                    <div className="space-y-3 pt-3 border-t border-white/10">
                        <h4 className="text-xs font-bold text-white/80">Subtitle</h4>

                        <div className="flex items-center justify-between text-xs">
                            <span className="text-white/70">Offset: {subOffset >= 0 ? '+' : ''}{subOffset.toFixed(1)}s</span>
                            <div className="flex gap-1 items-center">
                                <button
                                    onClick={handleAutoSync}
                                    disabled={!currentSubLink || isAutoSyncing}
                                    className={cn(
                                        "px-2 py-1 rounded text-xs transition-colors flex items-center gap-1",
                                        isAutoSyncing ? "bg-primary/30 text-primary-foreground/70" : "bg-primary text-primary-foreground hover:bg-primary/90"
                                    )}
                                    title="Auto Sync with Audio"
                                >
                                    {isAutoSyncing ? <Loader2 size={12} className="animate-spin" /> : "Sync"}
                                </button>
                                <button onClick={() => setSubOffset(subOffset - 0.5)} className="px-2 py-1 bg-white/10 rounded hover:bg-white/20 text-white">-</button>
                                <button onClick={() => setSubOffset(subOffset + 0.5)} className="px-2 py-1 bg-white/10 rounded hover:bg-white/20 text-white">+</button>
                            </div>
                        </div>

                        <div className="space-y-1">
                            <div className="flex items-center gap-2 text-xs text-white/80">
                                <Type size={14} /> <span>Size</span>
                            </div>
                            <input
                                type="range" min="50" max="200" step="10"
                                value={subSize} onChange={e => setSubSize(Number(e.target.value))}
                                className="w-full h-1 bg-white/20 rounded-lg appearance-none cursor-pointer accent-primary"
                            />
                        </div>

                        <div className="space-y-1">
                            <div className="flex items-center gap-2 text-xs text-white/80">
                                <ArrowUp size={14} /> <span>Position (0 = Bottom)</span>
                            </div>
                            <div className="relative w-full h-4 flex items-center">
                                <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 w-[1px] h-3 bg-white/40 z-0 pointer-events-none" />
                                <input
                                    type="range" min="-100" max="100" step="1"
                                    value={subPos} onChange={e => setSubPos(Number(e.target.value))}
                                    className="w-full h-1 bg-white/20 rounded-lg appearance-none cursor-pointer accent-primary z-10 relative"
                                />
                            </div>
                        </div>
                    </div>
                </div>
            </PopoverContent>
        </Popover>
    );
}