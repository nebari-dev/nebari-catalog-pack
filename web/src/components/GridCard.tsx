import { Badge } from "@/ui/badge";
import { maturityLabel, maturityVariant } from "@/maturity";
import { NebariMark } from "@/components/NebariMark";
import { PackActions, type PackProps } from "@/components/PackActions";
import { ResultBlock } from "@/components/ResultBlock";

export function GridCard(props: PackProps) {
  const { pack, installed, result } = props;
  return (
    <div className="card" data-pack={pack.id}>
      <div className="ctop">
        <span className="tile">
          {pack.icon ? <img src={pack.icon} alt="" loading="lazy" /> : <NebariMark />}
        </span>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: "flex", alignItems: "flex-start", gap: 8 }}>
            <h2 style={{ flex: 1 }}>{pack.name}</h2>
            <Badge variant={maturityVariant(pack.maturity)}>{maturityLabel(pack.maturity)}</Badge>
          </div>
        </div>
      </div>
      <p className="desc">{pack.description}</p>
      <div className="meta">
        {pack.category && <Badge variant="secondary">{pack.category}</Badge>}
        {pack.version && <Badge variant="version">{pack.version.startsWith("v") ? pack.version : `v${pack.version}`}</Badge>}
        {pack.deprecated && <Badge variant="warning">deprecated</Badge>}
      </div>
      <div className="statusline">
        <span className="sd" style={{ background: installed ? "var(--success-foreground)" : "var(--muted-foreground)" }} />
        {installed ? `${pack.sync ?? "Synced"} · ${pack.health ?? "Healthy"}` : "Not installed"}
      </div>
      <div className="act">
        <PackActions {...props} />
      </div>
      <ResultBlock result={result} />
    </div>
  );
}
