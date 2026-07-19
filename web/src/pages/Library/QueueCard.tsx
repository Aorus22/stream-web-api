import { Play, Pause, FileVideo, HardDrive } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatSpeed, isCompleted } from "./utils";
import type { LibraryItem } from "./types";

type Props = {
    item: LibraryItem;
    active: boolean;
    onClick: () => void;
};

const STATUS_LABEL: Record<string, string> = {
    downloading: "Downloading",
    completed: "Completed",
    failed: "Failed",
    missing: "Missing",
    orphan: "Orphan",
    on_demand: "On Demand",
    paused: "Paused",
    seeding: "Seeding",
    streaming: "Streaming",
};

const shortName = (name: string, max = 60): string => {
    if (!name) return "Untitled";
    if (name.length <= max) return name;
    return name.slice(0, max - 1) + "…";
};

export default function QueueCard({ item, active, onClick }: Props) {
    const statusLabel = STATUS_LABEL[item.status.toLowerCase()] ?? item.status;
    const completed = isCompleted(item.progress);
    const accent = item.accent;

    return (
        <div
            role="button"
            tabIndex={0}
            onClick={onClick}
            onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onClick();
                }
            }}
            className={cn(
                "group relative flex cursor-pointer gap-3 overflow-hidden rounded-2xl border p-3 transition-all duration-300",
                "border-transparent bg-white/[0.03] hover:scale-[1.015] hover:border-white/10 hover:bg-white/[0.06]",
                active && "border-white/20 bg-white/[0.08] shadow-2xl shadow-black/40"
            )}
        >
            <div
                className="relative flex h-[110px] w-[80px] flex-shrink-0 items-center justify-center overflow-hidden rounded-lg text-white/60 shadow-lg shadow-black/40"
                style={{ background: item.backdrop }}
            >
                {item.type === "torrent" ? (
                    completed ? (
                        <Play className="h-6 w-6 opacity-70" fill="currentColor" />
                    ) : (
                        <FileVideo className="h-6 w-6 opacity-60" />
                    )
                ) : (
                    <HardDrive className="h-6 w-6 opacity-60" />
                )}
            </div>

            <div className="flex min-w-0 flex-1 flex-col justify-center">
                <div className="mb-1.5 truncate text-[15px] font-bold leading-tight">
                    {shortName(item.name, 40)}
                </div>

                <div className="mb-2.5 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                    <span
                        className={cn(
                            "rounded-full px-1.5 py-0.5 text-[10px] font-extrabold",
                            completed
                                ? "bg-emerald-500/15 text-emerald-400"
                                : item.status === "paused" || item.status === "failed" || item.status === "missing"
                                    ? "bg-white/10 text-white/60"
                                    : "bg-white/10 text-white"
                        )}
                    >
                        {statusLabel}
                    </span>
                    <span className="opacity-50">•</span>
                    <span className="font-mono normal-case tracking-normal text-white/60">
                        {item.size ? item.size : "—"}
                    </span>
                </div>

                <div className="relative h-1 overflow-hidden rounded-full bg-white/10">
                    <div
                        className="absolute inset-y-0 left-0 rounded-full transition-all duration-500 ease-out"
                        style={{
                            width: `${Math.min(100, Math.max(0, item.progress))}%`,
                            background: accent,
                            boxShadow: `0 0 12px ${accent}`,
                        }}
                    />
                </div>

                <div className="mt-2 flex items-center justify-between font-mono text-[11px]">
                    <span style={{ color: accent }}>
                        {item.progress.toFixed(1)}%
                    </span>
                    <span className="flex items-center gap-1 text-white/60">
                        {!completed && item.speed > 0 ? (
                            <>
                                <span style={{ color: accent }}>{formatSpeed(item.speed)}</span>
                            </>
                        ) : item.status === "paused" || item.speed === 0 ? (
                            <>
                                <Pause className="h-3 w-3" />
                                <span>Idle</span>
                            </>
                        ) : completed ? (
                            <span className="text-emerald-400">Ready</span>
                        ) : (
                            <span>—</span>
                        )}
                    </span>
                </div>
            </div>
        </div>
    );
}
