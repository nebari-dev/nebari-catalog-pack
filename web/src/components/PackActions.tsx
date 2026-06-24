import { Check, Eye, Package, SlidersHorizontal } from "lucide-react";
import { Button } from "@/ui/button";
import type { InstallResult, Pack } from "@/api";

export type PackProps = {
  pack: Pack;
  mode: "dry-run" | "ready";
  installed: boolean;
  busy?: "preview" | "install";
  ver: string;
  setVer: (id: string, v: string) => void;
  onPreview: (p: Pack) => void;
  onInstall: (p: Pack) => void;
  onConfigure: (p: Pack) => void;
  result?: InstallResult;
};

function VersionSelect({ pack, ver, setVer }: Pick<PackProps, "pack" | "ver" | "setVer">) {
  if (pack.versions.length <= 1) return null;
  return (
    <span className="selct mono">
      <select value={ver} onChange={(e) => setVer(pack.id, e.target.value)} aria-label={`${pack.name} version`}>
        {pack.versions.map((v) => (
          <option key={v} value={v}>
            {v.startsWith("v") ? v : `v${v}`}
          </option>
        ))}
      </select>
    </span>
  );
}

// Per-pack action row, shared by the grid card and table row. `compact` (table)
// drops the inline version select (version is its own column) and shrinks.
export function PackActions(props: PackProps & { compact?: boolean }) {
  const { pack, mode, installed, busy, onPreview, onInstall, onConfigure, compact } = props;
  const size = compact ? "sm" : "default";
  const grow = compact ? "" : "grow";

  if (installed) {
    return (
      <Button variant="secondary" size={size} className={grow} disabled>
        <Check />
        Installed
      </Button>
    );
  }

  const configure = (
    <Button
      variant="outline"
      size={compact ? "icon-sm" : "icon"}
      onClick={() => onConfigure(pack)}
      title="Configure values"
      aria-label={`Configure values for ${pack.name}`}
    >
      <SlidersHorizontal />
    </Button>
  );

  return (
    <>
      {configure}
      {!compact && <VersionSelect {...props} />}
      <Button
        variant="outline"
        size={size}
        className={mode === "dry-run" ? grow : ""}
        loading={busy === "preview"}
        loadingText={compact || mode !== "dry-run" ? undefined : "Rendering…"}
        onClick={() => onPreview(pack)}
      >
        <Eye />
        Preview
      </Button>
      {mode === "ready" && (
        <Button
          size={size}
          className={grow}
          loading={busy === "install"}
          loadingText={compact ? undefined : "Installing…"}
          onClick={() => onInstall(pack)}
        >
          <Package />
          Install
        </Button>
      )}
    </>
  );
}
