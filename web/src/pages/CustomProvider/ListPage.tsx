import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Plus, Play, Trash2, Edit, Code2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useServer } from "@/contexts/ServerContext";
import { Empty, EmptyMedia, EmptyTitle, EmptyDescription } from "@/components/ui/empty";

interface CustomProvider {
    id: string;
    name: string;
    baseUrl: string;
    pageType: string;
    code: string;
    createdAt: string;
    updatedAt: string;
}

export function CustomProviderListPage() {
    const { serverUrl } = useServer();
    const navigate = useNavigate();
    const [providers, setProviders] = useState<CustomProvider[]>([]);
    const [loading, setLoading] = useState(true);

    const fetchProviders = async () => {
        if (!serverUrl) return;

        try {
            const response = await fetch(`${serverUrl}/api/custom-providers`);
            if (response.ok) {
                const data = await response.json();
                setProviders(data);
            }
        } catch (err) {
            console.error("Failed to fetch providers:", err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchProviders();
    }, [serverUrl]);

    const handleDelete = async (id: string) => {
        if (!serverUrl || !confirm("Are you sure you want to delete this provider?")) return;

        try {
            const response = await fetch(`${serverUrl}/api/custom-providers/${id}`, {
                method: "DELETE",
            });
            if (response.ok) {
                fetchProviders();
            }
        } catch (err) {
            console.error("Failed to delete provider:", err);
        }
    };

    const handleRun = async (provider: CustomProvider) => {
        // Navigate to a test/run page with the provider
        navigate(`/custom-provider/test/${provider.id}`);
    };

    return (
        <div className="space-y-8 p-4 md:p-8 max-w-7xl mx-auto">
            {/* Header */}
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6 border-b border-border/40 pb-6">
                <div className="flex items-center gap-4">
                    <div className="p-3 bg-primary/10 rounded-2xl ring-1 ring-primary/20 shadow-inner">
                        <Code2 className="size-8 text-primary" />
                    </div>
                    <div>
                        <h1 className="text-3xl md:text-4xl font-bold tracking-tight text-foreground">
                            Custom Providers
                        </h1>
                        <p className="text-muted-foreground mt-1">
                            Manage your custom JavaScript scrapers for torrent sources
                        </p>
                    </div>
                </div>
                <Button 
                    onClick={() => navigate("/custom-provider/new")} 
                    className="shadow-lg shadow-primary/20 hover:shadow-primary/40 transition-all"
                    size="lg"
                >
                    <Plus className="size-5 mr-2" />
                    Create Provider
                </Button>
            </div>

            {/* List */}
            {loading ? (
                <div className="flex flex-col items-center justify-center py-20 space-y-4">
                    <div className="animate-spin rounded-full h-12 w-12 border-4 border-primary border-t-transparent"></div>
                    <p className="text-muted-foreground animate-pulse">Loading providers...</p>
                </div>
            ) : providers.length === 0 ? (
                <Empty className="min-h-[400px] border border-dashed border-border/60 bg-muted/5 rounded-3xl">
                    <EmptyMedia variant="icon" className="bg-muted/20 p-6 rounded-full mb-4">
                        <Code2 className="size-10 text-muted-foreground" />
                    </EmptyMedia>
                    <EmptyTitle className="text-xl">No custom providers yet</EmptyTitle>
                    <EmptyDescription className="text-base max-w-md mx-auto mt-2">
                        Create your first custom provider to extend your streaming sources with any torrent site.
                    </EmptyDescription>
                    <Button 
                        onClick={() => navigate("/custom-provider/new")} 
                        variant="outline" 
                        className="mt-6 border-primary/30 hover:bg-primary/10 hover:text-primary"
                    >
                        <Plus className="size-4 mr-2" />
                        Create Now
                    </Button>
                </Empty>
            ) : (
                <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                    {providers.map((provider) => (
                        <Card 
                            key={provider.id} 
                            className="group hover:shadow-xl hover:shadow-primary/5 hover:-translate-y-1 transition-all duration-300 border-border/50 bg-card/50 backdrop-blur-sm overflow-hidden flex flex-col"
                        >
                            <CardContent className="p-0 flex flex-col h-full">
                                {/* Card Header */}
                                <div className="p-5 border-b border-border/50 bg-muted/20">
                                    <div className="flex justify-between items-start gap-3 mb-2">
                                        <div className="p-2 bg-background rounded-lg shadow-sm border border-border/50">
                                            <Code2 className="size-5 text-primary" />
                                        </div>
                                        <span className={`text-[10px] uppercase font-bold tracking-wider px-2 py-1 rounded-full border ${
                                            provider.pageType === 'list' 
                                                ? 'bg-primary/10 text-primary border-primary/20' 
                                                : 'bg-accent text-accent-foreground border-border'
                                        }`}>
                                            {provider.pageType}
                                        </span>
                                    </div>
                                    <h3 className="font-bold text-lg leading-tight line-clamp-1" title={provider.name}>
                                        {provider.name}
                                    </h3>
                                </div>

                                {/* Card Body */}
                                <div className="p-5 flex-1 space-y-4">
                                    <div className="space-y-1.5">
                                        <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Base URL</p>
                                        <div className="bg-muted/50 rounded-md p-2 border border-border/50">
                                            <code className="text-xs font-mono text-foreground/80 break-all line-clamp-2">
                                                {provider.baseUrl}
                                            </code>
                                        </div>
                                    </div>
                                    
                                    <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
                                        <div>
                                            <span className="block opacity-70">Updated</span>
                                            <span className="font-medium text-foreground">
                                                {new Date(provider.updatedAt).toLocaleDateString()}
                                            </span>
                                        </div>
                                        <div>
                                            <span className="block opacity-70">ID</span>
                                            <span className="font-mono opacity-50 truncate" title={provider.id}>
                                                #{provider.id.substring(0, 6)}
                                            </span>
                                        </div>
                                    </div>
                                </div>

                                {/* Card Footer */}
                                <div className="p-4 pt-0 mt-auto grid grid-cols-3 gap-2">
                                    <Button
                                        variant="secondary"
                                        size="sm"
                                        className="w-full text-xs font-medium hover:bg-primary/10 hover:text-primary transition-colors"
                                        onClick={() => navigate(`/custom-provider/edit/${provider.id}`)}
                                    >
                                        <Edit className="size-3.5 mr-1.5" />
                                        Edit
                                    </Button>
                                    <Button
                                        variant="outline"
                                        size="sm"
                                        className="w-full text-xs font-medium border-border/50 hover:bg-accent hover:text-accent-foreground transition-colors"
                                        onClick={() => handleRun(provider)}
                                    >
                                        <Play className="size-3.5 mr-1.5" />
                                        Run
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        className="w-full text-xs font-medium text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
                                        onClick={() => handleDelete(provider.id)}
                                    >
                                        <Trash2 className="size-3.5" />
                                    </Button>
                                </div>
                            </CardContent>
                        </Card>
                    ))}
                </div>
            )}
        </div>
    );
}

export default CustomProviderListPage;
