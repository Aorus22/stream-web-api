import { useEffect, useState } from "react";
import { useServer } from "@/contexts/ServerContext";

export type DownloadProgress = {
  id: number;
  progress: number;
  downloadedBytes: number;
  totalBytes: number;
  status: string;
};

export function useDownloadProgress(downloadId: number | null) {
  const { serverUrl } = useServer();
  const [progress, setProgress] = useState<DownloadProgress | null>(null);

  useEffect(() => {
    if (!serverUrl || !downloadId) return;

    const eventSource = new EventSource(`${serverUrl}/api/direct/${downloadId}/progress`);

    eventSource.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data) as DownloadProgress;
        setProgress(data);
      } catch {
        // ignore
      }
    };

    eventSource.onerror = () => {
      eventSource.close();
    };

    return () => {
      eventSource.close();
    };
  }, [downloadId, serverUrl]);

  return progress;
}

