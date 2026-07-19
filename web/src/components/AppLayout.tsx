import { Link, useLocation } from "react-router-dom";
import { useEffect } from "react";
import {
    Compass,
    Search,
    Library as LibraryIcon,
    Code2,
    LogOut,
    Menu,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { ThemeSwitcher } from "@/components/theme-switcher";
import { useServer } from "@/contexts/ServerContext";
import { useNavigate } from "react-router-dom";
import { useIsMobile } from "@/hooks/use-mobile";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

type NavItem = {
    icon: React.ElementType;
    label: string;
    href: string;
};

const navItems: NavItem[] = [
    { icon: Compass, label: "Discover", href: "/" },
    { icon: Search, label: "Torrent Search", href: "/search" },
    { icon: LibraryIcon, label: "Library", href: "/library" },
    { icon: Code2, label: "Custom Provider", href: "/custom-provider" },
];

// Logo SVG Component
function AppLogo({ className }: { className?: string }) {
    return (
        <div className={cn("relative", className)}>
            <svg viewBox="0 0 512 512" className="w-full h-full">
                <defs>
                    <linearGradient id="logoGradient" x1="0%" y1="0%" x2="100%" y2="100%">
                        <stop offset="0%" stopColor="oklch(0.60 0.12 190)" />
                        <stop offset="100%" stopColor="oklch(0.40 0.08 200)" />
                    </linearGradient>
                    <linearGradient id="playGradient" x1="0%" y1="0%" x2="100%" y2="100%">
                        <stop offset="0%" stopColor="#FFFFFF" stopOpacity="0.95" />
                        <stop offset="100%" stopColor="#E0F2F1" stopOpacity="0.95" />
                    </linearGradient>
                </defs>
                <rect width="512" height="512" rx="120" fill="url(#logoGradient)" />
                <path d="M200 140 L200 372 L360 256 Z" fill="url(#playGradient)" />
                <g fill="none" stroke="#FFFFFF" strokeWidth="6" strokeLinecap="round" opacity="0.3">
                    <path d="M140 380 Q180 360 200 380" />
                    <path d="M130 400 Q170 370 210 400" />
                </g>
            </svg>
        </div>
    );
}

export function Sidebar() {
    const location = useLocation();
    const { serverUrl, setServerUrl } = useServer();
    const navigate = useNavigate();
    const isMobile = useIsMobile();

    const handleDisconnect = () => {
        setServerUrl(null);
        navigate("/login");
    };

    return (
        <TooltipProvider delayDuration={0}>
            {isMobile ? (
                <aside className="fixed bottom-0 left-0 right-0 z-50 h-16 flex items-center justify-center bg-background/95 backdrop-blur-xl border-t border-border px-4">
                    <nav className="flex items-center gap-1 w-full max-w-md">
                        {navItems.map((item) => {
                            const isActive = location.pathname === item.href ||
                                (item.href !== "/" && (location.pathname === item.href || location.pathname.startsWith(item.href + "/")));

                            return (
                                <Link
                                    key={item.href}
                                    to={item.href}
                                    className={cn(
                                        "flex-1 flex items-center justify-center py-2 rounded-xl transition-all duration-200",
                                        isActive
                                            ? "bg-primary text-primary-foreground shadow-lg shadow-primary/25"
                                            : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
                                    )}
                                >
                                    <item.icon className="size-5" />
                                </Link>
                            );
                        })}
                        <Popover>
                            <PopoverTrigger asChild>
                                <Button
                                    variant="ghost"
                                    size="icon"
                                    className="flex-1 text-muted-foreground hover:text-foreground rounded-xl transition-all duration-200"
                                >
                                    <Menu className="size-5" />
                                </Button>
                            </PopoverTrigger>
                            <PopoverContent align="end" className="w-56 p-2">
                                <div className="flex flex-col gap-1">
                                    {serverUrl && (
                                        <div className="flex items-center gap-2 px-2 py-1.5 mb-1 bg-muted/40 rounded-md border border-border/50">
                                            <div className="size-1.5 rounded-full bg-green-500 animate-pulse shrink-0 shadow-[0_0_8px_rgba(34,197,94,0.5)]" />
                                            <span className="text-xs font-medium text-muted-foreground truncate flex-1 font-mono">
                                                {serverUrl.replace(/^https?:\/\//, '').replace(/\/$/, '')}
                                            </span>
                                        </div>
                                    )}
                                    <Button
                                        variant="ghost"
                                        className="w-full justify-start gap-2 h-9 px-2 text-destructive hover:text-destructive hover:bg-destructive/10 font-medium"
                                        onClick={handleDisconnect}
                                    >
                                        <LogOut className="size-4" />
                                        Disconnect
                                    </Button>
                                    <div className="flex items-center justify-between px-2 py-1.5 h-9">
                                        <span className="text-sm font-medium">Theme</span>
                                        <ThemeSwitcher />
                                    </div>
                                </div>
                            </PopoverContent>
                        </Popover>
                    </nav>
                </aside>
            ) : (
                <aside className="fixed left-0 top-0 z-40 h-screen w-16 flex flex-col bg-background/80 backdrop-blur-xl border-r border-border">
                    <div className="flex h-16 items-center justify-center border-b border-border">
                        <Link to="/" className="flex items-center justify-center p-2">
                            <div className="w-10 h-10 rounded-xl overflow-hidden shadow-lg shadow-primary/20">
                                <AppLogo />
                            </div>
                        </Link>
                    </div>

                    <nav className="flex-1 flex flex-col items-center gap-2 py-4">
                        {navItems.map((item) => {
                            const isActive = location.pathname === item.href ||
                                (item.href !== "/" && (location.pathname === item.href || location.pathname.startsWith(item.href + "/")));

                            return (
                                <Tooltip key={item.href}>
                                    <TooltipTrigger asChild>
                                        <Button
                                            variant="ghost"
                                            size="icon"
                                            className={cn(
                                                "size-11 rounded-xl transition-all duration-200",
                                                isActive
                                                    ? "bg-primary text-primary-foreground shadow-lg shadow-primary/25"
                                                    : "text-muted-foreground hover:text-foreground hover:bg-muted/50"
                                            )}
                                            asChild
                                        >
                                            <Link to={item.href}>
                                                <item.icon className="size-5" />
                                            </Link>
                                        </Button>
                                    </TooltipTrigger>
                                    <TooltipContent side="right" sideOffset={10}>
                                        {item.label}
                                    </TooltipContent>
                                </Tooltip>
                            );
                        })}
                    </nav>

                    <div className="flex flex-col items-center gap-2 py-4 border-t border-border">
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <Button
                                    variant="ghost"
                                    size="icon"
                                    className="size-11 text-muted-foreground hover:text-destructive hover:bg-destructive/10 rounded-xl transition-all duration-200"
                                    onClick={handleDisconnect}
                                >
                                    <LogOut className="size-5" />
                                </Button>
                            </TooltipTrigger>
                            <TooltipContent side="right" sideOffset={10}>
                                Disconnect
                            </TooltipContent>
                        </Tooltip>
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <div className="size-11 flex items-center justify-center">
                                    <ThemeSwitcher />
                                </div>
                            </TooltipTrigger>
                            <TooltipContent side="right" sideOffset={10}>
                                Toggle Theme
                            </TooltipContent>
                        </Tooltip>
                    </div>
                </aside>
            )}
        </TooltipProvider>
    );
}

export function AppLayout({ children }: { children: React.ReactNode }) {
    const { serverUrl, isConnected } = useServer();
    const location = useLocation();
    const isMobile = useIsMobile();

    // Scroll to top on every route change
    useEffect(() => {
        const container = document.getElementById('main-scroll-container');
        if (container) {
            container.scrollTop = 0;
        }
    }, [location.pathname]);

    const getDisplayUrl = () => {
        if (!serverUrl) return null;
        try {
            const url = new URL(serverUrl);
            return `${url.hostname}${url.port ? ':' + url.port : ''}`;
        } catch {
            return serverUrl;
        }
    };

    const fullWidthPaths = ['/movie', '/series', '/tv'];
    const isMediaDetailRoute = fullWidthPaths.some((prefix) => location.pathname.startsWith(prefix));

    const libraryPaths = ['/library', '/dashboard'];
    const isLibraryRoute = libraryPaths.some((prefix) => location.pathname.startsWith(prefix));

    return (
        <div className={cn(
            "min-h-screen",
            isLibraryRoute ? "bg-background" : "bg-gradient-to-br from-background via-background to-muted/20"
        )}>
            <Sidebar />
            <main
                id="main-scroll-container"
                className={cn(
                    isLibraryRoute
                        ? "h-dvh overflow-hidden"
                        : "max-h-dvh overflow-y-auto",
                    isMediaDetailRoute
                        ? (isMobile ? "pb-20 px-0 pt-0" : "pt-0 pb-0 pl-16")
                        : isLibraryRoute
                            ? (isMobile ? "pb-16 pt-0" : "pl-16")
                            : (isMobile ? "pb-20 px-0 pt-8" : "pt-8 pb-0 pl-16")
                )}
            >
                <div className={cn(
                    "w-full h-full",
                    isMediaDetailRoute
                        ? "mx-auto px-0"
                        : isLibraryRoute
                            ? "mx-0 px-0"
                            : "max-w-6xl mx-auto px-4 lg:px-0"
                )}>
                    {children}
                </div>
            </main>
            {serverUrl && location.pathname !== '/watch' && (
                <div className={cn(
                    "fixed z-50 hidden md:flex items-center gap-2 px-4 py-2 bg-card/80 backdrop-blur-md rounded-full border border-border shadow-lg bottom-4 right-4"
                )}>
                    <div className={cn(
                        "w-2 h-2 rounded-full animate-pulse",
                        isConnected ? "bg-success" : "bg-destructive"
                    )} />
                    <span className="text-xs font-medium text-foreground">
                        {getDisplayUrl()}
                    </span>
                </div>
            )}
        </div>
    );
}

export default AppLayout;
