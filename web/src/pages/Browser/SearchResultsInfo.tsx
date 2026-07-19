import { Button } from "@/components/ui/button";
import { Waves } from "lucide-react";
import type { MediaItem } from "./types";

type SearchResultsInfoProps = {
    searchQuery: string;
    hasSearched: boolean;
    isSearching: boolean;
    searchResults: MediaItem[];
    clearSearch: () => void;
};

export default function SearchResultsInfo({ searchQuery, hasSearched, isSearching, searchResults, clearSearch }: SearchResultsInfoProps) {
    if (!searchQuery && !hasSearched) return null;

    return (
        <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
                {isSearching ? (
                    <>
                        <div className="size-4 border-2 border-primary border-t-transparent rounded-full animate-spin" />
                        <h2 className="text-lg font-semibold">Searching...</h2>
                    </>
                ) : (
                    <>
                        <Waves className="size-5 text-primary" />
                        <h2 className="text-lg font-semibold">
                            {searchResults.length} result{searchResults.length !== 1 ? 's' : ''} for "{searchQuery}"
                        </h2>
                    </>
                )}
            </div>
            <Button variant="ghost" size="sm" onClick={clearSearch}>
                Clear
            </Button>
        </div>
    );
}
