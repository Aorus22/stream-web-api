import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Copy, Download, HardDrive, Loader2, Users, Database, Check } from "lucide-react";
import type { SearchResult } from "./types";

type ResultCardProps = {
  result: SearchResult;
  addingMagnet: string | null;
  onAdd: (magnet: string) => void;
  onCopy: (magnet: string) => void;
  copiedMagnet: string | null;
};

export default function ResultCard({ result, addingMagnet, onAdd, onCopy, copiedMagnet }: ResultCardProps) {
  const isAdding = addingMagnet === result.magnet;
  const isCopied = copiedMagnet === result.magnet;
  const hasMagnet = !!result.magnet;

  return (
    <Card className="border">
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div className="flex-1 space-y-1">
          <CardTitle className="text-lg">{result.name}</CardTitle>
          <div className="flex flex-wrap gap-2 text-sm text-muted-foreground">
            <Badge variant="outline" className="gap-1">
              <HardDrive className="h-3 w-3" />
              {result.size}
            </Badge>
            <Badge variant="outline" className="gap-1 text-green-600">
              <Users className="h-3 w-3" />
              {result.seeders} / {result.leechers}
            </Badge>
            {result.category && (
              <Badge variant="outline" className="gap-1">
                <Database className="h-3 w-3" />
                {result.category}
              </Badge>
            )}
          </div>
          {result.uploadedBy && result.dateUploaded && (
            <CardDescription className="text-xs">
              Uploaded by {result.uploadedBy} on {result.dateUploaded}
            </CardDescription>
          )}
        </div>

        {hasMagnet && (
          <div className="flex gap-2" onClick={(e) => e.stopPropagation()}>
            <Button
              variant="outline"
              size="sm"
              onClick={() => onCopy(result.magnet)}
              className="gap-1"
            >
              {isCopied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
              <span className="hidden sm:inline">{isCopied ? "Copied" : "Copy"}</span>
            </Button>
            <Button
              size="sm"
              disabled={isAdding}
              onClick={() => onAdd(result.magnet)}
              className="gap-1"
            >
              {isAdding ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Download className="h-4 w-4" />
              )}
              <span className="hidden sm:inline">Add</span>
            </Button>
          </div>
        )}
      </CardHeader>
      {(result.type || result.language) && (
        <CardContent className="pt-0 text-xs text-muted-foreground">
          <div className="flex flex-wrap gap-3">
            {result.type && <span>Provider: {result.type}</span>}
            {result.language && <span>Language: {result.language}</span>}
          </div>
        </CardContent>
      )}
    </Card>
  );
}
