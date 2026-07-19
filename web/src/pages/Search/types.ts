export type SearchResult = {
  name: string;
  magnet: string;
  poster: string;
  category: string;
  type: string;
  language: string;
  size: string;
  uploadedBy: string;
  downloads: string;
  lastChecked: string;
  dateUploaded: string;
  seeders: string;
  leechers: string;
  url: string;
};

export type ProviderInfo = {
  id: string;
  name: string;
  type: "embedded" | "custom";
  pageType: "list" | "detail";
};

export type ProviderSelectProps = {
  providers: ProviderInfo[];
  selected: string;
  onChange: (value: string) => void;
};
