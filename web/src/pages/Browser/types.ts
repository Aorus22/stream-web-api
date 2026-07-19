export type MediaItem = {
    id: string; // IMDb ID (tt1234567)
    title: string;
    overview: string;
    poster: string;
    backdrop: string;
    releaseInfo: string;
    year: string;
    rating: string;
    runtime: string;
    mediaType: "movie" | "series";
    genres: string[];
};

export type CatalogResponse = {
    results: MediaItem[];
    hasMore: boolean;
};

export type Category = "popular" | "top-rated" | "genre";

export const GENRES = [
    "Action",
    "Adventure",
    "Animation",
    "Biography",
    "Comedy",
    "Crime",
    "Documentary",
    "Drama",
    "Family",
    "Fantasy",
    "History",
    "Horror",
    "Music",
    "Mystery",
    "Romance",
    "Sci-Fi",
    "Sport",
    "Thriller",
    "War",
    "Western",
];
