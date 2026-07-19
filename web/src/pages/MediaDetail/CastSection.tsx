import { ScrollArea, ScrollBar } from "@/components/ui/scroll-area";

type CastSectionProps = {
    cast: string[];
};

export default function CastSection({ cast }: CastSectionProps) {
    return (
        <div className="w-full px-4 py-8">
            <h2 className="text-xl font-semibold mb-4">Full Cast</h2>
            <ScrollArea className="w-full">
                <div className="flex gap-4 pb-4">
                    {cast.map((member, i) => (
                        <div
                            key={i}
                            className="w-[100px] flex-shrink-0 text-left"
                        >
                            <div className="w-16 h-16 rounded-full overflow-hidden bg-muted mb-2 flex items-center justify-center">
                                <span className="text-xl text-muted-foreground">
                                    {member.charAt(0)}
                                </span>
                            </div>
                            <p className="text-sm font-medium leading-tight">{member}</p>
                        </div>
                    ))}
                </div>
                <ScrollBar orientation="horizontal" />
            </ScrollArea>
        </div>
    );
}
