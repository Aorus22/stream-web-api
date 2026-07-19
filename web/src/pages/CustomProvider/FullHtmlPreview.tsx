import { useParams, useNavigate } from "react-router-dom";
import { ArrowLeft, Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

export function FullHtmlPreviewPage() {
    const { encodedHtml } = useParams<{ encodedHtml?: string }>();
    const navigate = useNavigate();

    // Decode the HTML from URL or sessionStorage
    let decodedHtml = "<html><body><p>No HTML content</p></body></html>";
    if (encodedHtml === "session") {
        const storedHtml = sessionStorage.getItem('pending_html_preview');
        if (storedHtml) {
            decodedHtml = storedHtml;
        } else {
            decodedHtml = "<html><body><p>Session-stored HTML expired or not found. Please try again.</p></body></html>";
        }
    } else if (encodedHtml) {
        try {
            decodedHtml = decodeURIComponent(escape(atob(encodedHtml)));
        } catch (e) {
            console.error("Decoding error:", e);
            try {
                decodedHtml = atob(encodedHtml);
            } catch (e2) {
                decodedHtml = `<html><body><p>Error decoding HTML: ${(e2 as Error).message}</p></body></html>`;
            }
        }
    }

    const handleDownload = () => {
        const blob = new Blob([decodedHtml], { type: "text/html" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = "preview.html";
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    };

    const handleOpenInNewTab = () => {
        const blob = new Blob([decodedHtml], { type: "text/html" });
        const url = URL.createObjectURL(blob);
        window.open(url, "_blank");
    };

    return (
        <div className="min-h-screen bg-background">
            {/* Header */}
            <div className="border-b bg-card">
                <div className="container mx-auto px-4 py-4 flex items-center justify-between">
                    <div className="flex items-center gap-4">
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => navigate(-1)}
                            className="gap-2"
                        >
                            <ArrowLeft className="size-4" />
                            Back
                        </Button>
                        <h1 className="text-xl font-bold">HTML Preview</h1>
                    </div>
                    <div className="flex gap-2">
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={handleOpenInNewTab}
                            className="gap-2"
                        >
                            Open in New Tab
                        </Button>
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={handleDownload}
                            className="gap-2"
                        >
                            <Download className="size-4" />
                            Download HTML
                        </Button>
                    </div>
                </div>
            </div>

            {/* Main Content - Split View */}
            <div className="container mx-auto px-4 py-6">
                <div className="grid grid-cols-2 gap-4 h-[calc(100vh-140px)]">
                    {/* Raw HTML - Left */}
                    <Card className="flex flex-col">
                        <div className="border-b px-4 py-2 bg-muted/50">
                            <h2 className="text-sm font-semibold">Source Code</h2>
                            <p className="text-xs text-muted-foreground">{decodedHtml.length} characters</p>
                        </div>
                        <CardContent className="flex-1 p-0 overflow-hidden">
                            <pre className="h-full p-4 text-xs bg-muted overflow-auto font-mono whitespace-pre-wrap break-all m-0">
                                {decodedHtml}
                            </pre>
                        </CardContent>
                    </Card>

                    {/* Rendered HTML - Right */}
                    <Card className="flex flex-col">
                        <div className="border-b px-4 py-2 bg-muted/50">
                            <h2 className="text-sm font-semibold">Rendered Preview</h2>
                        </div>
                        <CardContent className="flex-1 p-0 overflow-hidden">
                            <iframe
                                srcDoc={decodedHtml}
                                className="w-full h-full border-0 bg-background"
                                sandbox="allow-same-origin allow-scripts allow-forms allow-popups"
                                title="HTML Preview"
                            />
                        </CardContent>
                    </Card>
                </div>
            </div>
        </div>
    );
}

export default FullHtmlPreviewPage;
