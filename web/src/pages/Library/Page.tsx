import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { Plus, RefreshCw, Activity, Library as LibraryIcon, Wifi, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Empty, EmptyMedia, EmptyTitle, EmptyDescription } from "@/components/ui/empty";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { useServer } from "@/contexts/ServerContext";
import { cn } from "@/lib/utils";
import QueueCard from "./QueueCard";
import DetailPane from "./DetailPane";
import { formatBytes, formatSpeed, getBackdrop, getAccent } from "./utils";
import type { LibraryItem, LibraryItemType, LibraryFile } from "./types";
import type { Torrent, CachedFile, DirectDownload } from "./types";

type Tab = "all" | "torrent" | "direct" | "cached";

export function Library() {
    const { serverUrl } = useServer();
    const [torrents, setTorrents] = useState<Torrent[]>([]);
    const [directDownloads, setDirectDownloads] = useState<DirectDownload[]>([]);
    const [cachedFiles, setCachedFiles] = useState<CachedFile[]>([]);
    const [selectedId, setSelectedId] = useState<string | null>(null);
    const [tab, setTab] = useState<Tab>("all");
    const [search, setSearch] = useState("");
    const [isInitialLoading, setIsInitialLoading] = useState(true);
    const [refreshing, setRefreshing] = useState(false);
    const [showAddForm, setShowAddForm] = useState(false);
    const [addType, setAddType] = useState<"magnet" | "direct">("magnet");
    const [magnet, setMagnet] = useState("");
    const [directUrl, setDirectUrl] = useState("");
    const [addError, setAddError] = useState("");
    const [addLoading, setAddLoading] = useState(false);
    const containerRef = useRef<HTMLDivElement>(null);

    const fetchTorrents = useCallback(async () => {
        if (!serverUrl) return;
        try {
            const res = await fetch(`${serverUrl}/api/torrents`);
            const data = await res.json();
            setTorrents(data || []);
        } catch (err) {
            console.error(err);
        }
    }, [serverUrl]);

    const fetchDirect = useCallback(async () => {
        if (!serverUrl) return;
        try {
            const res = await fetch(`${serverUrl}/api/direct`);
            const data = await res.json();
            setDirectDownloads(data || []);
        } catch (err) {
            console.error("Failed to fetch direct downloads:", err);
        }
    }, [serverUrl]);

    const fetchCache = useCallback(async () => {
        if (!serverUrl) return;
        try {
            const res = await fetch(`${serverUrl}/api/cache`);
            const files = await res.json();
            setCachedFiles(files || []);
        } catch (err) {
            console.error("Failed to fetch cache:", err);
        }
    }, [serverUrl]);

    const refreshAll = useCallback(async () => {
        if (!serverUrl) return;
        setRefreshing(true);
        await Promise.allSettled([fetchTorrents(), fetchDirect(), fetchCache()]);
        setRefreshing(false);
    }, [serverUrl, fetchTorrents, fetchDirect, fetchCache]);

    useEffect(() => {
        const init = async () => {
            setIsInitialLoading(true);
            await refreshAll();
            setIsInitialLoading(false);
        };
        init();

        if (!serverUrl) return;

        const torrentsSource = new EventSource(`${serverUrl}/api/torrents/stream`);
        torrentsSource.onmessage = (e) => {
            try {
                setTorrents(JSON.parse(e.data) || []);
            } catch {
                // Ignore SSE parse errors
            }
        };

        const directSource = new EventSource(`${serverUrl}/api/direct/stream`);
        directSource.onmessage = (e) => {
            try {
                setDirectDownloads(JSON.parse(e.data) || []);
            } catch {
                // Ignore SSE parse errors
            }
        };

        return () => {
            torrentsSource.close();
            directSource.close();
        };
    }, [serverUrl, refreshAll]);

    const items: LibraryItem[] = useMemo(() => {
        const list: LibraryItem[] = [];

        for (const t of torrents) {
            const id = `torrent:${t.infoHash}`;
            const status = t.progress >= 100
                ? t.downloadSpeed > 0 ? "Seeding" : "Completed"
                : t.downloadSpeed === 0 ? "Paused" : "Streaming";
            const totalSize = t.totalLength;
            const files: LibraryFile[] = t.files.map((f) => ({
                name: f.name,
                size: formatBytes(f.length),
                status: `${Math.round(f.progress)}%`,
                progress: f.progress,
                infoHash: t.infoHash,
                fileIndex: t.files.indexOf(f),
                streamUrl: serverUrl ? `${serverUrl}/stream/${t.infoHash}/${t.files.indexOf(f)}` : "",
                canPlay: f.progress > 0,
            }));
            list.push({
                id,
                type: "torrent",
                name: t.name || `Torrent ${t.infoHash.slice(0, 8)}`,
                full: t.magnetUri,
                status,
                size: totalSize,
                progress: t.progress,
                speed: t.downloadSpeed,
                peers: t.peers,
                backdrop: getBackdrop(id),
                accent: getAccent(id),
                files,
                raw: t,
            });
        }

        for (const d of directDownloads) {
            const id = `direct:${d.id}`;
            const status = d.status === "downloading"
                ? "Downloading"
                : d.status === "completed"
                    ? "Completed"
                    : d.status === "on_demand"
                        ? "On Demand"
                        : d.status.charAt(0).toUpperCase() + d.status.slice(1);
            const totalSize = d.totalBytes;
            const speed = d.status === "downloading"
                ? Math.max(0, (d.totalBytes * (d.progress / 100) - d.downloadedBytes))
                : 0;
            const files: LibraryFile[] = [
                {
                    name: d.filename,
                    size: formatBytes(totalSize),
                    status: `${d.progress.toFixed(1)}%`,
                    progress: d.progress,
                    downloadId: d.id,
                    streamUrl: serverUrl ? `${serverUrl}/stream/direct/${d.id}` : "",
                    canPlay: d.progress > 0,
                },
            ];
            list.push({
                id,
                type: "direct",
                name: d.filename,
                full: d.url,
                status,
                size: totalSize,
                progress: d.progress,
                speed,
                backdrop: getBackdrop(id),
                accent: getAccent(id),
                files,
                raw: d,
            });
        }

        const groupedCache: Record<string, CachedFile[]> = {};
        for (const f of cachedFiles) {
            if (f.type === "magnet" && f.infoHash) {
                if (!groupedCache[f.infoHash]) groupedCache[f.infoHash] = [];
                groupedCache[f.infoHash]!.push(f);
            } else if (f.type === "direct" && f.downloadId !== undefined) {
                const key = `direct:${f.downloadId}`;
                if (!groupedCache[key]) groupedCache[key] = [];
                groupedCache[key]!.push(f);
            }
        }

        for (const [key, files] of Object.entries(groupedCache)) {
            const first = files[0]!;
            const totalSize = files.reduce((sum, f) => sum + (f.size || 0), 0);
            const isDirect = first.type === "direct";
            const id = isDirect ? key : `cached:${key}`;
            const displayName = isDirect
                ? first.name
                : (first.name.split(".")[0] || key);
            const listFiles: LibraryFile[] = files.map((f) => ({
                name: f.name,
                size: formatBytes(f.size || 0),
                status: "100%",
                progress: 100,
                infoHash: f.infoHash,
                fileIndex: f.fileIndex,
                downloadId: f.downloadId,
                streamUrl: f.streamUrl,
                canPlay: f.canPlay,
            }));
            list.push({
                id,
                type: "cached",
                name: displayName,
                full: files.length > 1
                    ? `${files.length} cached files · ${formatBytes(totalSize)}`
                    : first.name,
                status: "Completed",
                size: totalSize,
                progress: 100,
                speed: 0,
                backdrop: getBackdrop(id),
                accent: getAccent(id),
                files: listFiles,
                raw: first,
            });
        }

        return list;
    }, [torrents, directDownloads, cachedFiles, serverUrl]);

    const filteredItems = useMemo(() => {
        let list = items;
        if (tab !== "all") {
            list = list.filter((i) => i.type === tab);
        }
        if (search.trim()) {
            const q = search.toLowerCase();
            list = list.filter((i) => i.name.toLowerCase().includes(q) || i.full.toLowerCase().includes(q));
        }
        return list;
    }, [items, tab, search]);

    useEffect(() => {
        if (filteredItems.length === 0) {
            setSelectedId(null);
            return;
        }
        if (!selectedId || !filteredItems.find((i) => i.id === selectedId)) {
            setSelectedId(filteredItems[0]!.id);
        }
    }, [filteredItems, selectedId]);

    const selected = useMemo(() => items.find((i) => i.id === selectedId) ?? null, [items, selectedId]);

    const counts = useMemo(() => {
        const total = items.length;
        const active = items.filter((i) => i.type === "torrent" && i.status !== "Completed" && i.status !== "Paused").length
            + items.filter((i) => i.type === "direct" && i.status === "Downloading").length;
        const cached = items.filter((i) => i.type === "cached").length;
        return { total, active, cached };
    }, [items]);

    const totals = useMemo(() => {
        let dl = 0, ul = 0;
        for (const t of torrents) {
            dl += t.downloadSpeed;
            ul += t.downloadSpeed * 0.3;
        }
        for (const d of directDownloads) {
            if (d.status === "downloading") {
                dl += (d.totalBytes * (d.progress / 100) - d.downloadedBytes);
            }
        }
        return { dl, ul };
    }, [torrents, directDownloads]);

    const handleCopy = (text: string) => {
        if (!text) return;
        navigator.clipboard.writeText(text).catch(() => {});
    };

    const handleRemoveTorrent = async (infoHash: string) => {
        if (!serverUrl) return;
        await fetch(`${serverUrl}/api/remove/${infoHash}`, { method: "DELETE" });
        fetchTorrents();
    };

    const handleRemoveDirect = async (id: number) => {
        if (!serverUrl) return;
        await fetch(`${serverUrl}/api/direct/${id}`, { method: "DELETE" });
        fetchDirect();
    };

    const handleRemove = (id: string, type: LibraryItemType) => {
        if (type === "torrent") {
            const hash = id.replace("torrent:", "");
            handleRemoveTorrent(hash);
        } else if (type === "direct") {
            const dId = parseInt(id.replace("direct:", ""), 10);
            handleRemoveDirect(dId);
        }
    };

    const handleDeleteCache = async (infoHash: string) => {
        if (!serverUrl) return;
        try {
            await fetch(`${serverUrl}/api/cache/${infoHash}`, { method: "DELETE" });
            fetchCache();
        } catch (err) {
            console.error("Failed to delete cache:", err);
        }
    };

    const handleReencode = async (options: { infoHash?: string; fileIndex?: number; downloadId?: number; resolution: string; bitrate: string }): Promise<boolean> => {
        if (!serverUrl) return false;
        try {
            const res = await fetch(`${serverUrl}/api/reencode`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(options),
            });
            if (!res.ok) throw new Error("Failed to start reencoding");
            setTimeout(fetchCache, 2000);
            return true;
        } catch (err) {
            console.error("Reencode error:", err);
            return false;
        }
    };

    const handleMoveToDrive = (options: { infoHash?: string; fileIndex?: number; downloadId?: number; exportPath?: string }) => {
        if (!serverUrl) return;
        fetch(`${serverUrl}/api/gdrive/upload`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(options),
        })
            .then(() => alert("Upload to Google Drive started in background!"))
            .catch(() => alert("Failed to start GDrive upload."));
    };

    const handleAddMagnet = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!magnet || !serverUrl) return;
        setAddLoading(true);
        setAddError("");
        try {
            const res = await fetch(`${serverUrl}/api/add`, {
                method: "POST",
                headers: { "Content-Type": "application/x-www-form-urlencoded" },
                body: `magnet=${encodeURIComponent(magnet)}`,
            });
            if (!res.ok) throw new Error("Failed to add torrent");
            setMagnet("");
            setShowAddForm(false);
            fetchTorrents();
        } catch {
            setAddError("Invalid magnet link or server error");
        } finally {
            setAddLoading(false);
        }
    };

    const handleAddDirect = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!directUrl || !serverUrl) return;
        setAddLoading(true);
        setAddError("");
        try {
            const res = await fetch(`${serverUrl}/api/direct/add`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ url: directUrl, mode: "ondemand" }),
            });
            if (!res.ok) throw new Error("Failed to add direct download");
            setDirectUrl("");
            setShowAddForm(false);
            fetchDirect();
        } catch {
            setAddError("Invalid direct URL or server error");
        } finally {
            setAddLoading(false);
        }
    };

    const handleClearAll = async () => {
        if (!serverUrl) return;
        try {
            await fetch(`${serverUrl}/api/torrents/all`, { method: "DELETE" });
            fetchTorrents();
        } catch (err) {
            console.error("Failed to remove all torrents:", err);
        }
    };

    return (
        <TooltipProvider>
            <div
                ref={containerRef}
                className="library-root relative flex h-full w-full overflow-hidden bg-background"
            >
                {/* Dynamic Backdrop */}
                <div className="pointer-events-none fixed inset-0 -z-10 overflow-hidden">
                    <div
                        className="absolute -inset-10 opacity-50 transition-all duration-700 ease-out"
                        style={{
                            background: selected?.backdrop ?? "transparent",
                            filter: "blur(60px) saturate(1.4) brightness(0.45)",
                        }}
                    />
                    <div
                        className="absolute inset-0"
                        style={{
                            background:
                                "radial-gradient(circle at 20% 50%, transparent 0%, var(--background, #0a0a0a) 80%)",
                        }}
                    />
                    <div className="absolute inset-0 bg-gradient-to-b from-transparent via-background/40 to-background/80" />
                </div>

                {/* Left Pane: Queue */}
                <aside className="flex h-full w-full max-w-[460px] flex-shrink-0 flex-col border-r border-white/[0.06] bg-gradient-to-b from-black/30 to-black/10 backdrop-blur-md">
                    {/* Header */}
                    <div className="px-7 pt-9 pb-5">
                        <div className="mb-4 flex items-center justify-between">
                            <div className="flex items-center gap-2 text-xs font-bold uppercase tracking-[0.2em] text-white/50">
                                <Activity className="h-3.5 w-3.5" />
                                <span>Library</span>
                            </div>
                            <div className="flex items-center gap-1">
                                <Button
                                    variant="ghost"
                                    size="icon-sm"
                                    onClick={refreshAll}
                                    disabled={refreshing}
                                    className="rounded-full text-white/50 hover:bg-white/10 hover:text-white"
                                >
                                    <RefreshCw className={cn("h-4 w-4", refreshing && "animate-spin")} />
                                </Button>
                                <Button
                                    size="icon-sm"
                                    onClick={() => setShowAddForm((s) => !s)}
                                    className={cn(
                                        "rounded-full",
                                        showAddForm
                                            ? "bg-white/10 text-white/70 hover:bg-white/20"
                                            : "bg-white text-black shadow-lg shadow-white/20 hover:bg-white"
                                    )}
                                >
                                    <Plus className={cn("h-4 w-4", showAddForm && "rotate-45 transition-transform")} />
                                </Button>
                            </div>
                        </div>

                        <h2 className="bg-gradient-to-b from-white to-white/50 bg-clip-text text-3xl font-black tracking-tight text-transparent">
                            Transfers
                        </h2>

                        <div className="mt-3 flex items-center gap-5 text-[12px] font-medium text-white/50">
                            <div className="flex items-center gap-1.5">
                                <Wifi className="h-3 w-3" />
                                <span>Active: <span className="font-mono text-white">{counts.active}</span></span>
                            </div>
                            <div className="flex items-center gap-1.5">
                                <LibraryIcon className="h-3 w-3" />
                                <span>Library: <span className="font-mono text-white">{counts.cached}</span></span>
                            </div>
                        </div>

                        {showAddForm && (
                            <div className="mt-5 space-y-3 rounded-2xl border border-white/10 bg-white/[0.04] p-3 backdrop-blur-md animate-in slide-in-from-top-2">
                                <div className="flex gap-1.5">
                                    <button
                                        onClick={() => setAddType("magnet")}
                                        className={cn(
                                            "flex-1 rounded-full px-3 py-1.5 text-xs font-semibold transition-all",
                                            addType === "magnet"
                                                ? "bg-white text-black"
                                                : "bg-white/5 text-white/60 hover:bg-white/10"
                                        )}
                                    >
                                        Magnet
                                    </button>
                                    <button
                                        onClick={() => setAddType("direct")}
                                        className={cn(
                                            "flex-1 rounded-full px-3 py-1.5 text-xs font-semibold transition-all",
                                            addType === "direct"
                                                ? "bg-white text-black"
                                                : "bg-white/5 text-white/60 hover:bg-white/10"
                                        )}
                                    >
                                        Direct URL
                                    </button>
                                </div>
                                <form
                                    onSubmit={addType === "magnet" ? handleAddMagnet : handleAddDirect}
                                    className="flex gap-2"
                                >
                                    <Input
                                        value={addType === "magnet" ? magnet : directUrl}
                                        onChange={(e) => addType === "magnet" ? setMagnet(e.target.value) : setDirectUrl(e.target.value)}
                                        placeholder={addType === "magnet" ? "magnet:?xt=urn:btih:..." : "https://example.com/video.mp4"}
                                        className="h-9 border-white/10 bg-black/40 text-xs"
                                    />
                                    <Button type="submit" size="sm" disabled={addLoading} className="h-9 rounded-full bg-white px-4 text-xs font-bold text-black hover:bg-white/90">
                                        {addLoading ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : "Add"}
                                    </Button>
                                </form>
                                {addError && (
                                    <p className="flex items-center gap-1.5 text-[11px] font-semibold text-red-400">
                                        <AlertCircle className="h-3 w-3" />
                                        {addError}
                                    </p>
                                )}
                            </div>
                        )}

                        {/* Tabs */}
                        <div className="mt-5 flex gap-1 border-b border-white/5 pb-0">
                            {([
                                { key: "all", label: "All" },
                                { key: "torrent", label: "Torrents" },
                                { key: "direct", label: "Direct" },
                                { key: "cached", label: "Cached" },
                            ] as { key: Tab; label: string }[]).map((t) => (
                                <button
                                    key={t.key}
                                    onClick={() => setTab(t.key)}
                                    className={cn(
                                        "relative -mb-px border-b-2 px-3 py-2 text-xs font-semibold transition-colors",
                                        tab === t.key
                                            ? "border-white text-white"
                                            : "border-transparent text-white/50 hover:text-white/80"
                                    )}
                                >
                                    {t.label}
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* Search */}
                    <div className="px-7 pb-2">
                        <Input
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                            placeholder="Search library…"
                            className="h-9 border-white/5 bg-white/[0.03] text-xs placeholder:text-white/30"
                        />
                    </div>

                    {/* List */}
                    <div className="flex-1 overflow-y-auto px-4 pb-8 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                        {isInitialLoading ? (
                            <div className="space-y-3">
                                {[0, 1, 2, 3].map((i) => (
                                    <div
                                        key={i}
                                        className="h-[110px] animate-pulse rounded-2xl bg-white/[0.04]"
                                    />
                                ))}
                            </div>
                        ) : filteredItems.length === 0 ? (
                            <Empty className="border-0 bg-transparent py-16">
                                <EmptyMedia variant="icon">
                                    <LibraryIcon />
                                </EmptyMedia>
                                <EmptyTitle>Library is empty</EmptyTitle>
                                <EmptyDescription>
                                    {search.trim()
                                        ? "No items match your search."
                                        : "Add a magnet or direct URL to get started."}
                                </EmptyDescription>
                            </Empty>
                        ) : (
                            <div className="space-y-2.5">
                                {filteredItems.map((item) => (
                                    <QueueCard
                                        key={item.id}
                                        item={item}
                                        active={item.id === selectedId}
                                        onClick={() => setSelectedId(item.id)}
                                    />
                                ))}
                            </div>
                        )}

                        {torrents.length > 0 && (
                            <div className="mt-6 flex items-center justify-center">
                                <AlertDialog>
                                    <AlertDialogTrigger asChild>
                                        <Button variant="ghost" size="sm" className="text-xs text-white/40 hover:bg-red-500/10 hover:text-red-400">
                                            Clear All Torrents
                                        </Button>
                                    </AlertDialogTrigger>
                                    <AlertDialogContent>
                                        <AlertDialogHeader>
                                            <AlertDialogTitle>Stop all transfers?</AlertDialogTitle>
                                            <AlertDialogDescription>This will remove all active torrent sessions.</AlertDialogDescription>
                                        </AlertDialogHeader>
                                        <AlertDialogFooter>
                                            <AlertDialogCancel>Cancel</AlertDialogCancel>
                                            <AlertDialogAction onClick={handleClearAll} className="bg-red-500 text-white hover:bg-red-500/90">
                                                Remove All
                                            </AlertDialogAction>
                                        </AlertDialogFooter>
                                    </AlertDialogContent>
                                </AlertDialog>
                            </div>
                        )}
                    </div>
                </aside>

                {/* Right Pane: Detail */}
                <section className="relative flex h-full flex-1 flex-col overflow-hidden">
                    {/* Global Stats - Top Right */}
                    <div className="pointer-events-none absolute right-10 top-9 z-10 flex gap-8">
                        <div className="text-right">
                            <div className="font-mono text-xl font-bold tracking-tight text-white">
                                {formatSpeed(totals.dl)}
                            </div>
                            <div className="text-[10px] font-extrabold uppercase tracking-[0.15em] text-white/40">
                                Total Down
                            </div>
                        </div>
                        <div className="text-right">
                            <div className="font-mono text-xl font-bold tracking-tight text-white">
                                {formatSpeed(totals.ul)}
                            </div>
                            <div className="text-[10px] font-extrabold uppercase tracking-[0.15em] text-white/40">
                                Total Up
                            </div>
                        </div>
                    </div>

                    {selected ? (
                        <div className="h-full w-full overflow-x-hidden px-10 pt-24 pb-8 lg:px-16">
                            <DetailPane
                                item={selected}
                                onCopy={handleCopy}
                                onRemove={handleRemove}
                                onDeleteCache={handleDeleteCache}
                                onReencode={handleReencode}
                                onMoveToDrive={handleMoveToDrive}
                            />
                        </div>
                    ) : (
                        <div className="flex h-full items-center justify-center px-16">
                            <Empty className="border-0 bg-transparent">
                                <EmptyMedia variant="icon">
                                    <LibraryIcon />
                                </EmptyMedia>
                                <EmptyTitle>Select an item to preview</EmptyTitle>
                                <EmptyDescription>
                                    Pick a transfer or cached file from the left to see its details.
                                </EmptyDescription>
                            </Empty>
                        </div>
                    )}
                </section>
            </div>
        </TooltipProvider>
    );
}

export default Library;
