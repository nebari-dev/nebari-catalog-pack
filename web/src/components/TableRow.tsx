import { Badge } from "@/ui/badge";
import { maturityLabel, maturityVariant } from "@/maturity";
import { NebariMark } from "@/components/NebariMark";
import { PackActions, type PackProps } from "@/components/PackActions";
import { ResultBlock } from "@/components/ResultBlock";

export function TableRow(props: PackProps) {
  const { pack, installed, result } = props;
  return (
    <>
      <tr className="row" data-pack={pack.id}>
        <td>
          <div className="pk">
            <span className="tile">
              {pack.icon ? <img src={pack.icon} alt="" loading="lazy" /> : <NebariMark />}
            </span>
            <div>
              <div className="nm">{pack.name}</div>
              <div className="ds">{pack.description}</div>
            </div>
          </div>
        </td>
        <td>{pack.category && <Badge variant="secondary">{pack.category}</Badge>}</td>
        <td>
          <Badge variant={maturityVariant(pack.maturity)}>{maturityLabel(pack.maturity)}</Badge>
        </td>
        <td>
          <span className="ver-cell">{pack.version ? (pack.version.startsWith("v") ? pack.version : `v${pack.version}`) : "—"}</span>
        </td>
        <td>
          <span className="sync">
            <span className="sd" style={{ background: installed ? "var(--success-foreground)" : "var(--muted-foreground)" }} />
            {installed ? `${pack.sync ?? "Synced"} · ${pack.health ?? "Healthy"}` : "Not installed"}
          </span>
        </td>
        <td>
          <div className="act-cell">
            <PackActions {...props} compact />
          </div>
        </td>
      </tr>
      {result && (
        <tr className="detail-row">
          <td colSpan={6}>
            <ResultBlock result={result} />
          </td>
        </tr>
      )}
    </>
  );
}
