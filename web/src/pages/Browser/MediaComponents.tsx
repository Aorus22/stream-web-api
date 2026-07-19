import { memo } from "react";
import { Link } from "react-router-dom";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Film, Star } from "lucide-react";
import { cn } from "@/lib/utils";
import type { MediaItem } from "./types";

export const MediaCard = memo(function MediaCard({ item }: { item: MediaItem }) {
    const detailPath = item.mediaType === "series" ? `/series/${item.id}` : `/movie/${item.id}`;

    return (
        <Link 
            to={detailPath} 
            className="group block" 
        >
            <Card className="p-0 overflow-hidden border-0 bg-transparent hover:shadow-xl hover:shadow-primary/10 transition-all duration-300 transform-gpu">
                <div className="aspect-[2/3] relative rounded-xl overflow-hidden bg-muted/50">
                    {item.poster ? (
                        <img
                            src={item.poster}
                            alt={item.title}
                            className="w-full h-full object-cover transition-transform duration-500 group-hover:scale-105"
                            loading="lazy"
                            decoding="async"
                        />
                    ) : (
                        <div className="w-full h-full flex items-center justify-center">
                            <Film className="size-12 text-muted-foreground/50" />
                        </div>
                    )}
                    {item.rating && (
                        <div className="absolute top-2 left-2">
                            <Badge className="bg-black/70 text-yellow-400 border-yellow-500/30 text-[10px] px-1.5 h-5 flex items-center gap-0.5 font-bold border backdrop-blur-md">
                                <Star className="size-2.5 fill-yellow-400" />
                                {item.rating}
                            </Badge>
                        </div>
                    )}
                    <div className="absolute top-2 right-2">
                        <Badge
                            className={cn(
                                "text-[10px] px-1.5 h-5 flex items-center font-bold border backdrop-blur-md text-white border-white/20",
                                item.mediaType === "series"
                                    ? "bg-accent/90"
                                    : "bg-primary/90"
                            )}
                        >
                            {item.mediaType === "series" ? "TV" : "FILM"}
                        </Badge>
                    </div>
                    <div className="absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-black/95 via-black/40 to-transparent opacity-90 group-hover:opacity-100 transition-opacity" />
                    <div className="absolute bottom-0 left-0 right-0 p-3">
                        <h3 className="text-white font-medium text-sm line-clamp-2 drop-shadow-md">{item.title}</h3>
                        {item.year && (
                            <p className="text-white/80 text-xs mt-0.5 drop-shadow-md">{item.year}</p>
                        )}
                    </div>
                </div>
            </Card>
        </Link>
    );
});

export const MediaRow = memo(function MediaRow({ title, items, loading, icon: Icon }: {
    title: string;
    items: MediaItem[];
    loading: boolean;
    icon?: React.ElementType;
}) {
    return (
        <div className="space-y-3">
            <div className="flex items-center gap-2">
                {Icon && <Icon className="size-5 text-primary" />}
                <h2 className="text-lg font-semibold">{title}</h2>
            </div>
            <div className="w-full overflow-x-auto scrollbar-hide flex gap-4 pb-4 -mx-1 px-1">
                {loading ? (
                    Array.from({ length: 8 }).map((_, i) => (
                        <div key={i} className="w-[120px] sm:w-[150px] flex-shrink-0">
                            <Skeleton className="aspect-[2/3] w-full rounded-xl" />
                        </div>
                    ))
                ) : (
                    items.map((item) => (
                        <div key={item.id} className="w-[120px] sm:w-[150px] flex-shrink-0">
                            <MediaCard item={item} />
                        </div>
                    ))
                )}
            </div>
        </div>
    );
});

export const MediaGrid = memo(function MediaGrid({ items, loading }: { items: MediaItem[]; loading: boolean }) {
    if (loading) {
        return (
            <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8 gap-2 sm:gap-4 min-h-[50vh]">
                {Array.from({ length: 18 }).map((_, i) => (
                    <div key={i}>
                        <Skeleton className="aspect-[2/3] w-full rounded-xl" />
                    </div>
                ))}
            </div>
        );
    }

    if (items.length === 0) {
        return (
            <div className="flex flex-col items-center justify-center py-20 text-center">
                <div className="w-20 h-20 rounded-full bg-muted flex items-center justify-center mb-4">
                    <Film className="size-10 text-muted-foreground" />
                </div>
                <h3 className="text-lg font-semibold mb-2">No content found</h3>
                <p className="text-sm text-muted-foreground">Try selecting a different category</p>
            </div>
        );
    }

    return (
        <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8 gap-2 sm:gap-4">
            {items.map((item) => (
                <MediaCard key={item.id} item={item} />
            ))}
        </div>
    );
});
