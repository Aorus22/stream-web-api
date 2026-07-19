import { Button } from "@/components/ui/button";
import { ChevronLeft, ChevronRight } from "lucide-react";

type PaginationControlsProps = {
    categorySkip: number;
    hasMore: boolean;
    onPrev: () => void;
    onNext: () => void;
};

export default function PaginationControls({ categorySkip, hasMore, onPrev, onNext }: PaginationControlsProps) {
    return (
        <div className="flex items-center justify-center gap-4">
            <Button
                variant="outline"
                size="sm"
                disabled={categorySkip <= 0}
                onClick={onPrev}
            >
                <ChevronLeft className="size-4 mr-1" />
                Previous
            </Button>
            <span className="text-sm text-muted-foreground">
                Page {Math.floor(categorySkip / 20) + 1}
            </span>
            <Button
                variant="outline"
                size="sm"
                disabled={!hasMore}
                onClick={onNext}
            >
                Next
                <ChevronRight className="size-4 ml-1" />
            </Button>
        </div>
    );
}
