import { useState } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Loader2, Search, ChevronLeft, HardDrive, Users, Copy, Download, Check, ArrowLeft } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { MediaInfo, TorrentResult, EpisodeInfo, ProviderInfo } from "./types";

type TorrentPanelProps = {
    detail: MediaInfo;
    show: boolean;
    onClose: () => void;
    providers: ProviderInfo[];
    selectedProvider: string;
    onProviderChange: (value: string) => void;
    torrentQuery: string;
    setTorrentQuery: (value: string) => void;
    searchTorrents: (query?: string, episode?: EpisodeInfo) => void;
    searchingTorrents: boolean;
    torrentResults: TorrentResult[];
    addingTorrent: string | null;
    copyMagnet: (magnet: string) => void;
    addTorrent: (magnet: string) => void;
    copiedMagnet: string | null;
    selectedEpisode?: EpisodeInfo | null;
    fetchDetailFromUrl?: (url: string) => Promise<void>;
};

export default function TorrentPanel({
    detail,
    show,
    onClose,
    providers,
    selectedProvider,
    onProviderChange,
    torrentQuery,
    setTorrentQuery,
    searchTorrents,
    searchingTorrents,
    torrentResults,
    addingTorrent,
    addTorrent,
    copyMagnet,
    copiedMagnet,
    selectedEpisode,
    fetchDetailFromUrl,
}: TorrentPanelProps) {
const [detailView, setDetailView] = useState<TorrentResult | null>(null);
    const [loadingDetail, setLoadingDetail] = useState(false);

    if (!show) return null;

    const handleResultClick = async (torrent: TorrentResult) => {
        if (torrent.url && fetchDetailFromUrl) {
            setDetailView(torrent);
            setLoadingDetail(true);
            try {
                await fetchDetailFromUrl(torrent.url);
            } finally {
                setLoadingDetail(false);
            }
        } else if (torrent.magnet) {
            setDetailView(torrent);
        }
    };

    const handleBackToList = () => {
        setDetailView(null);
    };

    const getUpdatedMagnet = () => {
        if (!detailView?.url) return "";
        const updated = torrentResults.find(r => r.url === detailView.url);
        return updated?.magnet || "";
    };

    return (
        <div className="fixed inset-0 z-50 flex justify-end">
            <div
                className="absolute inset-0 bg-black/60 backdrop-blur-sm"
                onClick={onClose}
            />
            <div className="relative w-full max-w-md bg-background/95 backdrop-blur-md border-l border-white/10 shadow-2xl animate-in slide-in-from-right duration-300">
                <div className="p-4 border-b border-white/10 flex items-center justify-between">
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                        <Button
                            variant="ghost"
                            size="icon"
                            className="size-8 flex-shrink-0"
                            onClick={detailView ? handleBackToList : onClose}
                        >
                            {detailView ? <ArrowLeft className="size-4" /> : <ChevronLeft className="size-4" />}
                        </Button>
                        <div className="min-w-0">
                            <h3 className="font-semibold truncate">
                                {detailView 
                                    ? detailView.name 
                                    : selectedEpisode
                                        ? `S${selectedEpisode.season}E${selectedEpisode.episode} ${selectedEpisode.title}`
                                        : detail.title
                                }
                            </h3>
                            <p className="text-xs text-muted-foreground truncate">
                                {detailView ? "Detail view" : torrentQuery}
                            </p>
                        </div>
                    </div>
                    {!detailView && (
                        <Select value={selectedProvider || ""} onValueChange={(v) => onProviderChange(v)}>
                            <SelectTrigger className="w-[140px]">
                                <SelectValue placeholder="Choose" />
                            </SelectTrigger>
<SelectContent>
                            {providers.map((p) => (
                                <SelectItem key={p.id} value={p.id}>
                                    {p.name}
                                </SelectItem>
                            ))}
                        </SelectContent>
                        </Select>
                    )}
                </div>

                {!detailView ? (
                    <>
                        <div className="p-4 border-b border-white/10">
                            <form onSubmit={(e) => {
                                e.preventDefault();
                                searchTorrents(torrentQuery);
                            }} className="flex gap-2">
                                <Input
                                    value={torrentQuery}
                                    onChange={(e) => setTorrentQuery(e.target.value)}
                                    placeholder="Search query..."
                                    className="flex-1"
                                />
                                <Button type="submit" size="icon" disabled={searchingTorrents}>
                                    {searchingTorrents ? (
                                        <Loader2 className="size-4 animate-spin" />
                                    ) : (
                                        <Search className="size-4" />
                                    )}
                                </Button>
                            </form>
                        </div>

                        <div className="flex-1 overflow-y-auto h-[calc(100vh-180px)] w-full">
                            <div className="flex flex-col gap-3 p-4 w-full">
                                {!selectedProvider ? (
                                    <div className="text-center py-12 text-muted-foreground">
                                        <Search className="size-12 mx-auto mb-4 opacity-20" />
                                        <p>Select a provider first</p>
                                        <p className="text-xs mt-1">Choose a torrent provider from the dropdown above</p>
                                    </div>
                                ) : searchingTorrents ? (
                                    <div className="flex flex-col items-center justify-center py-12 gap-3">
                                        <Loader2 className="size-8 animate-spin text-primary" />
                                        <p className="text-sm text-muted-foreground">Searching torrents...</p>
                                    </div>
                                ) : torrentResults.length === 0 ? (
                                    <div className="text-center py-12 text-muted-foreground">
                                        <Search className="size-12 mx-auto mb-4 opacity-20" />
                                        <p>No torrents found</p>
                                        <p className="text-xs mt-1">Try a different search query</p>
                                    </div>
                                ) : (
torrentResults.map((torrent, index) => (
                                        <Card 
                                            key={index} 
                                            className={`p-4 space-y-3 w-full ${!torrent.magnet && torrent.url ? 'cursor-pointer' : ''}`}
                                            onClick={() => handleResultClick(torrent)}
                                        >
                                            <h4 className="text-sm font-medium leading-normal line-clamp-2 break-words pr-2">
                                                {torrent.name}
                                            </h4>
                                            <div className="flex flex-wrap gap-2">
                                                <Badge variant="outline" className="text-xs gap-1">
                                                    <HardDrive className="size-3" />
                                                    {torrent.size}
                                                </Badge>
                                                <Badge variant="outline" className="text-xs gap-1 text-green-600 border-green-600/30">
                                                    <Users className="size-3" />
                                                    {torrent.seeders}
                                                </Badge>
                                                {torrent.category && (
                                                    <Badge variant="secondary" className="text-xs">
                                                        {torrent.category}
                                                    </Badge>
                                                )}
                                                {!torrent.magnet && torrent.url && (
                                                    <Badge variant="outline" className="text-xs gap-1 text-purple-500 border-purple-500/30">
                                                        <ArrowLeft className="size-3" />
                                                        Click for detail
                                                    </Badge>
                                                )}
                                            </div>
                                        </Card>
                                    ))
                                )}
                            </div>
                        </div>
                    </>
                ) : (
                    <div className="flex-1 overflow-y-auto h-[calc(100vh-80px)] w-full">
                        <div className="flex flex-col gap-3 p-4 w-full">
                            {loadingDetail ? (
                                <div className="flex flex-col items-center justify-center py-12 gap-3">
                                    <Loader2 className="size-8 animate-spin text-primary" />
                                    <p className="text-sm text-muted-foreground">Fetching magnet link...</p>
                                </div>
                            ) : (
                                <Card className="p-4 space-y-3 w-full">
                                    <h4 className="text-sm font-medium leading-normal break-words pr-2">
                                        {detailView.name}
                                    </h4>
                                    <div className="flex flex-wrap gap-2">
                                        <Badge variant="outline" className="text-xs gap-1">
                                            <HardDrive className="size-3" />
                                            {detailView.size}
                                        </Badge>
                                        <Badge variant="outline" className="text-xs gap-1 text-green-600 border-green-600/30">
                                            <Users className="size-3" />
                                            {detailView.seeders}
                                        </Badge>
                                    </div>
                                    <div className="flex gap-2 pt-1">
                                        {detailView.url && (() => {
                                            const url = detailView.url;
                                            return (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                className="flex-1 gap-1.5"
                                                onClick={() => copyMagnet(url)}
                                            >
                                                {copiedMagnet === url ? (
                                                <>
                                                    <Check className="size-3" />
                                                    Copied
                                                </>
                                            ) : (
                                                <>
                                                    <Copy className="size-3" />
                                                    Copy URL
                                                </>
                                            )}
                                            </Button>
                                            );
                                        })()}
                                        <Button
                                            size="sm"
                                            className="flex-1 gap-1.5"
                                            onClick={() => addTorrent(getUpdatedMagnet())}
                                            disabled={addingTorrent === getUpdatedMagnet()}
                                        >
                                            {addingTorrent === getUpdatedMagnet() ? (
                                                <Loader2 className="size-3 animate-spin" />
                                            ) : (
                                                <Download className="size-3" />
                                            )}
                                            Add
                                        </Button>
                                    </div>
                                </Card>
                            )}
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}
