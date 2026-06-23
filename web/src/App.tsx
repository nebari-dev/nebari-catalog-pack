// Stage-1 foundation: proves the @nebari registry components + theme tokens
// render and build. The full catalog UI lands in a follow-up.
import { Button } from "@/ui/button";
import { Badge } from "@/ui/badge";

export function App() {
  return (
    <div className="dark min-h-screen bg-background text-foreground flex items-center justify-center">
      <div className="flex flex-col items-center gap-5">
        <h1 className="text-2xl font-semibold tracking-tight">Nebari Pack Catalog</h1>
        <p className="text-muted-foreground text-sm">
          SPA scaffold — Vite + React + TS + Tailwind v4 + @nebari components
        </p>
        <div className="flex items-center gap-3">
          <Button>Install</Button>
          <Button variant="outline">Preview</Button>
          <Badge>GA</Badge>
          <Badge variant="secondary">Data Science</Badge>
          <Badge variant="outline">v0.1.0</Badge>
        </div>
      </div>
    </div>
  );
}
