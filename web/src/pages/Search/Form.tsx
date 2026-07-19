import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Loader2, Search as SearchIcon } from "lucide-react";
import type { FormEvent } from "react";
import type { ProviderInfo } from "./types";

type SearchFormProps = {
  providers: ProviderInfo[];
  selectedProvider: string;
  onProviderChange: (value: string) => void;
  query: string;
  onQueryChange: (value: string) => void;
  loading: boolean;
  onSubmit: (e: FormEvent<HTMLFormElement>) => void;
};

export default function SearchForm({
  providers,
  selectedProvider,
  onProviderChange,
  query,
  onQueryChange,
  loading,
  onSubmit,
}: SearchFormProps) {
  return (
    <form onSubmit={onSubmit} className="flex flex-col md:flex-row gap-4">
      <Select value={selectedProvider} onValueChange={onProviderChange}>
        <SelectTrigger className="w-full md:w-[200px]">
          <SelectValue placeholder="Select Provider" />
        </SelectTrigger>
        <SelectContent>
          {providers.map(p => (
            <SelectItem key={p.id} value={p.id}>
              {p.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        placeholder="Search query..."
        value={query}
        onChange={(e) => onQueryChange(e.target.value)}
        className="flex-1"
      />
      <Button type="submit" disabled={loading}>
        {loading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <SearchIcon className="mr-2 h-4 w-4" />}
        Search
      </Button>
    </form>
  );
}
