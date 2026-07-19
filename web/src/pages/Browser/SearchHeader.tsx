import type { FormEvent } from "react";
import { Search as SearchIcon } from "lucide-react";
import { Input } from "@/components/ui/input";

type SearchHeaderProps = {
    searchQuery: string;
    setSearchQuery: (value: string) => void;
    handleSearch: (e: FormEvent<HTMLFormElement>) => void;
    clearSearch: () => void;
};

export default function SearchHeader({ searchQuery, setSearchQuery, handleSearch, clearSearch }: SearchHeaderProps) {
    return (
        <header className="absolute top-0 left-0 right-0 z-30 bg-background/80 backdrop-blur-lg border-b border-border">
            <div className="px-4 sm:px-6 lg:px-8 py-4">
                <div className="flex items-center justify-center gap-4">
                    <form onSubmit={handleSearch} className="w-full max-w-xl">
                        <div className="relative">
                            <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
                            <Input
                                type="text"
                                placeholder="Search movies, TV shows..."
                                value={searchQuery}
                                onChange={(e) => setSearchQuery(e.target.value)}
                                className="pl-10 pr-10 h-10 bg-muted/50 focus-visible:bg-muted focus-visible:ring-2 focus-visible:ring-primary"
                            />
                            {searchQuery && (
                                <button
                                    type="button"
                                    onClick={clearSearch}
                                    className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                                >
                                    <span className="sr-only">Clear</span>
                                    <svg className="size-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                    </svg>
                                </button>
                            )}
                        </div>
                    </form>
                </div>
            </div>
        </header>
    );
}
