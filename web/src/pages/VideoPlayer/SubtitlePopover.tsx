import { useState, useEffect, useRef } from "react";
import { Captions, Search, Loader2 } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

type SubtitleCue = {
    start: number;
    end: number;
    text: string;
};

type Subtitle = {
    IDMovie: string;
    IDSubtitleFile: string;
    MovieName: string;
    SubFileName: string;
    LanguageName: string;
    ZipDownloadLink: string;
    SubDownloadLink: string;
};

type EmbeddedSubtitle = {
    index: number;
    language: string;
    title: string;
    codec: string;
};

type SubtitlePopoverProps = {
    containerRef: React.RefObject<HTMLDivElement | null>;
    subtitleCues: SubtitleCue[];
    setSubtitleCues: (cues: SubtitleCue[]) => void;
    embeddedSubs: EmbeddedSubtitle[];
    selectedSubId: string | null;
    setSelectedSubId: (id: string | null) => void;
    setCurrentSubLink: (link: string | null) => void;
    infoHash: string;
    fileIndex: number;
    serverUrl: string | null;
    setSubOffset: (offset: number) => void;
    initialQuery?: string;
};

export default function SubtitlePopover({
    containerRef,
    subtitleCues,
    setSubtitleCues,
    embeddedSubs,
    selectedSubId,
    setSelectedSubId,
    setCurrentSubLink,
    infoHash,
    fileIndex,
    serverUrl,
    setSubOffset,
    initialQuery = ""
}: SubtitlePopoverProps) {
    const [subLang, setSubLang] = useState("eng");
    const [searchingSubs, setSearchingSubs] = useState(false);
    const [subtitles, setSubtitles] = useState<Subtitle[]>([]);
    const [showLangPopover, setShowLangPopover] = useState(false);
    const [isLoadingSubtitle, setIsLoadingSubtitle] = useState(false);
    const [popoverOpen, setPopoverOpen] = useState(false);
    const [searchInput, setSearchInput] = useState(initialQuery);
    const inputRef = useRef<HTMLInputElement>(null);

    const searchSubtitles = async () => {
        if (!searchInput || !serverUrl) return;
        setSearchingSubs(true);
        try {
            const res = await fetch(`${serverUrl}/api/subtitles/search?query=${encodeURIComponent(searchInput)}&lang=${subLang}`);
            const data = await res.json();
            setSubtitles(data || []);
        } catch (e) {
            console.error(e);
            toast.error("Failed to search subtitles");
        } finally {
            setSearchingSubs(false);
        }
    };

    const selectSubtitle = async (link: string, id: string) => {
        if (!serverUrl) return;
        setSelectedSubId(id);
        setCurrentSubLink(link);
        setSubOffset(0);
        try {
            const res = await fetch(`${serverUrl}/api/subtitles/download?link=${encodeURIComponent(link)}`);
            const data = await res.json();
            if (Array.isArray(data)) {
                setSubtitleCues(data);
                toast.success("Subtitle loaded successfully");
            } else {
                console.error("Invalid subtitle format received");
                setSubtitleCues([]);
                toast.error("Invalid subtitle format");
            }
        } catch (e) {
            console.error(e);
            setSubtitleCues([]);
            toast.error("Failed to download subtitle");
        }
    };

    const selectEmbeddedSubtitle = async (streamIndex: number) => {
        if (!serverUrl) return;
        setIsLoadingSubtitle(true);
        setSelectedSubId(`embedded-${streamIndex}`);
        setCurrentSubLink(null);
        setSubOffset(0);

        try {
            const res = await fetch(`${serverUrl}/api/stream/${infoHash}/${fileIndex}/sub/${streamIndex}`);
            const data = await res.json();
            if (Array.isArray(data)) {
                setSubtitleCues(data);
                toast.success("Embedded subtitle loaded");
            } else {
                setSubtitleCues([]);
                toast.error("Failed to load embedded subtitle");
            }
        } catch (e) {
            console.error(e);
            toast.error("Failed to load embedded subtitle");
        } finally {
            setIsLoadingSubtitle(false);
        }
    };

    useEffect(() => {
        if (popoverOpen) {
            setSearchInput(initialQuery);
        }
    }, [popoverOpen, initialQuery]);

    return (
        <Popover modal={true} open={popoverOpen} onOpenChange={setPopoverOpen}>
            <PopoverTrigger asChild>
                <button
                    className={cn("transition-transform hover:scale-110", subtitleCues.length > 0 ? "text-primary/80" : "text-white/70 hover:text-white")}
                    title="Subtitles"
                >
                    <Captions size={24} />
                </button>
            </PopoverTrigger>
            <PopoverContent
                container={containerRef.current || undefined}
                className="w-80 p-4 bg-black/90 border-white/10 backdrop-blur-md text-white"
                side="top"
                align="end"
                onOpenAutoFocus={(e) => e.preventDefault()}
            >
                <div className="flex flex-col gap-4">
                    <div>
                        <h3 className="font-bold mb-2 flex items-center gap-2"><Captions size={18} /> Subtitles</h3>
                        <div className="flex gap-2">
                            <input
                                ref={inputRef}
                                className="bg-white/10 border-none rounded px-2 py-1 flex-1 text-sm text-white focus:outline-none focus:ring-1 focus:ring-primary"
                                value={searchInput}
                                onChange={e => setSearchInput(e.target.value)}
                                placeholder="Search..."
                                onKeyDown={e => e.key === 'Enter' && searchSubtitles()}
                                tabIndex={-1}
                            />

                            <div className="relative">
                                <Popover open={showLangPopover} onOpenChange={setShowLangPopover}>
                                    <PopoverTrigger asChild>
                                        <button
                                            className="px-2 py-1 bg-white/10 rounded hover:bg-white/20 text-lg flex items-center justify-center min-w-[36px]"
                                            title="Select Language"
                                        >
                                            {subLang === 'eng' ? '🇬🇧' : '🇮🇩'}
                                        </button>
                                    </PopoverTrigger>
                                    <PopoverContent container={containerRef.current || undefined} className="w-auto p-1 bg-black/95 border-white/10" side="top" align="end">
                                        <div className="flex flex-col gap-1">
                                            <button
                                                onClick={() => { setSubLang('eng'); setShowLangPopover(false); }}
                                                className={cn("flex items-center gap-2 px-3 py-1.5 rounded text-sm hover:bg-white/10", subLang === 'eng' && "bg-primary text-primary-foreground")}
                                            >
                                                🇬🇧 English
                                            </button>
                                            <button
                                                onClick={() => { setSubLang('ind'); setShowLangPopover(false); }}
                                                className={cn("flex items-center gap-2 px-3 py-1.5 rounded text-sm hover:bg-white/10", subLang === 'ind' && "bg-primary text-primary-foreground")}
                                            >
                                                🇮🇩 Indonesia
                                            </button>
                                        </div>
                                    </PopoverContent>
                                </Popover>
                            </div>

                            <button onClick={searchSubtitles} className="p-1 bg-primary rounded hover:bg-primary/80 text-primary-foreground">
                                <Search size={16} />
                            </button>
                        </div>
                    </div>

                    <div className="flex-1 overflow-y-auto max-h-[250px] min-h-[150px] border border-white/5 rounded p-1 bg-white/5 custom-scrollbar">
                        {embeddedSubs.length > 0 && (
                            <>
                                <div className="text-[10px] uppercase text-white/30 px-2 py-1 font-bold">Embedded</div>
                                {embeddedSubs.map(s => (
                                    <button
                                        key={`embedded-${s.index}`}
                                        onClick={() => selectEmbeddedSubtitle(s.index)}
                                        disabled={isLoadingSubtitle}
                                        className={cn(
                                            "block w-full text-left text-xs p-2 rounded truncate flex items-center justify-between gap-2 mb-1",
                                            selectedSubId === `embedded-${s.index}` ? "bg-primary text-primary-foreground" : "hover:bg-white/10 text-white/90",
                                            isLoadingSubtitle && "opacity-50 cursor-not-allowed"
                                        )}
                                    >
                                        <span className="truncate flex-1" title={s.title || `Track ${s.index}`}>{s.title || `Track ${s.index}`}</span>
                                        {isLoadingSubtitle && selectedSubId === `embedded-${s.index}` ? (
                                            <Loader2 size={12} className="animate-spin text-white/90 shrink-0" />
                                        ) : (
                                            <span className="text-[10px] uppercase bg-white/10 px-1 rounded text-white/70 shrink-0">{s.language || 'UNK'}</span>
                                        )}
                                    </button>
                                ))}
                                {subtitles.length > 0 && <div className="h-px bg-white/5 my-2 mx-1" />}
                            </>
                        )}

                        {searchingSubs ? (
                            <div className="text-center text-xs py-4 text-white/70">Searching online...</div>
                        ) : (
                            <>
                                {subtitles.length > 0 && embeddedSubs.length > 0 && (
                                    <div className="text-[10px] uppercase text-white/30 px-2 py-1 font-bold">OpenSubtitles</div>
                                )}

                                {subtitles.map(s => (
                                    <button
                                        key={s.IDSubtitleFile}
                                        onClick={() => selectSubtitle(s.SubDownloadLink, s.IDSubtitleFile)}
                                        className={cn(
                                            "block w-full text-left text-xs p-2 rounded truncate flex items-center justify-between gap-2 mb-1",
                                            selectedSubId === s.IDSubtitleFile ? "bg-primary text-primary-foreground" : "hover:bg-white/10 text-white/90"
                                        )}
                                    >
                                        <span className="truncate flex-1" title={s.SubFileName}>{s.SubFileName}</span>
                                        <span className="text-[10px] uppercase bg-white/10 px-1 rounded text-white/70 shrink-0">{s.LanguageName}</span>
                                    </button>
                                ))}

                                {subtitles.length === 0 && embeddedSubs.length === 0 && (
                                    <div className="text-center text-xs text-white/50 py-4">No subtitles found</div>
                                )}
                            </>
                        )}
                    </div>
                </div>
            </PopoverContent>
        </Popover>
    );
}