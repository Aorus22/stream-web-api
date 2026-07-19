export type TrailerInfo = {
    source: string;
    type: string;
};

export type EpisodeInfo = {
    id: string;
    title: string;
    season: number;
    episode: number;
    released: string;
    thumbnail: string;
    overview: string;
};

export type MediaInfo = {
    id: string; // IMDb ID
    title: string;
    overview: string;
    poster: string;
    backdrop: string;
    logo: string;
    releaseInfo: string;
    year: string;
    rating: string;
    runtime: string;
    genres: string[];
    cast: string[];
    director: string[];
    writer: string[];
    mediaType: "movie" | "series";
    trailers: TrailerInfo[];
    episodes: EpisodeInfo[];
};

export type TorrentResult = {
    name: string;
    magnet: string;
    size: string;
    seeders: string;
    leechers: string;
    category: string;
    uploadedBy: string;
    dateUploaded: string;
    url?: string;
};

export type ProviderInfo = {
    id: string;
    name: string;
    type: "embedded" | "custom";
    pageType: "list" | "detail";
};
