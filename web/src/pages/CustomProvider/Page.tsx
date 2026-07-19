import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Code2, Play, Loader2, Copy, Check, Eye, EyeOff, Globe, ExternalLink } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Empty, EmptyMedia, EmptyTitle, EmptyDescription } from "@/components/ui/empty";
import { useServer } from "@/contexts/ServerContext";
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from "@/components/ui/tooltip";
import Editor from "@monaco-editor/react";

interface CustomProviderResult {
    type?: 'list' | 'detail' | 'error';
    name?: string;
    magnetLink?: string;
    directDownloads?: Array<{ url: string; text: string }>;
    similarFiles?: Array<{ filename: string; size: string; downloadUrl: string }>;
    results?: Array<{
        name: string;
        url: string;
        seeds: number;
        leeches: number;
        size: string;
        uploader: string;
        time: string;
    }>;
    error?: string;
}

interface ExecuteResponse {
    result?: CustomProviderResult;
    error?: string;
}

const DEFAULT_SCRIPT = `const https = require('https');
const cheerio = require('cheerio');

// ARG_FULL_URL is the full URL with placeholders replaced

async function fetchUrl(url) {
  return new Promise((resolve, reject) => {
    https.get(url, (res) => {
      let data = '';
      res.on('data', (chunk) => data += chunk);
      res.on('end', () => resolve(data));
    }).on('error', reject);
  });
}

// ============================================
// FUNCTION 1: Parse List/Search Results Page
// Returns: { type: 'list', results: [...] }
// ============================================
async function parseListPage(fullUrl) {
  try {
    const html = await fetchUrl(fullUrl);
    const $ = cheerio.load(html);

    const results = [];

    // Find table rows with torrent listings - try multiple selectors
    const rows = $('table.table-list tbody tr, table tbody tr, .table-list tr');

    rows.each((i, elem) => {
      const $row = $(elem);

      // Skip header rows (contain th elements)
      if ($row.find('th').length > 0) return;

      // Get all cells in the row
      const cols = $row.find('td');
      if (cols.length < 3) return;

      // Extract name and link from first column
      // Try multiple approaches
      const nameCell = $row.find('td.coll-1.name, td.coll-1, td:nth-child(1)').first();
      const allLinks = nameCell.find('a');
      let name = '';
      let relativeUrl = '';

      // Try to get the second link (often the actual torrent link, first is icon)
      if (allLinks.length >= 2) {
        name = allLinks.eq(1).text().trim();
        relativeUrl = allLinks.eq(1).attr('href');
      } else if (allLinks.length === 1) {
        name = allLinks.eq(0).text().trim();
        relativeUrl = allLinks.eq(0).attr('href');
      }

      // Fallback: try any link in the first column
      if (!name || name.length < 3) {
        const anyLink = cols.first().find('a').first();
        name = anyLink.text().trim();
        relativeUrl = anyLink.attr('href');
      }

      if (!name || name.length < 3 || !relativeUrl) return;

      // Build full URL
      const url = relativeUrl.startsWith('http')
        ? relativeUrl
        : (relativeUrl.startsWith('//')
          ? 'https:' + relativeUrl
          : new URL(relativeUrl, fullUrl).href);

      // Extract seeds from column 2
      let seeds = 0;
      const seedsCol = $row.find('td.coll-2, td:nth-child(2)').first();
      const seedsText = seedsCol.text().trim();
      if (seedsText) {
        const parsed = parseInt(seedsText.replace(/,/g, ''));
        if (!isNaN(parsed)) seeds = parsed;
      }

      // Extract leeches from column 3
      let leeches = 0;
      const leechesCol = $row.find('td.coll-3, td:nth-child(3)').first();
      const leechesText = leechesCol.text().trim();
      if (leechesText) {
        const parsed = parseInt(leechesText.replace(/,/g, ''));
        if (!isNaN(parsed)) leeches = parsed;
      }

      // Extract date/time from coll-date or column 4
      let time = '';
      const timeCol = $row.find('td.coll-date, td:nth-child(4)').first();
      time = timeCol.text().trim();

      // Extract size from column 5 (or size column)
      let size = 'Unknown';
      const sizeCol = $row.find('td.coll-4.size, td.coll-4, td.size, td:nth-child(5)').first();
      if (sizeCol.length > 0) {
        const sizeText = sizeCol.text().trim();
        size = sizeText.split(new RegExp('\\n'))[0] || sizeText;
      }

      // Extract uploader from last column
      const uploader = $row.find('td.coll-5 a, td.uploader a, td:last-child a').first().text().trim() || 'Unknown';

      results.push({ name, url, seeds, leeches, size, uploader, time });
    });

    return {
      type: 'list',
      results
    };
  } catch (error) {
    return {
      type: 'error',
      error: error.message
    };
  }
}

// ============================================
// FUNCTION 2: Parse Detail Page (with magnet link)
// Returns: { type: 'detail', name, magnetLink, directDownloads, ... }
// ============================================
async function parseDetailPage(fullUrl) {
  try {
    const html = await fetchUrl(fullUrl);
    const $ = cheerio.load(html);

    // Extract name
    const name = $('h1, title, .title, .name').first().text().trim() || 'Unknown';

    // Extract magnet link
    let magnetLink = '';
    $('a[href^="magnet:"]').each((i, elem) => {
      magnetLink = $(elem).attr('href');
      return false;
    });

    // Extract direct download links
    const directDownloads = [];
    $('a[href*="download"], a[href*=".torrent"]').each((i, elem) => {
      const href = $(elem).attr('href');
      if (href && !href.startsWith('magnet:') && !href.startsWith('#')) {
        directDownloads.push({
          url: href.startsWith('http') ? href : new URL(href, fullUrl).href,
          text: $(elem).text().trim() || 'Download'
        });
      }
    });

    return {
      type: 'detail',
      name,
      magnetLink,
      directDownloads,
      similarFiles: []
    };
  } catch (error) {
    return {
      type: 'error',
      name: 'Error',
      magnetLink: '',
      directDownloads: [],
      similarFiles: [],
      error: error.message
    };
  }
}

// ============================================
// MAIN ENTRY POINT
// ARG_FULL_URL: The full URL to scrape
// ARG_PAGE_TYPE: 'list' for search results, 'detail' for magnet link pages
// ============================================

// Choose which function to use based on ARG_PAGE_TYPE
if (ARG_PAGE_TYPE === 'list') {
  return await parseListPage(ARG_FULL_URL);
} else if (ARG_PAGE_TYPE === 'detail') {
  return await parseDetailPage(ARG_FULL_URL);
} else {
  // Default to list if page type is not recognized
  return await parseListPage(ARG_FULL_URL);
}
`;

export function CustomProviderPage() {
    const { serverUrl } = useServer();
    const navigate = useNavigate();
    const [urlTemplate, setUrlTemplate] = useState("");
    const [queryValue, setQueryValue] = useState("");
    const [pageType, setPageType] = useState<"list" | "detail">("list");
    const [code, setCode] = useState(DEFAULT_SCRIPT);
    const [loading, setLoading] = useState(false);
    const [previewing, setPreviewing] = useState(false);
    const [htmlPreview, setHtmlPreview] = useState<string | null>(null);
    const [showHtmlPreview, setShowHtmlPreview] = useState(false);
    const [htmlPreviewTab, setHtmlPreviewTab] = useState<"raw" | "rendered">("raw");
    const [result, setResult] = useState<ExecuteResponse | null>(null);
    const [copied, setCopied] = useState(false);

    // Build full URL by replacing placeholders
    const buildFullUrl = (template: string, query: string): string => {
        if (!template) return "";
        // Replace {q}, {query}, {search} with the query value
        return template
            .replace(/\{q\}/g, encodeURIComponent(query))
            .replace(/\{query\}/g, encodeURIComponent(query))
            .replace(/\{search\}/g, encodeURIComponent(query))
            .replace(/\{raw_q\}/g, query)
            .replace(/\{raw_query\}/g, query)
            .replace(/\{raw_search\}/g, query);
    };

    const previewHtml = async () => {
        if (!urlTemplate || !serverUrl) return;

        setPreviewing(true);
        setHtmlPreview(null);

        try {
            // Build full URL with query replacement
            const fullUrl = queryValue
                ? buildFullUrl(urlTemplate, queryValue)
                : urlTemplate;

            const response = await fetch(`${serverUrl}/api/js/preview`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ url: fullUrl }),
            });

            if (!response.ok) {
                throw new Error("Failed to fetch HTML preview");
            }

            const text = await response.text();
            setHtmlPreview(text);
            setShowHtmlPreview(true);
        } catch (err) {
            setHtmlPreview(`Error: ${err instanceof Error ? err.message : 'Unknown error'}`);
            setShowHtmlPreview(true);
        } finally {
            setPreviewing(false);
        }
    };

    const executeScript = async () => {
        if (!urlTemplate || !queryValue || !serverUrl) return;

        setLoading(true);
        setResult(null);

        try {
            // Build full URL with query replacement
            const fullUrl = buildFullUrl(urlTemplate, queryValue);

            // Encode code to base64 before sending to avoid encoding issues
            const base64Code = btoa(unescape(encodeURIComponent(code)));

            const response = await fetch(`${serverUrl}/api/js/execute`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    code: base64Code,
                    url: fullUrl,
                    pageType: pageType,
                    isBase64: true
                }),
            });

            const data = await response.json();
            setResult(data);
        } catch (err) {
            setResult({ error: "Failed to execute script. Check if server is running." });
        } finally {
            setLoading(false);
        }
    };

    const copyResult = () => {
        if (result) {
            navigator.clipboard.writeText(JSON.stringify(result, null, 2));
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        }
    };

    const copyHtmlPreview = () => {
        if (htmlPreview) {
            navigator.clipboard.writeText(htmlPreview);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        }
    };

    return (
        <TooltipProvider>
            <div className="space-y-6">
                {/* Header */}
                <div className="flex items-center gap-3">
                    <div className="p-2 bg-purple-500/10 rounded-xl ring-1 ring-purple-500/20">
                        <Code2 className="size-6 text-purple-500" />
                    </div>
                    <div>
                        <h1 className="text-2xl md:text-3xl font-bold tracking-tight">Custom Provider</h1>
                        <p className="text-muted-foreground text-xs md:text-sm">
                            Run custom JavaScript to scrape torrent pages
                        </p>
                    </div>
                </div>

                <div className="grid gap-6 lg:grid-cols-2">
                    {/* Input Section */}
                    <div className="space-y-4">
                        <Card>
                            <CardContent className="pt-4 space-y-4">
                                {/* Base URL Input */}
                                <div className="space-y-2">
                                    <label className="text-sm font-medium flex items-center gap-2">
                                        <Globe className="size-4" />
                                        URL Template
                                    </label>
                                    <Input
                                        placeholder="https://example.com/search?q={q} or https://example.com/{q}"
                                        value={urlTemplate}
                                        onChange={(e) => setUrlTemplate(e.target.value)}
                                        className="h-11 font-mono text-sm"
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        Use <code className="bg-muted px-1 py-0.5 rounded">{'{q}'}</code>, <code className="bg-muted px-1 py-0.5 rounded">{'{query}'}</code>, or <code className="bg-muted px-1 py-0.5 rounded">{'{search}'}</code> as placeholder. Use <code className="bg-muted px-1 py-0.5 rounded">{'{raw_q}'}</code> for no encoding.
                                    </p>
                                </div>

                                {/* Query Value Input */}
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">Query Value (Optional for Preview)</label>
                                    <Input
                                        placeholder="e.g., Your Name, movie title, search term..."
                                        value={queryValue}
                                        onChange={(e) => setQueryValue(e.target.value)}
                                        className="h-11"
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        This will replace the placeholder in the URL template. Leave empty to preview the template URL directly.
                                    </p>
                                </div>

                                {/* Page Type Selector */}
                                <div className="space-y-2">
                                    <label className="text-sm font-medium">Page Type</label>
                                    <div className="flex gap-4">
                                        <label className="flex items-center gap-2 cursor-pointer">
                                            <input
                                                type="radio"
                                                name="pageType"
                                                checked={pageType === "list"}
                                                onChange={() => setPageType("list")}
                                                className="accent-primary"
                                            />
                                            <span className="text-sm">List (Search Results)</span>
                                        </label>
                                        <label className="flex items-center gap-2 cursor-pointer">
                                            <input
                                                type="radio"
                                                name="pageType"
                                                checked={pageType === "detail"}
                                                onChange={() => setPageType("detail")}
                                                className="accent-primary"
                                            />
                                            <span className="text-sm">Detail (Magnet Link)</span>
                                        </label>
                                    </div>
                                    <p className="text-xs text-muted-foreground">
                                        {pageType === "list"
                                            ? "Parses torrent list from search results table"
                                            : "Parses magnet link and downloads from detail page"}
                                    </p>
                                </div>

                                {/* Preview Button */}
                                <Button
                                    onClick={previewHtml}
                                    disabled={!urlTemplate || previewing || !serverUrl}
                                    variant="outline"
                                    className="w-full gap-2"
                                >
                                    {previewing ? (
                                        <>
                                            <Loader2 className="size-4 animate-spin" />
                                            Fetching HTML...
                                        </>
                                    ) : (
                                        <>
                                            <Eye className="size-4" />
                                            Preview HTML
                                        </>
                                    )}
                                </Button>

                                {/* HTML Preview */}
                                {htmlPreview && (
                                    <div className="space-y-2">
                                        <div className="flex items-center justify-between">
                                            <label className="text-sm font-medium">HTML Preview</label>
                                            <div className="flex gap-1">
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() => setShowHtmlPreview(!showHtmlPreview)}
                                                    className="h-7 text-xs"
                                                >
                                                    {showHtmlPreview ? <EyeOff className="size-3" /> : <Eye className="size-3" />}
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={() => navigate(`/custom-provider/preview/${btoa(encodeURIComponent(htmlPreview))}`)}
                                                    className="h-7 text-xs gap-1"
                                                >
                                                    <ExternalLink className="size-3" />
                                                    Open Full Page
                                                </Button>
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    onClick={copyHtmlPreview}
                                                    className="h-7 text-xs"
                                                >
                                                    <Copy className="size-3" />
                                                </Button>
                                            </div>
                                        </div>
                                        {showHtmlPreview && (
                                            <div className="space-y-2">
                                                {/* Tabs */}
                                                <div className="flex gap-1 border-b">
                                                    <button
                                                        onClick={() => setHtmlPreviewTab("raw")}
                                                        className={`px-3 py-1.5 text-xs font-medium border-b-2 transition-colors ${
                                                            htmlPreviewTab === "raw"
                                                                ? "border-primary text-primary"
                                                                : "border-transparent text-muted-foreground hover:text-foreground"
                                                        }`}
                                                    >
                                                        Raw HTML
                                                    </button>
                                                    <button
                                                        onClick={() => setHtmlPreviewTab("rendered")}
                                                        className={`px-3 py-1.5 text-xs font-medium border-b-2 transition-colors ${
                                                            htmlPreviewTab === "rendered"
                                                                ? "border-primary text-primary"
                                                                : "border-transparent text-muted-foreground hover:text-foreground"
                                                        }`}
                                                    >
                                                        Rendered
                                                    </button>
                                                </div>

                                                {/* Tab Content */}
                                                {htmlPreviewTab === "raw" ? (
                                                    <div className="space-y-1">
                                                        <div className="flex items-center justify-between px-1">
                                                            <span className="text-xs text-muted-foreground">Source Code</span>
                                                            <span className="text-xs text-muted-foreground">{htmlPreview.length} chars</span>
                                                        </div>
                                                        <pre className="text-xs bg-muted p-2 rounded h-80 overflow-auto font-mono whitespace-pre-wrap break-all">
                                                            {htmlPreview}
                                                        </pre>
                                                    </div>
                                                ) : (
                                                    <iframe
                                                        srcDoc={htmlPreview}
                                                        className="w-full h-80 rounded border bg-background"
                                                        sandbox="allow-same-origin allow-scripts"
                                                        title="HTML Preview"
                                                    />
                                                )}
                                            </div>
                                        )}
                                    </div>
                                )}

                                <div className="border-t" />

                                {/* Code Editor */}
                                <div className="space-y-2">
                                    <div className="flex items-center justify-between">
                                        <label className="text-sm font-medium">JavaScript Code</label>
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            onClick={() => setCode(DEFAULT_SCRIPT)}
                                            className="h-7 text-xs"
                                        >
                                            Reset to Default
                                        </Button>
                                    </div>
                                    <div className="border rounded-md overflow-hidden">
                                        <Editor
                                            height="400px"
                                            defaultLanguage="javascript"
                                            value={code}
                                            onChange={(value) => setCode(value || "")}
                                            theme="vs-dark"
                                            options={{
                                                minimap: { enabled: false },
                                                fontSize: 13,
                                                lineNumbers: "on",
                                                scrollBeyondLastLine: false,
                                                automaticLayout: true,
                                                tabSize: 2,
                                            }}
                                        />
                                    </div>
                                    <p className="text-xs text-muted-foreground">
                                        Available variable:{" "}
                                        <code className="bg-muted px-1 py-0.5 rounded">ARG_FULL_URL</code>
                                        {" • "}
                                        Node.js modules: <code className="bg-muted px-1 py-0.5 rounded">require()</code>
                                    </p>
                                </div>

                                {/* Execute Button */}
                                <Button
                                    onClick={executeScript}
                                    disabled={!urlTemplate || !queryValue || !code || loading || !serverUrl}
                                    size="lg"
                                    className="w-full gap-2"
                                >
                                    {loading ? (
                                        <>
                                            <Loader2 className="size-5 animate-spin" />
                                            Running...
                                        </>
                                    ) : (
                                        <>
                                            <Play className="size-5" fill="currentColor" />
                                            Run {pageType === "list" ? "List Parser" : "Detail Parser"}
                                        </>
                                    )}
                                </Button>
                            </CardContent>
                        </Card>
                    </div>

                    {/* Results Section */}
                    <div className="space-y-4">
                        <Card className="min-h-[500px]">
                            <CardContent className="pt-4 h-full">
                                {result ? (
                                    <div className="space-y-4">
                                        <div className="flex items-center justify-between">
                                            <h3 className="font-semibold">Results</h3>
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                onClick={copyResult}
                                                className="gap-1.5 h-8"
                                            >
                                                {copied ? (
                                                    <>
                                                        <Check className="size-4" />
                                                        Copied!
                                                    </>
                                                ) : (
                                                    <>
                                                        <Copy className="size-4" />
                                                        Copy JSON
                                                    </>
                                                )}
                                            </Button>
                                        </div>

                                        {result.error ? (
                                            <div className="p-4 bg-destructive/10 border border-destructive/20 rounded-lg">
                                                <p className="text-sm text-destructive font-mono whitespace-pre-wrap">
                                                    {result.error}
                                                </p>
                                            </div>
                                        ) : result.result?.type === 'list' ? (
                                            /* List Type - Search Results */
                                            <div className="space-y-3">
                                                <div className="flex items-center justify-between">
                                                    <p className="text-sm text-muted-foreground">
                                                        Found {result.result.results?.length || 0} results
                                                    </p>
                                                    <Badge variant="secondary">List</Badge>
                                                </div>
                                                <div className="max-h-96 overflow-y-auto space-y-2">
                                                    {result.result.results?.map((item, i) => (
                                                        <div key={i} className="p-3 bg-muted rounded-lg space-y-2 hover:bg-muted/80 transition-colors">
                                                            <div className="flex items-start justify-between gap-2">
                                                                <div className="flex-1 min-w-0">
                                                                    <p className="font-medium text-sm truncate">{item.name}</p>
                                                                    <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                                                                        <span>{item.size}</span>
                                                                        <span>↑{item.seeds}</span>
                                                                        <span>↓{item.leeches}</span>
                                                                        {item.uploader && <span>by {item.uploader}</span>}
                                                                    </div>
                                                                </div>
                                                                <Tooltip>
                                                                    <TooltipTrigger asChild>
                                                                        <Button
                                                                            variant="ghost"
                                                                            size="sm"
                                                                            className="h-7 w-7 p-0"
                                                                            onClick={() => navigator.clipboard.writeText(item.url)}
                                                                        >
                                                                            <Copy className="size-3" />
                                                                        </Button>
                                                                    </TooltipTrigger>
                                                                    <TooltipContent>Copy URL</TooltipContent>
                                                                </Tooltip>
                                                            </div>
                                                        </div>
                                                    )) || <p className="text-sm text-muted-foreground">No results found</p>}
                                                </div>
                                            </div>
                                        ) : result.result ? (
                                            /* Detail Type - Single Torrent */
                                            <div className="space-y-4">
                                                {/* Name */}
                                                <div className="space-y-1">
                                                    <p className="text-xs text-muted-foreground uppercase tracking-wide">Name</p>
                                                    <p className="font-medium">{result.result.name || '-'}</p>
                                                </div>

                                                {/* Magnet Link */}
                                                <div className="space-y-1">
                                                    <p className="text-xs text-muted-foreground uppercase tracking-wide">Magnet Link</p>
                                                    {result.result?.magnetLink ? (
                                                        <div className="flex gap-2">
                                                            <code className="flex-1 text-xs bg-muted p-2 rounded overflow-hidden text-ellipsis">
                                                                {result.result.magnetLink}
                                                            </code>
                                                            <Tooltip>
                                                                <TooltipTrigger asChild>
                                                                    <Button
                                                                        variant="outline"
                                                                        size="sm"
                                                                        onClick={() => result.result?.magnetLink && navigator.clipboard.writeText(result.result.magnetLink)}
                                                                    >
                                                                        <Copy className="size-4" />
                                                                    </Button>
                                                                </TooltipTrigger>
                                                                <TooltipContent>Copy magnet link</TooltipContent>
                                                            </Tooltip>
                                                        </div>
                                                    ) : (
                                                        <p className="text-muted-foreground text-sm">No magnet link found</p>
                                                    )}
                                                </div>

                                                {/* Direct Downloads */}
                                                <div className="space-y-2">
                                                    <p className="text-xs text-muted-foreground uppercase tracking-wide">
                                                        Direct Downloads ({result.result.directDownloads?.length || 0})
                                                    </p>
                                                    {result.result.directDownloads && result.result.directDownloads.length > 0 ? (
                                                        <div className="space-y-2 max-h-40 overflow-y-auto">
                                                            {result.result.directDownloads.map((dl, i) => (
                                                                <div key={i} className="flex items-center gap-2 text-sm p-2 bg-muted rounded">
                                                                    <span className="flex-1 truncate">{dl.text || dl.url}</span>
                                                                    <Tooltip>
                                                                        <TooltipTrigger asChild>
                                                                            <Button
                                                                                variant="ghost"
                                                                                size="sm"
                                                                                className="h-6 w-6 p-0"
                                                                                onClick={() => navigator.clipboard.writeText(dl.url)}
                                                                            >
                                                                                <Copy className="size-3" />
                                                                            </Button>
                                                                        </TooltipTrigger>
                                                                        <TooltipContent>Copy URL</TooltipContent>
                                                                    </Tooltip>
                                                                </div>
                                                            ))}
                                                        </div>
                                                    ) : (
                                                        <p className="text-muted-foreground text-sm">No direct downloads found</p>
                                                    )}
                                                </div>

                                                {/* Similar Files */}
                                                <div className="space-y-2">
                                                    <p className="text-xs text-muted-foreground uppercase tracking-wide">
                                                        Similar Files ({result.result.similarFiles?.length || 0})
                                                    </p>
                                                    {result.result.similarFiles && result.result.similarFiles.length > 0 ? (
                                                        <div className="space-y-2 max-h-40 overflow-y-auto">
                                                            {result.result.similarFiles.map((file, i) => (
                                                                <div key={i} className="text-sm p-2 bg-muted rounded space-y-1">
                                                                    <div className="font-medium truncate">{file.filename}</div>
                                                                    <div className="flex items-center justify-between">
                                                                        <Badge variant="secondary" className="text-xs">{file.size}</Badge>
                                                                        <Tooltip>
                                                                            <TooltipTrigger asChild>
                                                                                <Button
                                                                                    variant="ghost"
                                                                                    size="sm"
                                                                                    className="h-6 w-6 p-0"
                                                                                    onClick={() => navigator.clipboard.writeText(file.downloadUrl)}
                                                                                >
                                                                                    <Copy className="size-3" />
                                                                                </Button>
                                                                            </TooltipTrigger>
                                                                            <TooltipContent>Copy URL</TooltipContent>
                                                                        </Tooltip>
                                                                    </div>
                                                                </div>
                                                            ))}
                                                        </div>
                                                    ) : (
                                                        <p className="text-muted-foreground text-sm">No similar files found</p>
                                                    )}
                                                </div>

                                                {/* Error in result */}
                                                {result.result.error && (
                                                    <div className="p-3 bg-destructive/10 border border-destructive/20 rounded">
                                                        <p className="text-xs text-destructive">{result.result.error}</p>
                                                    </div>
                                                )}
                                            </div>
                                        ) : (
                                            <p className="text-muted-foreground">No result data</p>
                                        )}

                                        {/* Raw JSON */}
                                        <details className="mt-4">
                                            <summary className="text-xs text-muted-foreground cursor-pointer hover:underline">
                                                View Raw JSON
                                            </summary>
                                            <pre className="mt-2 text-xs bg-muted p-3 rounded overflow-x-auto">
                                                {JSON.stringify(result, null, 2)}
                                            </pre>
                                        </details>
                                    </div>
                                ) : (
                                    <Empty className="min-h-[400px]">
                                        <EmptyMedia variant="icon">
                                            <Code2 className="size-8" />
                                        </EmptyMedia>
                                        <EmptyTitle>No results yet</EmptyTitle>
                                        <EmptyDescription>
                                            Enter URL template, query value, and script, then click "Test Script" to see results here.
                                        </EmptyDescription>
                                    </Empty>
                                )}
                            </CardContent>
                        </Card>
                    </div>
                </div>
            </div>
        </TooltipProvider>
    );
}

export default CustomProviderPage;
