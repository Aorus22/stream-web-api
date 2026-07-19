import { memo, useEffect } from "react";
import { TabsContent } from "@/components/ui/tabs";
import { Search as SearchIcon } from "lucide-react";
import { MediaGrid, MediaRow } from "./MediaComponents";
import PaginationControls from "./PaginationControls";
import type { MediaItem } from "./types";
import type { Dispatch, ElementType, SetStateAction } from "react";

type BrowseTabPanelProps = {
    value: "movies" | "series";
    searchQuery: string;
    hasSearched: boolean;
    isSearching: boolean;
    searchResults: MediaItem[];
    categoryItems: MediaItem[];
    categoryLoading: boolean;
    categorySkip: number;
    setCategorySkip: Dispatch<SetStateAction<number>>;
    hasMore: boolean;
    rows: {
        title: string;
        items: MediaItem[];
        loading: boolean;
        icon: ElementType;
    }[];
};

const BrowseTabPanel = memo(function BrowseTabPanel({
    value,
    searchQuery,
    hasSearched,
    isSearching,
    searchResults,
    categoryItems,
    categoryLoading,
    categorySkip,
    setCategorySkip,
    hasMore,
    rows
}: BrowseTabPanelProps) {
    useEffect(() => {
        if (categorySkip > 0) {
            const scrollContainer = document.getElementById('main-scroll-container');
            if (scrollContainer) {
                scrollContainer.scrollTo({ top: 0, behavior: 'smooth' });
            }
        }
    }, [categorySkip]);

    return (
        <TabsContent value={value} className="space-y-6">
            {(searchQuery || hasSearched) ? (
                isSearching ? (
                    <MediaGrid items={[]} loading={true} />
                ) : searchResults.length > 0 ? (
                    <MediaGrid items={searchResults} loading={false} />
                ) : (
                    hasSearched && (
                        <div className="text-center py-20">
                            <div className="w-20 h-20 rounded-full bg-muted flex items-center justify-center mx-auto mb-4">
                                <SearchIcon className="size-10 text-muted-foreground" />
                            </div>
                            <h3 className="text-lg font-semibold mb-2">No results found</h3>
                            <p className="text-sm text-muted-foreground">Try different keywords</p>
                        </div>
                    )
                )
            ) : (
                <>
                    <MediaGrid items={categoryItems} loading={categoryLoading} />
                    <PaginationControls
                        categorySkip={categorySkip}
                        hasMore={hasMore}
                        onPrev={() => setCategorySkip(s => Math.max(0, s - 20))}
                        onNext={() => setCategorySkip(s => s + 20)}
                    />
                    {rows.map(row => (
                        <MediaRow
                            key={row.title}
                            title={row.title}
                            items={row.items}
                            loading={row.loading}
                            icon={row.icon}
                        />
                    ))}
                </>
            )}
        </TabsContent>
    );
});

export default BrowseTabPanel;
