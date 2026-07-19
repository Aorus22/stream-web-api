import { Search as SearchIcon } from "lucide-react";

export default function SearchHeader() {
  return (
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-4">
        <div className="p-3 bg-primary/10 rounded-2xl ring-1 ring-primary/20">
          <SearchIcon className="size-8 text-primary" />
        </div>
        <div>
          <h1 className="text-3xl md:text-4xl font-bold tracking-tight">Torrent Search</h1>
          <p className="text-muted-foreground text-sm md:text-base">Search for torrents across multiple providers</p>
        </div>
      </div>
    </div>
  );
}
