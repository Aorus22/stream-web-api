import { useState } from "react";
import { Play, Pause, Trash2, Copy, Download, RotateCw, FileVideo, Cloud, Loader2, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { useServer } from "@/contexts/ServerContext";
import { cn } from "@/lib/utils";
import { isCompleted } from "./utils";
import type { LibraryItem } from "./types";

type Props = {
    item: LibraryItem;
    onCopy: (text: string) => void;
    onRemove: (id: string, type: LibraryItem["type"]) => void;
    onDeleteCache?: (infoHash: string) => void;
    onReencode: (options: { infoHash?: string; fileIndex?: number; downloadId?: number; resolution: string; bitrate: string }) => Promise<boolean>;
    onMoveToDrive: (options: { infoHash?: string; fileIndex?: number; downloadId?: number; exportPath?: string }) => void;
};

const STATUS_COLORS: Record<string, string> = {
    downloading: "bg-white/15 text-white",
    completed: "bg-emerald-500/15 text-emerald-400",
    failed: "bg-red-500/20 text-red-400",
    missing: "bg-amber-500/20 text-amber-400",
    orphan: "bg-amber-500/20 text-amber-400",
    on_demand: "bg-cyan-500/20 text-cyan-400",
    paused: "bg-white/10 text-white/70",
    seeding: "bg-emerald-500/15 text-emerald-400",
    streaming: "bg-white/15 text-white",
};

export default function DetailPane({ item, onCopy, onRemove, onDeleteCache, onReencode, onMoveToDrive }: Props) {
    const { serverUrl } = useServer();
    const [reencodeLoading, setReencodeLoading] = useState<string | null>(null);

    const completed = isCompleted(item.progress);
    const statusLower = item.status.toLowerCase();
    const statusClass = STATUS_COLORS[statusLower] ?? "bg-white/10 text-white";

    const playFile = (file: LibraryItem["files"][number]) => {
        if (file.downloadId) {
            window.location.href = `/watch?directId=${file.downloadId}`;
        } else if (file.infoHash && file.fileIndex !== undefined) {
            window.location.href = `/watch?infoHash=${file.infoHash}&fileIndex=${file.fileIndex}`;
        }
    };

    const downloadFile = (file: LibraryItem["files"][number]) => {
        if (!serverUrl) return;
        let url = "";
        if (file.downloadId) {
            url = `${serverUrl}/stream/direct/${file.downloadId}?download=true`;
        } else if (file.infoHash && file.fileIndex !== undefined) {
            url = `${serverUrl}/stream/${file.infoHash}/${file.fileIndex}?download=true`;
        }
        if (url) window.open(url, "_blank");
    };

    const copyFileUrl = (file: LibraryItem["files"][number]) => {
        if (!serverUrl) return;
        let url = "";
        if (file.downloadId) {
            url = `${serverUrl}/stream/direct/${file.downloadId}`;
        } else if (file.infoHash && file.fileIndex !== undefined) {
            url = `${serverUrl}/stream/${file.infoHash}/${file.fileIndex}`;
        }
        if (url) onCopy(url);
    };

    const handleReencode = async (file: LibraryItem["files"][number]) => {
        const key = `${file.infoHash ?? ""}-${file.fileIndex ?? file.downloadId ?? ""}`;
        setReencodeLoading(key);
        try {
            await onReencode({
                infoHash: file.infoHash,
                fileIndex: file.fileIndex,
                downloadId: file.downloadId,
                resolution: "1080p",
                bitrate: "5000k",
            });
        } finally {
            setReencodeLoading(null);
        }
    };

    const moveToDrive = (file: LibraryItem["files"][number]) => {
        onMoveToDrive({
            infoHash: file.infoHash,
            fileIndex: file.fileIndex,
            downloadId: file.downloadId,
        });
    };

    const isDirect = item.type === "direct";
    const isTorrent = item.type === "torrent";
    const isCached = item.type === "cached";
    const canRemove = isTorrent || isDirect;

    const playUrl =
        isDirect && (item.raw as { id?: number }).id
            ? `/watch?directId=${(item.raw as { id: number }).id}`
            : isTorrent && item.files[0]?.infoHash && item.files[0]?.fileIndex !== undefined
                ? `/watch?infoHash=${item.files[0].infoHash}&fileIndex=${item.files[0].fileIndex}`
                : null;

    return (
        <div className="flex h-full w-full animate-in flex-col overflow-x-hidden overflow-y-auto fade-in duration-500">
            <div className="mb-8 space-y-5">
                <span
                    className={cn(
                        "inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[11px] font-extrabold uppercase tracking-[0.15em]",
                        statusClass
                    )}
                >
                    {statusLower === "failed" || statusLower === "missing" ? (
                        <AlertCircle className="h-3 w-3" />
                    ) : null}
                    {item.status}
                </span>

                <h1 className="bg-gradient-to-b from-white to-white/55 bg-clip-text text-3xl font-black leading-[1.1] tracking-tight text-transparent lg:text-4xl">
                    {item.name}
                </h1>

                <p className="max-w-2xl break-all text-[14px] leading-relaxed text-muted-foreground">{item.full}</p>

                <div className="flex flex-wrap gap-3 pt-2">
                    {playUrl && (
                        <Button
                            asChild
                            className="h-12 gap-2 rounded-full bg-white px-7 text-sm font-bold text-black shadow-lg shadow-white/10 transition-all hover:scale-[1.02] hover:bg-white hover:shadow-white/20"
                        >
                            <a href={playUrl}>
                                <Play className="h-4 w-4 fill-current" />
                                {completed ? "Play Now" : "Stream While Downloading"}
                            </a>
                        </Button>
                    )}

                    {canRemove && (
                        <Button
                            variant="outline"
                            className="h-12 gap-2 rounded-full border-white/10 bg-white/[0.04] px-6 text-sm font-semibold text-white backdrop-blur-md transition-all hover:bg-white/[0.1]"
                            onClick={() => onRemove(item.id, item.type)}
                        >
                            <Pause className="h-4 w-4" />
                            {statusLower === "downloading" || statusLower === "streaming" ? "Pause" : "Stop"}
                        </Button>
                    )}

                    {isTorrent && (item.raw as { magnetUri?: string }).magnetUri && (
                        <Button
                            variant="outline"
                            className="h-12 gap-2 rounded-full border-white/10 bg-white/[0.04] px-6 text-sm font-semibold text-white backdrop-blur-md transition-all hover:bg-white/[0.1]"
                            onClick={() => onCopy((item.raw as { magnetUri: string }).magnetUri)}
                        >
                            <Copy className="h-4 w-4" />
                            Copy Magnet
                        </Button>
                    )}

                    {canRemove && (
                        <AlertDialog>
                            <AlertDialogTrigger asChild>
                                <Button
                                    variant="outline"
                                    size="icon"
                                    className="h-12 w-12 rounded-full border-white/10 bg-white/[0.04] text-white backdrop-blur-md transition-all hover:bg-red-500/15 hover:text-red-400"
                                >
                                    <Trash2 className="h-4 w-4" />
                                </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent>
                                <AlertDialogHeader>
                                    <AlertDialogTitle>Remove from Library?</AlertDialogTitle>
                                    <AlertDialogDescription>
                                        This will stop the active transfer and remove it from the list.
                                    </AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                                    <AlertDialogAction
                                        onClick={() => onRemove(item.id, item.type)}
                                        className="bg-red-500 text-white hover:bg-red-500/90"
                                    >
                                        Remove
                                    </AlertDialogAction>
                                </AlertDialogFooter>
                            </AlertDialogContent>
                        </AlertDialog>
                    )}

                    {isCached && onDeleteCache && item.files[0]?.infoHash && (
                        <AlertDialog>
                            <AlertDialogTrigger asChild>
                                <Button
                                    variant="outline"
                                    size="icon"
                                    className="h-12 w-12 rounded-full border-white/10 bg-white/[0.04] text-white backdrop-blur-md transition-all hover:bg-red-500/15 hover:text-red-400"
                                >
                                    <Trash2 className="h-4 w-4" />
                                </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent>
                                <AlertDialogHeader>
                                    <AlertDialogTitle>Delete from Library?</AlertDialogTitle>
                                    <AlertDialogDescription>
                                        This will permanently delete all cached files for this content.
                                    </AlertDialogDescription>
                                </AlertDialogHeader>
                                <AlertDialogFooter>
                                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                                    <AlertDialogAction
                                        onClick={() => onDeleteCache(item.files[0]!.infoHash!)}
                                        className="bg-red-500 text-white hover:bg-red-500/90"
                                    >
                                        Delete
                                    </AlertDialogAction>
                                </AlertDialogFooter>
                            </AlertDialogContent>
                        </AlertDialog>
                    )}
                </div>
            </div>

            <div className="flex-1 pb-12">
                <div className="mb-6 flex items-center justify-between text-[12px] font-bold uppercase tracking-[0.18em] text-muted-foreground">
                    <span>Inside the Box · {item.files.length} file{item.files.length !== 1 ? "s" : ""}</span>
                    <span>Size</span>
                </div>

                <div className="flex flex-col gap-2">
                    {item.files.map((file, i) => {
                        const fileCompleted = (file.progress ?? 0) >= 100;
                        const fileSize = file.size;
                        const reencodeKey = `${file.infoHash ?? ""}-${file.fileIndex ?? file.downloadId ?? ""}`;
                        const reencodeBusy = reencodeLoading === reencodeKey;

                        return (
                            <div
                                key={`${file.name}-${i}`}
                                className="group flex items-center gap-4 rounded-xl border border-white/[0.06] bg-white/[0.02] p-4 transition-all hover:border-white/15 hover:bg-white/[0.05]"
                            >
                                <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-white/5 text-white/60">
                                    <FileVideo className="h-4 w-4" />
                                </div>

                                <div className="min-w-0 flex-1">
                                    <div className="truncate text-sm font-semibold transition-colors group-hover:text-white">
                                        {file.name}
                                    </div>
                                    <div className="mt-0.5 font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
                                        {fileSize}
                                    </div>
                                </div>

                                <div className="font-mono text-[11px] font-semibold">
                                    {fileCompleted ? (
                                        <span className="text-emerald-400">100%</span>
                                    ) : (
                                        <span style={{ color: item.accent }}>
                                            {file.progress?.toFixed(0) ?? 0}%
                                        </span>
                                    )}
                                </div>

                                <div className="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                                    {fileCompleted && (file.infoHash || file.downloadId) && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <Button
                                                    variant="ghost"
                                                    size="icon-sm"
                                                    className="text-muted-foreground hover:text-white"
                                                    onClick={() => handleReencode(file)}
                                                    disabled={reencodeBusy}
                                                >
                                                    {reencodeBusy ? (
                                                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                                                    ) : (
                                                        <RotateCw className="h-3.5 w-3.5" />
                                                    )}
                                                </Button>
                                            </TooltipTrigger>
                                            <TooltipContent>Reencode to MP4</TooltipContent>
                                        </Tooltip>
                                    )}

                                    {(file.infoHash || file.downloadId) && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <Button
                                                    variant="ghost"
                                                    size="icon-sm"
                                                    className="text-muted-foreground hover:text-white"
                                                    onClick={() => copyFileUrl(file)}
                                                >
                                                    <Copy className="h-3.5 w-3.5" />
                                                </Button>
                                            </TooltipTrigger>
                                            <TooltipContent>Copy Stream URL</TooltipContent>
                                        </Tooltip>
                                    )}

                                    {(file.infoHash || file.downloadId) && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <Button
                                                    variant="ghost"
                                                    size="icon-sm"
                                                    className="text-muted-foreground hover:text-white"
                                                    onClick={() => moveToDrive(file)}
                                                >
                                                    <Cloud className="h-3.5 w-3.5" />
                                                </Button>
                                            </TooltipTrigger>
                                            <TooltipContent>Move to Drive</TooltipContent>
                                        </Tooltip>
                                    )}

                                    {fileCompleted && (file.infoHash || file.downloadId) && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <Button
                                                    variant="ghost"
                                                    size="icon-sm"
                                                    className="text-muted-foreground hover:text-white"
                                                    onClick={() => downloadFile(file)}
                                                >
                                                    <Download className="h-3.5 w-3.5" />
                                                </Button>
                                            </TooltipTrigger>
                                            <TooltipContent>Download</TooltipContent>
                                        </Tooltip>
                                    )}

                                    {file.canPlay && (file.infoHash || file.downloadId) && (
                                        <Tooltip>
                                            <TooltipTrigger asChild>
                                                <Button
                                                    size="icon-sm"
                                                    className="ml-1 bg-white text-black shadow-md shadow-white/10 hover:bg-white/90"
                                                    onClick={() => playFile(file)}
                                                >
                                                    <Play className="h-3.5 w-3.5 fill-current" />
                                                </Button>
                                            </TooltipTrigger>
                                            <TooltipContent>Play</TooltipContent>
                                        </Tooltip>
                                    )}
                                </div>
                            </div>
        );
    })}
                </div>
            </div>
        </div>
    );
}
