import { useEffect, useState, useMemo } from "react";
import type { FormEvent } from "react";
import { TrendingUp, Star } from "lucide-react";
import { Tabs } from "@/components/ui/tabs";
import { useServer } from "@/contexts/ServerContext";
import SearchHeader from "./SearchHeader";
import TabFilters from "./TabFilters";
import SearchResultsInfo from "./SearchResultsInfo";
import BrowseTabPanel from "./BrowseTabPanel";
import type { MediaItem, CatalogResponse, Category } from "./types";

export function Browse() {
    const { serverUrl } = useServer();
    const [activeTab, setActiveTab] = useState<"movies" | "series">("movies");
    const [category, setCategory] = useState<Category>("popular");
    const [selectedGenre, setSelectedGenre] = useState<string>("");
    const [searchQuery, setSearchQuery] = useState("");
    const [searchResults, setSearchResults] = useState<MediaItem[]>([]);
    const [isSearching, setIsSearching] = useState(false);
    const [hasSearched, setHasSearched] = useState(false);

    const [popularMovies, setPopularMovies] = useState<MediaItem[]>([]);
    const [topRatedMovies, setTopRatedMovies] = useState<MediaItem[]>([]);
    const [popularSeries, setPopularSeries] = useState<MediaItem[]>([]);
    const [topRatedSeries, setTopRatedSeries] = useState<MediaItem[]>([]);

    const [categoryItems, setCategoryItems] = useState<MediaItem[]>([]);
    const [categorySkip, setCategorySkip] = useState(0);
    const [hasMore, setHasMore] = useState(true);
    const [categoryLoading, setCategoryLoading] = useState(false);

    const [loadingMovies, setLoadingMovies] = useState(true);
    const [loadingSeries, setLoadingSeries] = useState(true);

    useEffect(() => {
        if (!serverUrl) return;
        const fetchMovies = async () => {
            setLoadingMovies(true);
            try {
                const [popular, topRated] = await Promise.all([
                    fetch(`${serverUrl}/api/catalog/movies`).then((r) => r.json()),
                    fetch(`${serverUrl}/api/catalog/movies/top-rated`).then((r) => r.json()),
                ]);
                setPopularMovies(popular.results || []);
                setTopRatedMovies(topRated.results || []);
            } catch (err) {
                console.error("Failed to fetch movies:", err);
            } finally {
                setLoadingMovies(false);
            }
        };

        const fetchSeries = async () => {
            setLoadingSeries(true);
            try {
                const [popular, topRated] = await Promise.all([
                    fetch(`${serverUrl}/api/catalog/series`).then((r) => r.json()),
                    fetch(`${serverUrl}/api/catalog/series/top-rated`).then((r) => r.json()),
                ]);
                setPopularSeries(popular.results || []);
                setTopRatedSeries(topRated.results || []);
            } catch (err) {
                console.error("Failed to fetch series:", err);
            } finally {
                setLoadingSeries(false);
            }
        };

        fetchMovies();
        fetchSeries();
    }, [serverUrl]);

    useEffect(() => {
        const fetchCategory = async () => {
            if (!serverUrl) return;
            setCategoryLoading(true);
            const mediaType = activeTab;

            let url: string;
            if (category === "genre") {
                url = `${serverUrl}/api/catalog/${mediaType}/genre/${selectedGenre}?skip=${categorySkip}`;
            } else if (category === "top-rated") {
                url = `${serverUrl}/api/catalog/${mediaType}/top-rated?skip=${categorySkip}`;
            } else {
                url = `${serverUrl}/api/catalog/${mediaType}?skip=${categorySkip}`;
            }

            try {
                const res = await fetch(url);
                const data: CatalogResponse = await res.json();
                setCategoryItems(data.results || []);
                setHasMore(data.hasMore);
            } catch (err) {
                console.error("Failed to fetch category:", err);
            } finally {
                setCategoryLoading(false);
            }
        };

        fetchCategory();
    }, [category, activeTab, categorySkip, selectedGenre, serverUrl]);

    useEffect(() => {
        if (hasSearched && searchQuery && serverUrl) {
            const performSearch = async () => {
                setIsSearching(true);
                setSearchResults([]);
                try {
                    const mediaType = activeTab;
                    const res = await fetch(
                        `${serverUrl}/api/catalog/${mediaType}/search?q=${encodeURIComponent(searchQuery)}`
                    );
                    const data: CatalogResponse = await res.json();
                    setSearchResults(data.results || []);
                } catch (err) {
                    console.error("Failed to search:", err);
                } finally {
                    setIsSearching(false);
                }
            };
            performSearch();
        }
    }, [activeTab, serverUrl, hasSearched, searchQuery]);

    const handleSearch = async (e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        if (!searchQuery.trim() || !serverUrl) return;

        setIsSearching(true);
        setHasSearched(true);
        setSearchResults([]);
        try {
            const mediaType = activeTab;
            const res = await fetch(
                `${serverUrl}/api/catalog/${mediaType}/search?q=${encodeURIComponent(searchQuery)}`
            );
            const data: CatalogResponse = await res.json();
            setSearchResults(data.results || []);
        } catch (err) {
            console.error("Failed to search:", err);
        } finally {
            setIsSearching(false);
        }
    };

    const clearSearch = () => {
        setSearchQuery("");
        setSearchResults([]);
        setHasSearched(false);
    };

    const movieRows = useMemo(() => [
        {
            title: "Popular Movies",
            items: popularMovies,
            loading: loadingMovies,
            icon: TrendingUp,
        },
        {
            title: "Top Rated Movies",
            items: topRatedMovies,
            loading: loadingMovies,
            icon: Star,
        },
    ], [popularMovies, topRatedMovies, loadingMovies]);

    const seriesRows = useMemo(() => [
        {
            title: "Popular TV Series",
            items: popularSeries,
            loading: loadingSeries,
            icon: TrendingUp,
        },
        {
            title: "Top Rated TV Series",
            items: topRatedSeries,
            loading: loadingSeries,
            icon: Star,
        },
    ], [popularSeries, topRatedSeries, loadingSeries]);

    const showFilters = !(searchQuery || hasSearched);
    const handleTabChange = (value: string) => {
        const nextTab = value as "movies" | "series";
        setActiveTab(nextTab);
        setCategorySkip(0);
    };

    return (
        <div className="-mx-4 sm:-mx-6 lg:-mx-8 flex flex-col h-full">
            <SearchHeader
                searchQuery={searchQuery}
                setSearchQuery={setSearchQuery}
                handleSearch={handleSearch}
                clearSearch={clearSearch}
            />

            <main className="px-4 sm:px-6 lg:px-8 py-6 space-y-6 pt-20">
                <Tabs value={activeTab} onValueChange={handleTabChange}>
                    <TabFilters
                        category={category}
                        setCategory={setCategory}
                        selectedGenre={selectedGenre}
                        setSelectedGenre={setSelectedGenre}
                        setCategorySkip={setCategorySkip}
                        showControls={showFilters}
                    />

                    <SearchResultsInfo
                        searchQuery={searchQuery}
                        hasSearched={hasSearched}
                        isSearching={isSearching}
                        searchResults={searchResults}
                        clearSearch={clearSearch}
                    />

                    {activeTab === "movies" && (
                        <BrowseTabPanel
                            value="movies"
                            searchQuery={searchQuery}
                            hasSearched={hasSearched}
                            isSearching={isSearching}
                            searchResults={searchResults}
                            categoryItems={categoryItems}
                            categoryLoading={categoryLoading}
                            categorySkip={categorySkip}
                            setCategorySkip={setCategorySkip}
                            hasMore={hasMore}
                            rows={movieRows}
                        />
                    )}

                    {activeTab === "series" && (
                        <BrowseTabPanel
                            value="series"
                            searchQuery={searchQuery}
                            hasSearched={hasSearched}
                            isSearching={isSearching}
                            searchResults={searchResults}
                            categoryItems={categoryItems}
                            categoryLoading={categoryLoading}
                            categorySkip={categorySkip}
                            setCategorySkip={setCategorySkip}
                            hasMore={hasMore}
                            rows={seriesRows}
                        />
                    )}
                </Tabs>
            </main>
        </div>
    );
}

export default Browse;
