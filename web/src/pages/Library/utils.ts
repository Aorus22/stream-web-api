export const formatBytes = (bytes: number): string => {
    if (!bytes || bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
};

export const formatSpeed = (bytesPerSec: number): string => {
    if (!bytesPerSec || bytesPerSec === 0) return "0 B/s";
    return `${formatBytes(bytesPerSec)}/s`;
};

const BACKDROPS = [
    "linear-gradient(135deg, #2b1055 0%, #7597de 100%)",
    "linear-gradient(135deg, #310d0d 0%, #f12711 100%)",
    "linear-gradient(135deg, #0f2027 0%, #203a43 50%, #2c5364 100%)",
    "linear-gradient(135deg, #fceabb 0%, #f8b500 100%)",
    "linear-gradient(135deg, #1a2a6c 0%, #b21f1f 50%, #fdbb2d 100%)",
    "linear-gradient(135deg, #654ea3 0%, #eaafc8 100%)",
    "linear-gradient(135deg, #232526 0%, #414345 100%)",
    "linear-gradient(135deg, #1f4037 0%, #99f2c8 100%)",
    "linear-gradient(135deg, #870000 0%, #190a05 100%)",
    "linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%)",
];

const ACCENTS = [
    "oklch(65% 0.22 265)",
    "oklch(62% 0.2 25)",
    "oklch(70% 0.15 200)",
    "oklch(75% 0.18 80)",
    "oklch(60% 0.18 320)",
    "oklch(68% 0.16 155)",
    "oklch(72% 0.2 50)",
    "oklch(60% 0.15 240)",
    "oklch(68% 0.2 30)",
    "oklch(60% 0.18 290)",
];

export const hashString = (str: string): number => {
    let h = 0;
    for (let i = 0; i < str.length; i++) {
        h = (h << 5) - h + str.charCodeAt(i);
        h |= 0;
    }
    return Math.abs(h);
};

export const getBackdrop = (id: string): string => {
    return BACKDROPS[hashString(id) % BACKDROPS.length];
};

export const getAccent = (id: string): string => {
    return ACCENTS[hashString(id) % ACCENTS.length];
};

export const isCompleted = (progress: number): boolean => progress >= 100;
