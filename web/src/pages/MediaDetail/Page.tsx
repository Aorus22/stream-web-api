import { useState, useEffect, useMemo, useCallback } from "react";
import { useParams, useLocation } from "react-router-dom";
import { router } from "@/router/route";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useServer } from "@/contexts/ServerContext";
import { ArrowLeft, Share2 } from "lucide-react";
import { toast } from "sonner";
import HeroInfo from "./HeroInfo";
import EpisodePanel from "./EpisodePanel";
import TorrentPanel from "./TorrentPanel";
import CastSection from "./CastSection";
import CrewSection from "./CrewSection";
import type { MediaInfo, TorrentResult, EpisodeInfo, ProviderInfo } from "./types";

export function MediaDetail() {
    const { serverUrl } = useServer();
    const { id } = useParams<{ id: string }>();
    const location = useLocation();
    const type = location.pathname.split('/')[1];

const [detail, setDetail] = useState<MediaInfo | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const [selectedSeason, setSelectedSeason] = useState(1);
    const [showTorrentPanel, setShowTorrentPanel] = useState(false);
    const [torrentQuery, setTorrentQuery] = useState("");
    const [providers, setProviders] = useState<ProviderInfo[]>([]);
    const [selectedProvider, setSelectedProvider] = useState("");
    const [torrentResults, setTorrentResults] = useState<TorrentResult[]>([]);
    const [searchingTorrents, setSearchingTorrents] = useState(false);
    const [addingTorrent, setAddingTorrent] = useState<string | null>(null);
    const [copiedMagnet, setCopiedMagnet] = useState<string | null>(null);
    const [selectedEpisode, setSelectedEpisode] = useState<EpisodeInfo | null>(null);

    const handleShare = useCallback(async () => {
        if (typeof window === "undefined") return;
        const url = window.location.href;
        const title = detail?.title ?? "Media";
        const text = detail?.title ? `Check out ${detail.title}` : "Check out this media";

        try {
            if (navigator.share) {
                await navigator.share({ title, text, url });
                toast.success("Shared the media link");
            } else if (navigator.clipboard) {
                await navigator.clipboard.writeText(url);
                toast.success("Link copied to clipboard");
            } else {
                toast.error("Clipboard access is not available");
            }
        } catch {
            toast.error("Unable to share right now");
        }
    }, [detail]);

    useEffect(() => {
        if (!serverUrl) return;
        fetch(`${serverUrl}/api/providers`)
            .then((res) => res.json())
            .then((data) => {
                setProviders(data || []);
            })
            .catch(console.error);
    }, [serverUrl]);

    useEffect(() => {
        const fetchDetail = async () => {
            if (!serverUrl || !id) return;
            setLoading(true);
            setError("");

            const mediaType = type === "series" || type === "tv" ? "series" : "movie";
            try {
                const res = await fetch(`${serverUrl}/api/catalog/${mediaType}/${id}`);
                if (!res.ok) throw new Error(`Failed to fetch details: ${res.status}`);
                const data: MediaInfo = await res.json();
                setDetail(data);
            } catch (err) {
                console.error("Media detail error:", err);
                setError("Failed to load media details");
            } finally {
                setLoading(false);
            }
        };

        fetchDetail();
    }, [serverUrl, id, type]);

    const seasons = useMemo(() => {
        if (!detail?.episodes) return [];
        const seasonSet = new Set(detail.episodes.map((ep) => ep.season));
        return Array.from(seasonSet).filter((s) => s > 0).sort((a, b) => a - b);
    }, [detail?.episodes]);

    useEffect(() => {
        if (seasons.length > 0 && !seasons.includes(selectedSeason)) {
            setSelectedSeason(seasons[0]);
        }
    }, [seasons, selectedSeason]);

    const filteredEpisodes = useMemo(() => {
        if (!detail?.episodes) return [];
        return detail.episodes
            .filter((ep) => ep.season === selectedSeason)
            .sort((a, b) => a.episode - b.episode);
    }, [detail?.episodes, selectedSeason]);

const searchTorrents = async (query?: string, episode?: EpisodeInfo, providerOverride?: string) => {
        if (!detail || !serverUrl) return;
        const searchQuery = query || (detail.mediaType === "series"
            ? `${detail.title} S${String(selectedSeason).padStart(2, '0')}`
            : `${detail.title} ${detail.year || ""}`);

        setTorrentQuery(searchQuery);
        setSelectedEpisode(episode || null);
        setShowTorrentPanel(true);

        const providerId = providerOverride || selectedProvider;

        if (!providerId) {
            setSearchingTorrents(false);
            setTorrentResults([]);
            return;
        }

        setSearchingTorrents(true);
        setTorrentResults([]);

        try {
            const provider = providers.find(p => p.id === providerId);
            
            if (provider?.type === "custom") {
                const res = await fetch(
                    `${serverUrl}/api/search/custom/${providerId}?query=${encodeURIComponent(searchQuery.trim())}`
                );
                const data = await res.json();
                
                if (data.result?.type === 'list' && data.result?.results) {
                    const listResults = data.result.results.map((item: { name: string; url: string; size?: string; seeds?: number; leeches?: number }) => ({
                        name: item.name,
                        magnet: '',
                        size: item.size || 'Unknown',
                        seeders: String(item.seeds || 0),
                        leechers: String(item.leeches || 0),
                        category: '',
                        uploadedBy: '',
                        dateUploaded: '',
                        url: item.url,
                    }));
                    setTorrentResults(listResults);
                } else if (data.result?.type === 'detail') {
                    const detailResult: TorrentResult = {
                        name: data.result.name || searchQuery,
                        magnet: data.result.magnetLink || '',
                        size: '',
                        seeders: '0',
                        leechers: '0',
                        category: '',
                        uploadedBy: '',
                        dateUploaded: '',
                    };
                    setTorrentResults([detailResult]);
                } else {
                    setTorrentResults([]);
                }
            } else {
                const res = await fetch(
                    `${serverUrl}/api/search?provider=${providerId}&query=${encodeURIComponent(searchQuery.trim())}`
                );
                const data = await res.json();
                setTorrentResults(Array.isArray(data) ? data : []);
            }
        } catch (err) {
            console.error("Failed to search torrents:", err);
            setTorrentResults([]);
        } finally {
            setSearchingTorrents(false);
        }
    };

    const fetchDetailFromUrl = async (url: string) => {
        if (!serverUrl || !selectedProvider) return;
        
        const provider = providers.find(p => p.id === selectedProvider);
        if (provider?.type !== "custom") return;

        setSearchingTorrents(true);
        try {
            const res = await fetch(
                `${serverUrl}/api/search/custom/${selectedProvider}?detailUrl=${encodeURIComponent(url)}`
            );
            const data = await res.json();
            
            if (data.result?.type === 'detail') {
                const detailResult: TorrentResult = {
                    name: data.result.name || 'Unknown',
                    magnet: data.result.magnetLink || '',
                    size: '',
                    seeders: '0',
                    leechers: '0',
                    category: '',
                    uploadedBy: '',
                    dateUploaded: '',
                };
                setTorrentResults(prev => 
                    prev.map(item => item.url === url ? { ...item, magnet: detailResult.magnet } : item)
                );
            }
        } catch (err) {
            console.error("Failed to fetch detail:", err);
        } finally {
            setSearchingTorrents(false);
        }
    };

    const openTorrentPanel = () => {
        setShowTorrentPanel(true);
        if (selectedProvider) {
            searchTorrents(undefined, undefined, selectedProvider);
        }
    };

    const handleProviderChange = (value: string) => {
        setSelectedProvider(value);
        if (showTorrentPanel && value) {
            searchTorrents(torrentQuery, selectedEpisode || undefined, value);
        }
    };

    const addTorrent = async (magnet: string) => {
        if (!serverUrl || !detail) return;
        setAddingTorrent(magnet);
        try {
            const metadataObj: { title: string; background: string; logo: string } = {
                title: selectedEpisode
                    ? `${detail.title} S${String(selectedEpisode.season).padStart(2, '0')}E${String(selectedEpisode.episode).padStart(2, '0')}`
                    : detail.title,
                background: detail.backdrop || '',
                logo: detail.logo || '',
            };

            const res = await fetch(`${serverUrl}/api/add`, {
                method: "POST",
                headers: { "Content-Type": "application/x-www-form-urlencoded" },
                body: `magnet=${encodeURIComponent(magnet)}&metadata=${encodeURIComponent(JSON.stringify(metadataObj))}`,
            });
            if (!res.ok) throw new Error("Failed to add torrent");
            router.navigate('/dashboard');
        } catch (err) {
            console.error("Failed to add torrent:", err);
        } finally {
            setAddingTorrent(null);
        }
    };

    const copyMagnet = (magnet: string) => {
        navigator.clipboard.writeText(magnet);
        setCopiedMagnet(magnet);
        setTimeout(() => setCopiedMagnet(null), 2000);
    };

    const openTrailer = () => {
        if (!detail?.trailers?.length) return;
        const trailer = detail.trailers.find((t) => t.type === "Trailer") || detail.trailers[0];
        if (trailer?.source) {
            window.open(`https://www.youtube.com/watch?v=${trailer.source}`, '_blank');
        }
    };

    if (loading) {
        return (
            <div className="min-h-screen bg-background">
                <div className="relative h-[70vh]">
                    <Skeleton className="absolute inset-0" />
                </div>
            </div>
        );
    }

    if (error || !detail) {
        return (
            <div className="min-h-screen bg-background flex items-center justify-center">
                <div className="text-center space-y-4">
                    <p className="text-destructive">{error || "Not found"}</p>
                    <Button onClick={() => router.navigate(-1)}>Go Back</Button>
                </div>
            </div>
        );
    }

    const isSeries = detail.mediaType === "series";

    return (
        <div className="min-h-screen bg-background">
            <div className="relative min-h-screen">
                <div className="absolute inset-0">
                    {detail.backdrop ? (
                        <img
                            src={detail.backdrop}
                            alt={detail.title}
                            className="w-full h-full object-cover"
                        />
                    ) : (
                        <div className="w-full h-full bg-linear-to-br from-primary/20 to-background" />
                    )}
                    <div className="absolute inset-0 bg-linear-to-r from-background via-background/95 to-background/60" />
                    <div className="absolute inset-0 bg-linear-to-t from-background via-background/50 to-transparent" />
                </div>

                <div className="relative z-10 flex min-h-screen flex-col pt-20 lg:pt-28 pb-8">
                    <div className="absolute top-10 left-0 right-0 px-6 sm:px-8 md:px-10 lg:px-14 pointer-events-none">
                        <div className="flex justify-between pointer-events-auto w-full">
                            <button
                                onClick={() => router.navigate(-1)}
                                className="flex items-center justify-center p-2 bg-white/10 rounded-full text-white/80 hover:text-white hover:bg-white/20 transition-colors"
                                aria-label="Go back"
                            >
                                <ArrowLeft size={20} />
                            </button>
                            <button
                                onClick={handleShare}
                                className="flex items-center justify-center p-2 bg-white/10 rounded-full text-white/80 hover:text-white hover:bg-white/20 transition-colors"
                                aria-label="Share media"
                            >
                                <Share2 size={20} />
                            </button>
                        </div>
                    </div>

                    <div className="w-full flex flex-col gap-12 px-6 sm:px-8 md:px-10 lg:px-14 mt-6">
                        <div className="w-full flex flex-col lg:flex-row gap-8 flex-1">
                        <HeroInfo
                            detail={detail}
                            isSeries={isSeries}
                            seasons={seasons}
                            openTrailer={openTrailer}
                            onFindTorrents={openTorrentPanel}
                            searchingTorrents={searchingTorrents}
                            onImdb={() => detail.id && window.open(`https://www.imdb.com/title/${detail.id}`, '_blank')}
                        />
                            {isSeries && seasons.length > 0 && (
                        <EpisodePanel
                            detail={detail}
                            seasons={seasons}
                            selectedSeason={selectedSeason}
                            setSelectedSeason={setSelectedSeason}
                            filteredEpisodes={filteredEpisodes}
                            searchTorrents={searchTorrents}
                        />
                            )}
                        </div>
                        {detail.cast && detail.cast.length > 6 && (
                            <CastSection cast={detail.cast} />
                        )}
                        <div className="w-full">
                            <CrewSection director={detail.director} writer={detail.writer} />
                        </div>
                    </div>
                </div>
            </div>

<TorrentPanel
                detail={detail}
                show={showTorrentPanel}
                onClose={() => setShowTorrentPanel(false)}
                providers={providers}
                selectedProvider={selectedProvider}
                onProviderChange={handleProviderChange}
                torrentQuery={torrentQuery}
                setTorrentQuery={setTorrentQuery}
                searchTorrents={searchTorrents}
                searchingTorrents={searchingTorrents}
                torrentResults={torrentResults}
                addingTorrent={addingTorrent}
                copyMagnet={copyMagnet}
                addTorrent={addTorrent}
                copiedMagnet={copiedMagnet}
                selectedEpisode={selectedEpisode}
                fetchDetailFromUrl={fetchDetailFromUrl}
            />
        </div>
    );
}

export default MediaDetail;
