import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Film, Loader2, CheckCircle, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useServer } from "@/contexts/ServerContext";

export default function ServerLogin() {
    const [url, setUrl] = useState("");
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState("");
    const navigate = useNavigate();
    const { setServerUrl } = useServer();

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!url) return;

        setLoading(true);
        setError("");

        let normalizedUrl = url.trim();
        if (!normalizedUrl.startsWith("http://") && !normalizedUrl.startsWith("https://")) {
            normalizedUrl = `http://${normalizedUrl}`;
        }
        normalizedUrl = normalizedUrl.replace(/\/$/, "");

        try {
            const response = await fetch(`${normalizedUrl}/api/torrents`, {
                method: "GET",
                signal: AbortSignal.timeout(5000)
            });

            if (response.ok) {
                setServerUrl(normalizedUrl);
                navigate("/");
            } else {
                setError("Server is not responding correctly. Please check the URL.");
            }
        } catch (err) {
            setError("Cannot connect to server. Check the URL and make sure the server is running.");
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background via-background to-muted/20 p-4">
            <Card className="w-full max-w-md">
                <CardHeader className="space-y-1 text-center">
                    <div className="flex justify-center mb-4">
                        <div className="p-4 bg-primary/10 rounded-2xl ring-1 ring-primary/20">
                            <Film className="w-12 h-12 text-primary" />
                        </div>
                    </div>
                    <CardTitle className="text-2xl font-bold">TorrentStream</CardTitle>
                    <CardDescription>
                        Enter server URL to get started
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <form onSubmit={handleSubmit} className="space-y-4">
                        <div className="space-y-2">
                            <Input
                                type="text"
                                placeholder="http://localhost:6432"
                                value={url}
                                onChange={(e) => setUrl(e.target.value)}
                                disabled={loading}
                                className="h-12"
                            />
                            <p className="text-xs text-muted-foreground">
                                Example: http://localhost:6432 or http://192.168.1.100:6432
                            </p>
                        </div>
                        {error && (
                            <div className="flex items-start gap-2 text-sm text-destructive bg-destructive/10 p-3 rounded-lg">
                                <XCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
                                {error}
                            </div>
                        )}
                        <Button
                            type="submit"
                            className="w-full h-12"
                            disabled={loading || !url}
                        >
                            {loading ? (
                                <>
                                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                    Checking connection...
                                </>
                            ) : (
                                <>
                                    <CheckCircle className="w-4 h-4 mr-2" />
                                    Connect to Server
                                </>
                            )}
                        </Button>
                    </form>
                </CardContent>
            </Card>
        </div>
    );
}
