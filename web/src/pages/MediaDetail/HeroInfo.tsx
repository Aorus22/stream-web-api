import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Play, Search, Plus, ExternalLink, Clock, Calendar, Loader2, Star } from "lucide-react";
import type { MediaInfo } from "./types";

type HeroInfoProps = {
    detail: MediaInfo;
    isSeries: boolean;
    seasons: number[];
    openTrailer: () => void;
    onFindTorrents: () => void;
    searchingTorrents: boolean;
    onImdb?: () => void;
};

export default function HeroInfo({
    detail,
    isSeries,
    seasons,
    openTrailer,
    onFindTorrents,
    searchingTorrents,
    onImdb,
}: HeroInfoProps) {
    return (
        <div className="flex-1 space-y-6">
            {detail.logo ? (
                <img
                    src={detail.logo}
                    alt={detail.title}
                    className="max-w-md max-h-32 object-contain"
                />
            ) : (
                <h1 className="text-4xl md:text-5xl lg:text-6xl font-bold text-white drop-shadow-lg">
                    {detail.title}
                </h1>
            )}

            <div className="flex flex-wrap items-center gap-4 text-white/90">
                {detail.runtime && (
                    <div className="flex items-center gap-1.5">
                        <Clock className="size-4" />
                        <span>{detail.runtime}</span>
                    </div>
                )}
                {(detail.year || detail.releaseInfo) && (
                    <div className="flex items-center gap-1.5">
                        <Calendar className="size-4" />
                        <span>{detail.year || detail.releaseInfo}</span>
                    </div>
                )}
                {detail.rating && (
                    <div className="flex items-center gap-1.5">
                        <Badge
                            variant="secondary"
                            className="bg-yellow-500/20 text-yellow-400 border-yellow-500/30"
                        >
                            <Star className="size-3.5 mr-1 fill-yellow-400" />
                            {detail.rating}
                        </Badge>
                        <span className="text-white/60 text-sm">IMDb</span>
                    </div>
                )}
                {isSeries && seasons.length > 0 && (
                    <Badge variant="outline" className="border-white/30 text-white/90">
                        {seasons.length} Season{seasons.length > 1 ? "s" : ""}
                    </Badge>
                )}
            </div>

            {detail.genres?.length ? (
                <div className="space-y-2">
                    <h3 className="text-xs uppercase tracking-wider text-white/50">
                        Genres
                    </h3>
                    <div className="flex flex-wrap gap-2">
                        {detail.genres.map((genre, i) => (
                            <Badge
                                key={i}
                                variant="outline"
                                className="border-white/20 text-white/80 hover:bg-white/10"
                            >
                                {genre}
                            </Badge>
                        ))}
                    </div>
                </div>
            ) : null}

            {detail.cast?.length ? (
                <div className="space-y-2">
                    <h3 className="text-xs uppercase tracking-wider text-white/50">
                        Cast
                    </h3>
                    <div className="flex flex-wrap gap-2">
                        {detail.cast.slice(0, 6).map((member, i) => (
                            <Badge
                                key={i}
                                variant="secondary"
                                className="bg-white/10 text-white/90 hover:bg-white/20"
                            >
                                {member}
                            </Badge>
                        ))}
                    </div>
                </div>
            ) : null}

            {detail.overview && (
                <div className="space-y-2">
                    <h3 className="text-xs uppercase tracking-wider text-white/50">
                        Summary
                    </h3>
                    <p className="text-white/80 leading-relaxed max-w-2xl">
                        {detail.overview}
                    </p>
                </div>
            )}

            <div className="flex flex-wrap items-center gap-3 pt-4">
                <Button
                    variant="outline"
                    className="gap-2 bg-white/10 border-white/20 text-white hover:bg-white/20"
                >
                    <Plus className="size-4" />
                    Add to Library
                </Button>
                {detail.trailers?.length > 0 && (
                    <Button
                        variant="outline"
                        className="gap-2 bg-white/10 border-white/20 text-white hover:bg-white/20"
                        onClick={openTrailer}
                    >
                        <Play className="size-4" />
                        Trailer
                    </Button>
                )}
                <Button
                    onClick={onFindTorrents}
                    disabled={searchingTorrents}
                    className="gap-2 bg-primary hover:bg-primary/90"
                >
                    {searchingTorrents ? (
                        <Loader2 className="size-4 animate-spin" />
                    ) : (
                        <Search className="size-4" />
                    )}
                    Find Torrents
                </Button>
                {detail.id && onImdb && (
                    <Button
                        variant="outline"
                        className="gap-2 bg-white/10 border-white/20 text-white hover:bg-white/20"
                        onClick={onImdb}
                    >
                        <ExternalLink className="size-4" />
                        IMDb
                    </Button>
                )}
            </div>
        </div>
    );
}
