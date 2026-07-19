import { useState, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { Code2, Loader2, Save, ArrowLeft, Play, Eye } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useServer } from "@/contexts/ServerContext";
import Editor from "@monaco-editor/react";

// Import templates
import luaTemplate from "./templates/template.lua?raw";

// Types for test results
interface TestResult {
    result?: {
        type: string;
        results?: Array<{ url: string; [key: string]: unknown }>;
        [key: string]: unknown;
    } | null;
    error?: string;
    [key: string]: unknown;
}

export function CustomProviderEditorPage() {
    const { serverUrl } = useServer();
    const navigate = useNavigate();
    const { id } = useParams<{ id?: string }>();
    const isEdit = !!id;
    
    const [name, setName] = useState("");
    const [baseUrl, setBaseUrl] = useState("");
    const [pageType, setPageType] = useState<"list" | "detail">("list");
    const [code, setCode] = useState(luaTemplate);
    const [saving, setSaving] = useState(false);
    const [loading, setLoading] = useState(isEdit);

    // Test states
    const [query, setQuery] = useState("");
    const [testing, setTesting] = useState(false);
    const [testResult, setTestResult] = useState<TestResult | null>(null);
    const [showTestResult, setShowTestResult] = useState(false);
    const [displayMode, setDisplayMode] = useState<"interactive" | "json">("interactive");

    // Detail URL test states (for list type providers)
    const [detailUrl, setDetailUrl] = useState("");
    const [testingDetail, setTestingDetail] = useState(false);
    const [detailResult, setDetailResult] = useState<TestResult | null>(null);
    const [showDetailResult, setShowDetailResult] = useState(false);
    const [detailDisplayMode, setDetailDisplayMode] = useState<"interactive" | "json">("interactive");

    const [previewingHtml, setPreviewingHtml] = useState(false);
    const [previewingDetailHtml, setPreviewingDetailHtml] = useState(false);

    const openFullHtmlPreview = (html: string) => {
        if (!html) return;
        try {
            // Store in sessionStorage to avoid URL length limits (Error 431)
            sessionStorage.setItem('pending_html_preview', html);
            window.open('/custom-provider/preview/session', '_blank');
        } catch (err) {
            console.error("Failed to open full preview:", err);
            // Fallback for smaller HTML if storage fails
            try {
                const encoded = btoa(unescape(encodeURIComponent(html)));
                window.open(`/custom-provider/preview/${encoded}`, '_blank');
            } catch (e) {
                alert("Failed to preview HTML: Content is too large");
            }
        }
    };

// Load provider data if editing
    useEffect(() => {
        if (isEdit && serverUrl) {
            fetch(`${serverUrl}/api/custom-providers/${id}`)
                .then(res => res.json())
                .then(data => {
                    setName(data.name || "");
                    setBaseUrl(data.baseUrl || "");
                    setPageType(data.pageType || "list");
                    // Decode base64 code
                    if (data.code) {
                        try {
                            const decoded = decodeURIComponent(escape(atob(data.code)));
                            setCode(decoded);
                        } catch {
                            setCode(data.code);
                        }
                    } else {
                        // Set default script
                        setCode(luaTemplate);
                    }
                })
                .catch(err => console.error("Failed to load provider:", err))
                .finally(() => setLoading(false));
        }
    }, [isEdit, id, serverUrl]);

    const handleTest = async () => {
        if (!serverUrl || !baseUrl || !code) return;

        setTesting(true);
        setTestResult(null);
        setShowTestResult(true);
        try {
            const base64Code = btoa(unescape(encodeURIComponent(code)));
            const fullUrl = query ? baseUrl.replace("{q}", encodeURIComponent(query)) : baseUrl;

            const response = await fetch(`${serverUrl}/api/js/execute`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    code: base64Code,
                    url: fullUrl,
                    pageType: pageType,
                    isBase64: true,
                    language: "lua",
                }),
            });

            const data = await response.json();
            setTestResult(data);
        } catch (err) {
            console.error("Test failed:", err);
            setTestResult({ error: "Failed to execute script" });
        } finally {
            setTesting(false);
        }
    };

    const handleTestDetail = async () => {
        if (!serverUrl || !detailUrl || !code) return;

        setTestingDetail(true);
        setDetailResult(null);
        setShowDetailResult(true);
        try {
            const base64Code = btoa(unescape(encodeURIComponent(code)));

            const response = await fetch(`${serverUrl}/api/js/execute`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    code: base64Code,
                    url: detailUrl,
                    pageType: "detail", // Always use "detail" for child URL testing
                    isBase64: true,
                    language: "lua",
                }),
            });

            const data = await response.json();
            setDetailResult(data);
        } catch (err) {
            console.error("Detail test failed:", err);
            setDetailResult({ error: "Failed to execute script" });
        } finally {
            setTestingDetail(false);
        }
    };

    const handlePreviewHtml = async () => {
        if (!serverUrl || !baseUrl) return;

        setPreviewingHtml(true);
        try {
            const fullUrl = query ? baseUrl.replace("{q}", encodeURIComponent(query)) : baseUrl;

            const response = await fetch(`${serverUrl}/api/js/preview`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ url: fullUrl }),
            });

            if (response.ok) {
                const html = await response.text();
                openFullHtmlPreview(html);
            } else {
                alert(`Error fetching HTML: ${response.statusText}`);
            }
        } catch (err) {
            console.error("Preview HTML failed:", err);
            alert("Failed to fetch HTML: " + (err as Error).message);
        } finally {
            setPreviewingHtml(false);
        }
    };

    const handlePreviewDetailHtml = async () => {
        if (!serverUrl || !detailUrl) return;

        setPreviewingDetailHtml(true);
        try {
            const response = await fetch(`${serverUrl}/api/js/preview`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ url: detailUrl }),
            });

            if (response.ok) {
                const html = await response.text();
                openFullHtmlPreview(html);
            } else {
                alert(`Error fetching detail HTML: ${response.statusText}`);
            }
        } catch (err) {
            console.error("Preview detail HTML failed:", err);
            alert("Failed to fetch HTML: " + (err as Error).message);
        } finally {
            setPreviewingDetailHtml(false);
        }
    };

    const handleSave = async () => {
        if (!serverUrl || !name || !baseUrl || !code) return;

        setSaving(true);
        try {
            const base64Code = btoa(unescape(encodeURIComponent(code)));
            
            const payload = {
                name,
                baseUrl,
                pageType,
                code: base64Code,
                language: "lua",
            };

            if (isEdit) {
                await fetch(`${serverUrl}/api/custom-providers/${id}`, {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify(payload),
                });
            } else {
                await fetch(`${serverUrl}/api/custom-providers`, {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify(payload),
                });
            }

            navigate("/custom-provider");
        } catch (err) {
            console.error("Failed to save provider:", err);
        } finally {
            setSaving(false);
        }
    };

    if (loading) {
        return (
            <div className="flex justify-center items-center min-h-[400px]">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
        );
    }

        return (
    
            <div className="space-y-6 p-4 md:p-8 max-w-[1600px] mx-auto pb-20">
    
                {/* Header */}
    
                <div className="flex items-center gap-4 border-b border-border/40 pb-6">
    
                    <Button
    
                        variant="outline"
    
                        size="icon"
    
                        onClick={() => navigate(-1)}
    
                        className="h-10 w-10 rounded-full border-border/60 hover:bg-muted/50 transition-colors"
    
                    >
    
                        <ArrowLeft className="size-5" />
    
                    </Button>
    
                    <div className="flex items-center gap-4">
    
                        <div className="p-2.5 bg-primary/10 rounded-xl ring-1 ring-primary/20 shadow-inner hidden md:block">
    
                            <Code2 className="size-6 text-primary" />
    
                        </div>
    
                        <div>
    
                            <h1 className="text-2xl md:text-3xl font-bold tracking-tight text-foreground">
    
                                {isEdit ? "Edit Provider" : "New Provider"}
    
                            </h1>
    
                            <p className="text-muted-foreground text-sm">
    
                                {isEdit ? "Modify your custom Lua scraper logic" : "Create a new custom Lua scraper"}
    
                            </p>
    
                        </div>
    
                    </div>
    
                    <div className="ml-auto flex items-center gap-3">
    
                        <Button
    
                            onClick={handleSave}
    
                            disabled={!name || !baseUrl || !code || saving || !serverUrl}
    
                            className="shadow-lg shadow-primary/20 hover:shadow-primary/40 transition-all gap-2 min-w-[120px]"
                        >
    
                            {saving ? (
    
                                <>
    
                                    <Loader2 className="size-4 animate-spin" />
    
                                    Saving...
    
                                </>
    
                            ) : (
    
                                <>
    
                                    <Save className="size-4" />
    
                                    Save
    
                                </>
    
                            )}
    
                        </Button>
    
                    </div>
    
                </div>
    
    
    
                <div className="grid md:grid-cols-3 gap-6 lg:gap-8 h-full">
    
                    {/* Left Column: Configuration & Code */}
    
                    <div className="md:col-span-2 space-y-6">
    
                        <Card className="border-border/50 shadow-sm bg-card/50 backdrop-blur-sm">
    
                            <CardContent className="pt-6 space-y-6">
    
                                <div className="flex items-center gap-2 pb-2 border-b border-border/40">
    
                                    <Code2 className="size-5 text-primary" />
    
                                    <h3 className="font-semibold text-lg">Configuration</h3>
    
                                </div>
    
    
    
                                <div className="grid md:grid-cols-2 gap-6">
    
                                    <div className="space-y-2.5">
    
                                        <Label htmlFor="name" className="text-sm font-medium">Provider Name</Label>
    
                                        <Input
    
                                            id="name"
    
                                            placeholder="e.g., My Torrent Site"
    
                                            value={name}
    
                                            onChange={(e) => setName(e.target.value)}
    
                                            className="bg-background/50 focus:bg-background transition-colors"
    
                                        />
    
                                    </div>
    
    
    
                                    <div className="space-y-2.5">
    
                                        <Label htmlFor="baseUrl" className="text-sm font-medium">Base URL Template</Label>
    
                                        <div className="relative">
    
                                            <Input
    
                                                id="baseUrl"
    
                                                placeholder="https://example.com/search?q={q}"
    
                                                value={baseUrl}
    
                                                onChange={(e) => setBaseUrl(e.target.value)}
    
                                                className="font-mono text-sm pr-20 bg-background/50 focus:bg-background transition-colors"
    
                                            />
    
                                            <div className="absolute right-2 top-1/2 -translate-y-1/2">
    
                                                <code className="bg-muted px-1.5 py-0.5 rounded text-[10px] text-muted-foreground border border-border/50">
    
                                                    {'{q}'}
    
                                                </code>
    
                                            </div>
    
                                        </div>
    
                                    </div>
    
                                </div>
    
    
    
                                <div className="space-y-3">
    
                                    <Label className="text-sm font-medium">Page Type</Label>
    
                                    <div className="flex gap-4 p-1 bg-muted/30 rounded-lg w-fit">
    
                                        <label className={`flex items-center gap-2 px-4 py-2 rounded-md cursor-pointer transition-all ${pageType === "list" ? "bg-background shadow-sm text-foreground ring-1 ring-border" : "text-muted-foreground hover:text-foreground"}`}>
    
                                            <input
    
                                                type="radio"
    
                                                name="pageType"
    
                                                checked={pageType === "list"}
    
                                                onChange={() => setPageType("list")}
    
                                                className="sr-only"
    
                                            />
    
                                            <span className="text-sm font-medium">List (Search Results)</span>
    
                                        </label>
    
                                        <label className={`flex items-center gap-2 px-4 py-2 rounded-md cursor-pointer transition-all ${pageType === "detail" ? "bg-background shadow-sm text-foreground ring-1 ring-border" : "text-muted-foreground hover:text-foreground"}`}>
    
                                            <input
    
                                                type="radio"
    
                                                name="pageType"
    
                                                checked={pageType === "detail"}
    
                                                onChange={() => setPageType("detail")}
    
                                                className="sr-only"
    
                                            />
    
                                            <span className="text-sm font-medium">Detail (Magnet Link)</span>
    
                                        </label>
    
                                    </div>
    
                                </div>
    
                            </CardContent>
    
                        </Card>
    
    
    
                        <Card className="border-border/50 shadow-sm bg-card/50 backdrop-blur-sm flex flex-col min-h-[500px]">
    
                            <CardContent className="pt-4 p-0 flex flex-col flex-1">
    
                                <div className="px-6 py-3 flex items-center justify-between border-b border-border/40 bg-muted/20">
    
                                    <Label className="text-sm font-medium flex items-center gap-2">
    
                                        <div className="w-2 h-2 rounded-full bg-blue-500"></div>
    
                                        Lua Logic
    
                                    </Label>
    
                                    <Button
    
                                        variant="ghost"
    
                                        size="sm"
    
                                        onClick={() => setCode(luaTemplate)}
    
                                        className="h-7 text-xs hover:bg-destructive/10 hover:text-destructive transition-colors"
    
                                    >
    
                                        Reset to Default
    
                                    </Button>
    
                                </div>
    
                                                            <div className="relative border-b border-border/40">
    
                                                                <Editor
    
                                                                    height="600px"
    
                                                                    defaultLanguage="lua"
                                                                    language="lua"
    
                                                                    value={code}
    
                                                                    onChange={(value) => setCode(value || "")}
    
                                                                    theme="vs-dark"
    
                                                                    options={{
    
                                                                        minimap: { enabled: false },
    
                                                                        fontSize: 14,
    
                                                                        lineNumbers: "on",
    
                                                                        scrollBeyondLastLine: false,
    
                                                                        automaticLayout: true,
    
                                                                        tabSize: 2,
    
                                                                        padding: { top: 16, bottom: 16 },
    
                                                                        fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
    
                                                                    }}
    
                                                                />
    
                                                            </div>
    
                                                                <div className="px-4 py-2 bg-muted/30 border-t border-border/40 text-[11px] text-muted-foreground flex gap-3 flex-wrap">
                                    
                                                                    <span>Available Globals:</span>
                                    
                                                                    <code className="bg-muted px-1 rounded border border-border/50">ARG_FULL_URL</code>
                                    
                                                                    <code className="bg-muted px-1 rounded border border-border/50">ARG_PAGE_TYPE</code>
                                    
                                                                    <code className="bg-muted px-1 rounded border border-border/50">fetch(url)</code>

                                                                    <code className="bg-muted px-1 rounded border border-border/50">html_parse(html)</code>
                                    
                                                                </div>
                                    
                                                            </CardContent>
                                    
                                                        </Card>
                                
                                                        {/* Expected Schema Help (Moved to Left) */}
                                                        <div className="grid md:grid-cols-2 gap-4">
                                                            <Card className="border-border/50 shadow-sm bg-primary/5 border-l-4 border-l-emerald-500/50">
                                                                <CardContent className="pt-4 space-y-3">
                                                                    <div className="flex items-center gap-2 mb-1">
                                                                        <Code2 className="size-4 text-emerald-500" />
                                                                        <h3 className="font-semibold text-xs uppercase tracking-wider">Type: 'list' (Search)</h3>
                                                                    </div>
                                                                    <pre className="p-2 bg-black/80 rounded text-[9px] font-mono text-emerald-400 overflow-x-auto leading-relaxed">
{`{
  "type": "list",
  "results": [
    {
      "name": "File Name",
      "url": "https://site.com/1",
      "seeds": 120, "leeches": 15,
      "size": "2.4 GB"
    }
  ]
}`}
                                                                    </pre>
                                                                </CardContent>
                                                            </Card>
                                
                                                            <Card className="border-border/50 shadow-sm bg-primary/5 border-l-4 border-l-blue-500/50">
                                                                <CardContent className="pt-4 space-y-3">
                                                                    <div className="flex items-center gap-2 mb-1">
                                                                        <Code2 className="size-4 text-blue-500" />
                                                                        <h3 className="font-semibold text-xs uppercase tracking-wider">Type: 'detail' (Magnet)</h3>
                                                                    </div>
                                                                    <pre className="p-2 bg-black/80 rounded text-[9px] font-mono text-blue-400 overflow-x-auto leading-relaxed">
{`{
  "type": "detail",
  "name": "File Name",
  "magnetLink": "magnet:?xt=...",
  "directDownloads": [
    { "url": "https://...", "text": "Mirror 1" }
  ]
}`}
                                                                    </pre>
                                                                </CardContent>
                                                            </Card>
                                                        </div>
                                                    </div>
                                
    
    
    
                    {/* Right Column: Testing & Help */}
    
                    <div className="space-y-6">
    
                        {/* Testing Zone */}
    
                        <Card className="border-border/50 shadow-md bg-card border-t-4 border-t-primary">
    
                            <CardContent className="pt-6 space-y-4">
    
                                <div className="flex items-center justify-between pb-2 border-b border-border/40">
    
                                    <div className="flex items-center gap-2">
    
                                        <Play className="size-4 text-primary" />
    
                                        <h3 className="font-semibold text-sm uppercase tracking-wider">Debugger</h3>
    
                                    </div>
    
                                    <div className="flex gap-1">
    
                                         <Button
    
                                            onClick={handleTest}
    
                                            disabled={!baseUrl || !code || testing || !serverUrl}
    
                                            size="sm"
    
                                            className="h-7 text-xs"
                                        >
    
                                            {testing ? <Loader2 className="size-3 animate-spin" /> : <Play className="size-3 mr-1" />}
    
                                            Run
    
                                        </Button>
    
                                    </div>
    
                                </div>
    
    
    
                                {/* Inputs */}
    
                                <div className="space-y-4">
    
                                    <div className="space-y-1.5">
    
                                        <Label htmlFor="query" className="text-xs font-medium text-muted-foreground">Search Query</Label>
    
                                        <div className="flex gap-2">
    
                                            <Input
    
                                                id="query"
    
                                                placeholder="e.g., Inception"
    
                                                value={query}
    
                                                onChange={(e) => setQuery(e.target.value)}
    
                                                onKeyDown={(e) => e.key === "Enter" && handleTest()}
    
                                                className="h-8 text-sm"
    
                                            />
    
                                        </div>
    
                                    </div>
    
                                    
    
                                    <div className="space-y-1.5">
    
                                        <Label className="text-xs font-medium text-muted-foreground">Generated URL</Label>
    
                                        <div className="p-2 bg-muted/40 rounded border border-border/50 text-[10px] font-mono break-all text-foreground/80 min-h-[2.5rem] flex items-center">
    
                                            {query
    
                                                ? baseUrl.replace("{q}", encodeURIComponent(query))
    
                                                : baseUrl || <span className="text-muted-foreground italic">(enter base URL)</span>}
    
                                        </div>
    
                                    </div>
    
    
    
                                    <div className="grid grid-cols-2 gap-2">
    
                                        <Button
    
                                            onClick={handlePreviewHtml}
    
                                            disabled={!baseUrl || previewingHtml || !serverUrl}
    
                                            variant="secondary"
    
                                            size="sm"
    
                                            className="text-xs h-8"
    
                                        >
    
                                            {previewingHtml ? <Loader2 className="size-3 animate-spin mr-1" /> : <Eye className="size-3 mr-1" />}
    
                                            Preview HTML
    
                                        </Button>
    
                                    </div>
    
                                </div>
    
    
    
                                                                {/* Results Area */}
                                    
                                                                <div className="space-y-2 pt-2 border-t border-border/40">
                                    
                                                                    <div className="flex items-center justify-between">
                                                                        <Label className="text-xs font-medium text-muted-foreground">Console Output</Label>
                                                                        {(showTestResult && testResult && !testResult.error) && (
                                                                            <div className="flex gap-1 p-0.5 bg-muted/40 rounded border border-border/50">
                                                                                <Button 
                                                                                    size="sm" 
                                                                                    variant={displayMode === "interactive" ? "secondary" : "ghost"} 
                                                                                    onClick={() => setDisplayMode("interactive")}
                                                                                    className="h-5 px-1.5 text-[9px]"
                                                                                >
                                                                                    Interactive
                                                                                </Button>
                                                                                <Button 
                                                                                    size="sm" 
                                                                                    variant={displayMode === "json" ? "secondary" : "ghost"} 
                                                                                    onClick={() => setDisplayMode("json")}
                                                                                    className="h-5 px-1.5 text-[9px]"
                                                                                >
                                                                                    JSON
                                                                                </Button>
                                                                            </div>
                                                                        )}
                                                                    </div>
                                    
                                                                    
                                    
                                                                                                        {showTestResult && testResult ? (
                                                                                                            <div className="rounded-md border border-border/50 bg-black/90 p-3 max-h-[400px] overflow-auto custom-scrollbar">
                                                                                                                {testResult.error ? (
                                                                                                                    <div className="text-red-400 text-xs font-mono">
                                                                                                                        <span className="font-bold">Error:</span> {testResult.error}
                                                                                                                    </div>
                                                                                                                ) : (
                                                                                                                    <div className="space-y-4">
                                                                                                                        {displayMode === "interactive" && testResult.result?.type === 'list' && Array.isArray(testResult.result.results) ? (
                                                                                                                            <div className="space-y-1">
                                                                                                                                <div className="text-[10px] text-emerald-500 mb-2 border-b border-emerald-500/20 pb-1">Click a result to test its Detail Page:</div>
                                                                                                                                                                                            {testResult.result.results.map((res: any, idx: number) => (
                                                                                                                                                                                                <div 
                                                                                                                                                                                                    key={idx}
                                                                                                                                                                                                    onClick={() => {
                                                                                                                                                                                                        const result = res as any;
                                                                                                                                                                                                        setDetailUrl(result.url);
                                                                                                                                                                                                        setShowDetailResult(false);
                                                                                                                                                                                                        setTimeout(() => document.getElementById('detail-test-area')?.scrollIntoView({ behavior: 'smooth' }), 100);
                                                                                                                                                                                                    }}
                                                                                                                                
                                                                                                                                        className="p-2 rounded hover:bg-emerald-500/10 cursor-pointer border border-transparent hover:border-emerald-500/30 transition-all group"
                                                                                                                                    >
                                                                                                                                        <div className="text-[11px] font-bold text-emerald-400 group-hover:text-emerald-300 truncate">{res.name}</div>
                                                                                                                                        <div className="text-[9px] text-emerald-500/60 font-mono truncate">{res.url}</div>
                                                                                                                                        <div className="flex gap-3 mt-1">
                                                                                                                                            {res.seeds !== undefined && <span className="text-[9px] text-emerald-400/80">S: {res.seeds}</span>}
                                                                                                                                            {res.leeches !== undefined && <span className="text-[9px] text-red-400/80">L: {res.leeches}</span>}
                                                                                                                                            {res.size && <span className="text-[9px] text-blue-400/80">{res.size}</span>}
                                                                                                                                        </div>
                                                                                                                                    </div>
                                                                                                                                ))}
                                                                                                                            </div>
                                                                                                                        ) : (
                                                                                                                            <pre className="text-[10px] font-mono text-emerald-400 whitespace-pre-wrap">
                                                                                                                                {JSON.stringify(testResult.result, null, 2)}
                                                                                                                            </pre>
                                                                                                                        )}
                                                                                                                    </div>
                                                                                                                )}
                                                                                                            </div>
                                                                                                        ) : (
                                                                        
                                                                                                            <div className="rounded-md border border-border/50 bg-muted/20 p-8 flex flex-col items-center justify-center text-muted-foreground/50">
                                                                        
                                                                                                                <Play className="size-8 mb-2 opacity-20" />
                                                                        
                                                                                                                <span className="text-xs">Run a test to see results</span>
                                                                        
                                                                                                            </div>
                                                                        
                                                                                                        )}
                                                                                                                                    </div>
                                    
                                    
                                    
                                                                {/* Detail URL Test (restored) */}
                                    
                                                                {pageType === "list" && (
                                    
                                                                    <div id="detail-test-area" className="space-y-4 pt-4 border-t border-border/40">
                                    
                                                                        <div className="flex items-center justify-between">
                                                                            <div className="flex items-center gap-2">
                                                                                <div className="w-1.5 h-1.5 rounded-full bg-secondary"></div>
                                                                                <Label className="text-xs font-semibold text-secondary-foreground uppercase tracking-wider">Detail Page Test</Label>
                                                                            </div>
                                                                            {(showDetailResult && detailResult && !detailResult.error) && (
                                                                                <div className="flex gap-1 p-0.5 bg-muted/40 rounded border border-border/50">
                                                                                    <Button 
                                                                                        size="sm" 
                                                                                        variant={detailDisplayMode === "interactive" ? "secondary" : "ghost"} 
                                                                                        onClick={() => setDetailDisplayMode("interactive")}
                                                                                        className="h-5 px-1.5 text-[9px]"
                                                                                    >
                                                                                        Interactive
                                                                                    </Button>
                                                                                    <Button 
                                                                                        size="sm" 
                                                                                        variant={detailDisplayMode === "json" ? "secondary" : "ghost"} 
                                                                                        onClick={() => setDetailDisplayMode("json")}
                                                                                        className="h-5 px-1.5 text-[9px]"
                                                                                    >
                                                                                        JSON
                                                                                    </Button>
                                                                                </div>
                                                                            )}
                                                                        </div>
                                                                        
                                    
                                                                        <div className="space-y-2">
                                    
                                                                            <Input
                                    
                                                                                placeholder="Paste detail URL..."
                                    
                                                                                value={detailUrl}
                                    
                                                                                onChange={(e) => setDetailUrl(e.target.value)}
                                    
                                                                                onKeyDown={(e) => e.key === "Enter" && handleTestDetail()}
                                    
                                                                                className="h-8 text-sm bg-muted/20"
                                    
                                                                            />
                                    
                                                                            <div className="grid grid-cols-2 gap-2">
                                    
                                                                                <Button
                                    
                                                                                    onClick={handlePreviewDetailHtml}
                                    
                                                                                    disabled={!detailUrl || previewingDetailHtml || !serverUrl}
                                    
                                                                                    variant="secondary"
                                    
                                                                                    size="sm"
                                    
                                                                                    className="text-xs h-7"
                                    
                                                                                >
                                    
                                                                                    {previewingDetailHtml ? <Loader2 className="size-3 animate-spin mr-1" /> : <Eye className="size-3 mr-1" />}
                                                                                    HTML
                                    
                                                                                </Button>
                                    
                                                                                <Button
                                    
                                                                                    onClick={handleTestDetail}
                                    
                                                                                    disabled={!detailUrl || !code || testingDetail || !serverUrl}
                                    
                                                                                    size="sm"
                                                                                    variant="secondary"
                                                                                    className="text-xs h-7"
                                    
                                                                                >
                                    
                                                                                    {testingDetail ? <Loader2 className="size-3 animate-spin mr-1" /> : <Play className="size-3 mr-1" />}
                                                                                    Test Detail
                                    
                                                                                </Button>
                                    
                                                                            </div>
                                    
                                                                        </div>
                                    
                                    
                                    
                                                                                                                {/* Detail Results */}
                                                                            
                                                                                                                {(showDetailResult && detailResult) ? (
                                                                            
                                                                                                                    <div className="rounded-md border border-border/50 bg-black/90 p-3 max-h-[400px] overflow-auto custom-scrollbar relative">
                                                                            
                                                                                                                        <div className="flex justify-between items-center mb-2 sticky top-0 bg-black/80 backdrop-blur-sm p-1 rounded z-10">
                                                                                                                            <div className="text-[9px] text-muted-foreground font-mono">DETAIL OUTPUT</div>
                                                                                                                            <Button 
                                                                                                                                size="sm" 
                                                                                                                                variant="ghost" 
                                                                                                                                onClick={() => { setShowDetailResult(false); }} 
                                                                                                                                className="h-5 w-5 p-0 text-muted-foreground hover:text-white"
                                                                                                                            >
                                                                                                                                ×
                                                                                                                            </Button>
                                                                                                                        </div>
                                                                            
                                                                                                                                                                        {detailResult?.error ? (
                                                                                                                            
                                                                                                                                                                            <div className="text-red-400 text-xs font-mono">
                                                                                                                            
                                                                                                                                                                                <span className="font-bold">Error:</span> {detailResult.error}
                                                                                                                            
                                                                                                                                                                            </div>
                                                                                                                            
                                                                                                                                                                        ) : (
                                                                                                                                                                            <div className="space-y-4">
                                                                                                                                                                                {detailDisplayMode === "interactive" && (detailResult?.result as any)?.magnetLink ? (
                                                                                                                                                                                    <div className="space-y-3 p-1">
                                                                                                                                                                                        <div className="space-y-1">
                                                                                                                                                                                            <div className="text-[10px] text-emerald-500/70 font-bold uppercase tracking-wider">Magnet Link Found:</div>
                                                                                                                                                                                            <div className="p-2 bg-emerald-500/10 border border-emerald-500/20 rounded text-[10px] font-mono text-emerald-400 break-all select-all">
                                                                                                                                                                                                {(detailResult?.result as any).magnetLink}
                                                                                                                                                                                            </div>
                                                                                                                                                                                        </div>
                                                                                                                                                                                        
                                                                                                                                                                                        {Array.isArray((detailResult?.result as any).directDownloads) && (detailResult?.result as any).directDownloads.length > 0 && (
                                                                                                                                                                                            <div className="space-y-1">
                                                                                                                                                                                                <div className="text-[10px] text-blue-500/70 font-bold uppercase tracking-wider">Direct Downloads:</div>
                                                                                                                                                                                                <div className="space-y-1">
                                                                                                                                                                                                    {(detailResult?.result as any).directDownloads.map((dl: any, idx: number) => (
                                                                                                                                                                                                        <div key={idx} className="p-1.5 bg-blue-500/10 border border-blue-500/20 rounded flex justify-between items-center gap-2">
                                                                                                                                                                                                            <span className="text-[10px] text-blue-400 truncate">{dl.text || 'Download'}</span>
                                                                                                                                                                                                            <span className="text-[9px] text-blue-500/60 font-mono truncate max-w-[150px]">{dl.url}</span>
                                                                                                                                                                                                        </div>
                                                                                                                                                                                                    ))}
                                                                                                                                                                                                </div>
                                                                                                                                                                                            </div>
                                                                                                                                                                                        )}
                                                                                                                                                                                    </div>
                                                                                                                                                                                ) : (
                                                                                                                                                                                    <pre className="text-[10px] font-mono text-emerald-400 whitespace-pre-wrap">
                                                                                                                                                                                        {JSON.stringify(detailResult?.result, null, 2)}
                                                                                                                                                                                    </pre>
                                                                                                                                                                                )}
                                                                                                                                                                            </div>
                                                                                                                                                                        )}
                                                                                                                        
                                                                                                                    </div>
                                                                                                                ) : null}
                                                                                                            
                                                                    </div>
                                    
                                                                )}
                                
    
                            </CardContent>
    
                        </Card>
    
    
    
                                                {/* Quick Help */}
                            
                                                <Card className="border-border/50 shadow-sm bg-muted/10">
                            
                                                    <CardContent className="pt-6">
                            
                                                         <div className="flex items-center gap-2 mb-4">
                            
                                                            <Code2 className="size-4 text-primary" />
                            
                                                            <h3 className="font-semibold text-sm">Quick Reference</h3>
                            
                                                        </div>
                            
                                                        <div className="space-y-3 text-xs">
                            
                                                            <div className="p-2.5 bg-background rounded border border-border/50 shadow-sm">
                            
                                                                <code className="text-primary font-bold block mb-1">ARG_FULL_URL</code>
                            
                                                                <p className="text-muted-foreground leading-relaxed">Contains the fully constructed URL with query parameters.</p>
                            
                                                            </div>
                            
                                                            <div className="p-2.5 bg-background rounded border border-border/50 shadow-sm">
                            
                                                                <code className="text-primary font-bold block mb-1">return {'{...}'}</code>
                            
                                                                <p className="text-muted-foreground leading-relaxed">
                            
                                                                    Must return an object with <code className="bg-muted px-1 rounded">type: 'list'</code> or <code className="bg-muted px-1 rounded">type: 'detail'</code>.
                            
                                                                </p>
                            
                                                            </div>
                            
                                                        </div>
                            
                                                    </CardContent>
                            
                                                </Card>
                                            </div>
                                        </div>
                                    </div>
                        
    
        );
    
}

export default CustomProviderEditorPage;
