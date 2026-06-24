import { useEffect, useState } from "react";
import { Eye, Package, RotateCcw, X } from "lucide-react";
import { Button } from "@/ui/button";
import { getValues, postInstall, type InstallResult, type Pack } from "@/api";
import { ResultBlock } from "@/components/ResultBlock";

type Props = {
  pack: Pack | null;
  mode: "dry-run" | "ready";
  onClose: () => void;
  onInstalled: () => void;
};

// Right-side drawer for editing the Helm values an install will use. Prefilled
// with the catalog's generated values (the nebariapp/landingPage contract); the
// edited YAML is sent verbatim as spec.source.helm.values.
export function ValuesDrawer({ pack, mode, onClose, onInstalled }: Props) {
  const [version, setVersion] = useState("");
  const [values, setValues] = useState("");
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState<"preview" | "install" | null>(null);
  const [result, setResult] = useState<InstallResult | undefined>();

  // (Re)load the default values whenever the pack or version changes.
  useEffect(() => {
    if (!pack) return;
    const v = version || pack.version;
    setLoading(true);
    setResult(undefined);
    getValues(pack.id, v)
      .then(setValues)
      .finally(() => setLoading(false));
  }, [pack, version]);

  // Reset version + result when a different pack opens the drawer.
  useEffect(() => {
    if (pack) setVersion(pack.version);
  }, [pack]);

  // Close on Escape.
  useEffect(() => {
    if (!pack) return;
    const k = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", k);
    return () => window.removeEventListener("keydown", k);
  }, [pack, onClose]);

  if (!pack) return null;

  const run = async (kind: "preview" | "install") => {
    setBusy(kind);
    try {
      const res = await postInstall({
        pack: pack.id,
        version: version || pack.version,
        dryRun: kind === "preview" || mode === "dry-run",
        values,
      });
      setResult(res);
      if (kind === "install" && res.ok && !res.dryRun) onInstalled();
    } catch (e) {
      setResult({ ok: false, dryRun: true, pack: pack.id, version, message: String(e) });
    } finally {
      setBusy(null);
    }
  };

  const reload = () => getValues(pack.id, version || pack.version).then(setValues);

  return (
    <div className="drawer-overlay" onClick={onClose}>
      <aside className="drawer" role="dialog" aria-label={`Configure ${pack.name}`} onClick={(e) => e.stopPropagation()}>
        <div className="drawer-head">
          <div>
            <div className="drawer-title">{pack.name}</div>
            <div className="drawer-sub">Edit the Helm values for this install</div>
          </div>
          <button className="drawer-x" aria-label="Close" onClick={onClose}>
            <X />
          </button>
        </div>

        <div className="drawer-controls">
          <span className="selct mono">
            <select value={version || pack.version} onChange={(e) => setVersion(e.target.value)} aria-label="Version">
              {pack.versions.map((v) => (
                <option key={v} value={v}>
                  {v.startsWith("v") ? v : `v${v}`}
                </option>
              ))}
            </select>
          </span>
          <span className="spacer" style={{ flex: 1 }} />
          <Button variant="ghost" size="sm" onClick={reload} title="Reset to generated defaults">
            <RotateCcw />
            Reset
          </Button>
        </div>

        <label className="drawer-label" htmlFor="values-editor">
          spec.source.helm.values
        </label>
        <textarea
          id="values-editor"
          className="yaml-editor"
          spellCheck={false}
          value={loading ? "Loading…" : values}
          disabled={loading}
          onChange={(e) => setValues(e.target.value)}
        />

        {result && (
          <div className="drawer-result">
            <ResultBlock result={result} />
          </div>
        )}

        <div className="drawer-foot">
          <Button variant="outline" loading={busy === "preview"} loadingText="Rendering…" onClick={() => run("preview")}>
            <Eye />
            Preview
          </Button>
          {mode === "ready" && (
            <Button className="grow" loading={busy === "install"} loadingText="Installing…" onClick={() => run("install")}>
              <Package />
              Install
            </Button>
          )}
        </div>
      </aside>
    </div>
  );
}
