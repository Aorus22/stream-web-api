import { Badge } from "@/components/ui/badge";

type CrewSectionProps = {
    director?: string[];
    writer?: string[];
};

export default function CrewSection({ director, writer }: CrewSectionProps) {
    if (!director?.length && !writer?.length) return null;

    return (
        <div className="w-full px-4 py-8 border-t">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                {director?.length ? (
                    <div>
                        <h3 className="text-sm uppercase tracking-wider text-muted-foreground mb-2">
                            Director
                        </h3>
                        <div className="flex flex-wrap gap-2">
                            {director.map((d, i) => (
                                <Badge key={i} variant="outline">{d}</Badge>
                            ))}
                        </div>
                    </div>
                ) : null}
                {writer?.length ? (
                    <div>
                        <h3 className="text-sm uppercase tracking-wider text-muted-foreground mb-2">
                            Writer
                        </h3>
                        <div className="flex flex-wrap gap-2">
                            {writer.map((w, i) => (
                                <Badge key={i} variant="outline">{w}</Badge>
                            ))}
                        </div>
                    </div>
                ) : null}
            </div>
        </div>
    );
}
