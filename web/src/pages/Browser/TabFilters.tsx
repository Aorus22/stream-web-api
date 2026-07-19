import type { Dispatch, SetStateAction } from "react";
import { Film, Tv, TrendingUp, Star, Filter } from "lucide-react";
import { Button } from "@/components/ui/button";
import { TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { GENRES } from "./types";
import type { Category } from "./types";

type TabFiltersProps = {
    category: Category;
    setCategory: (value: Category) => void;
    selectedGenre: string;
    setSelectedGenre: (genre: string) => void;
    setCategorySkip: Dispatch<SetStateAction<number>>;
    showControls: boolean;
};

export default function TabFilters({
    category,
    setCategory,
    selectedGenre,
    setSelectedGenre,
    setCategorySkip,
    showControls
}: TabFiltersProps) {
    return (
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
            <TabsList className="bg-muted/50">
                <TabsTrigger value="movies" className="gap-2">
                    <Film className="size-4" />
                    Movies
                </TabsTrigger>
                <TabsTrigger value="series" className="gap-2">
                    <Tv className="size-4" />
                    TV Series
                </TabsTrigger>
            </TabsList>

            {showControls && (
                <div className="flex gap-2 flex-wrap items-center">
                    <Button
                        variant={category === "popular" ? "default" : "outline"}
                        size="sm"
                        onClick={() => { setCategory("popular"); setCategorySkip(0); }}
                        className="gap-1.5"
                    >
                        <TrendingUp className="size-4" />
                        Popular
                    </Button>
                    <Button
                        variant={category === "top-rated" ? "default" : "outline"}
                        size="sm"
                        onClick={() => { setCategory("top-rated"); setCategorySkip(0); }}
                        className="gap-1.5"
                    >
                        <Star className="size-4 fill-current" />
                        Top Rated
                    </Button>

                    <Select
                        value={category === "genre" ? selectedGenre : ""}
                        onValueChange={(value) => {
                            setSelectedGenre(value);
                            setCategory("genre");
                            setCategorySkip(0);
                        }}
                    >
                        <SelectTrigger className={cn(
                            "w-[140px] h-9",
                            category === "genre" && "ring-2 ring-primary"
                        )}>
                            <Filter className="size-4 mr-1" />
                            <SelectValue placeholder="Choose Genre" />
                        </SelectTrigger>
                        <SelectContent>
                            {GENRES.map((genre) => (
                                <SelectItem key={genre} value={genre}>
                                    {genre}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
            )}
        </div>
    );
}
