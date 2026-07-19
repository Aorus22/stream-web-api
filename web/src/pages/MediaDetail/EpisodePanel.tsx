import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ChevronLeft, ChevronRight, Tv, Play } from "lucide-react";
import type { EpisodeInfo, MediaInfo } from "./types";
import { formatDate } from "./utils";

type EpisodePanelProps = {
    detail: MediaInfo;
    seasons: number[];
    selectedSeason: number;
    setSelectedSeason: (season: number) => void;
    filteredEpisodes: EpisodeInfo[];
    searchTorrents: (query?: string, episode?: EpisodeInfo) => void;
};

export default function EpisodePanel({
    detail,
    seasons,
    selectedSeason,
    setSelectedSeason,
    filteredEpisodes,
    searchTorrents,
}: EpisodePanelProps) {
    return (
        <div className="lg:w-[560px] bg-background/80 backdrop-blur-md rounded-xl border border-white/10 overflow-hidden">
            <div className="p-4 border-b border-white/10 flex items-center gap-2">
                <Button
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    disabled={seasons.indexOf(selectedSeason) <= 0}
                    onClick={() => {
                        const idx = seasons.indexOf(selectedSeason);
                        if (idx > 0) setSelectedSeason(seasons[idx - 1]);
                    }}
                >
                    <ChevronLeft className="size-4" />
                </Button>
                <Select
                    value={String(selectedSeason)}
                    onValueChange={(value) => setSelectedSeason(Number(value))}
                >
                    <SelectTrigger className="flex-1 bg-transparent border-white/20">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {seasons.map((season) => (
                            <SelectItem key={season} value={String(season)}>
                                Season {season}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
                <Button
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    disabled={seasons.indexOf(selectedSeason) >= seasons.length - 1}
                    onClick={() => {
                        const idx = seasons.indexOf(selectedSeason);
                        if (idx < seasons.length - 1) setSelectedSeason(seasons[idx + 1]);
                    }}
                >
                    <ChevronRight className="size-4" />
                </Button>
            </div>

            <ScrollArea className="h-[520px] lg:h-[520px]">
                <div className="p-2 space-y-1">
                    {filteredEpisodes.length === 0 ? (
                        <div className="p-4 text-center text-muted-foreground">
                            No episodes found
                        </div>
                    ) : (
                        filteredEpisodes.map((episode) => (
                            <div
                                key={episode.id}
                                className="flex gap-3 p-2 rounded-lg hover:bg-white/5 cursor-pointer transition-colors group"
                                onClick={() => {
                                    const query = `${detail.title} S${String(episode.season).padStart(2, '0')}E${String(episode.episode).padStart(2, '0')}`;
                                    searchTorrents(query, episode);
                                }}
                            >
                                <div className="relative w-24 h-14 rounded overflow-hidden bg-muted flex-shrink-0">
                                    {episode.thumbnail ? (
                                        <img
                                            src={episode.thumbnail}
                                            alt={episode.title}
                                            className="w-full h-full object-cover"
                                        />
                                    ) : (
                                        <div className="w-full h-full flex items-center justify-center">
                                            <Tv className="size-6 text-muted-foreground" />
                                        </div>
                                    )}
                                    <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                                        <Play className="size-6 text-white fill-white" />
                                    </div>
                                </div>
                                <div className="flex-1 min-w-0">
                                    <h4 className="text-sm font-medium truncate">
                                        {episode.episode}. {episode.title}
                                    </h4>
                                    <p className="text-xs text-muted-foreground">
                                        {formatDate(episode.released)}
                                    </p>
                                </div>
                            </div>
                        ))
                    )}
                </div>
            </ScrollArea>
        </div>
    );
}
