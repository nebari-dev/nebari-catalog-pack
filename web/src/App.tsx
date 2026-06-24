import { useEffect, useMemo, useRef, useState } from "react";
import { GitBranch, Inbox, LayoutGrid, List, Monitor, Moon, Search, Sun } from "lucide-react";
import "./catalog.css";
import { getGitops, getPacks, postInstall, type Gitops, type InstallResult, type Pack } from "@/api";
import { maturityLabel, maturityRank } from "@/maturity";
import { Switch } from "@/ui/switch";
import { NebariMark } from "@/components/NebariMark";
import { GridCard } from "@/components/GridCard";
import { TableRow } from "@/components/TableRow";
import { ValuesDrawer } from "@/components/ValuesDrawer";

type Theme = "auto" | "light" | "dark";
type Mode = "dry-run" | "ready";

function useStored<T>(key: string, initial: T): [T, (v: T) => void] {
  const [v, setV] = useState<T>(() => {
    try {
      const s = localStorage.getItem(key);
      return s == null ? initial : (JSON.parse(s) as T);
    } catch {
      return initial;
    }
  });
  useEffect(() => {
    try {
      localStorage.setItem(key, JSON.stringify(v));
    } catch {
      /* ignore */
    }
  }, [key, v]);
  return [v, setV];
}

function countBy<T>(items: T[], key: (t: T) => string): Map<string, number> {
  const m = new Map<string, number>();
  for (const it of items) {
    const k = key(it);
    if (k) m.set(k, (m.get(k) ?? 0) + 1);
  }
  return m;
}

export function App() {
  const [packs, setPacks] = useState<Pack[] | null>(null);
  const [gitops, setGitops] = useState<Gitops | null>(null);
  const [loadErr, setLoadErr] = useState<string | null>(null);

  const [theme, setTheme] = useStored<Theme>("nebari-catalog:theme", "auto");
  const [view, setView] = useStored<"grid" | "table">("nebari-catalog:view", "grid");
  const [mode, setMode] = useStored<Mode>("nebari-catalog:mode", "dry-run");

  const [query, setQuery] = useState("");
  const [cats, setCats] = useState<string[]>([]);
  const [mats, setMats] = useState<string[]>([]);
  const [sort, setSort] = useState<"maturity" | "name" | "category">("maturity");

  const [vers, setVers] = useState<Record<string, string>>({});
  const [results, setResults] = useState<Record<string, InstallResult>>({});
  const [busy, setBusy] = useState<Record<string, "preview" | "install">>({});
  const [configure, setConfigure] = useState<Pack | null>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  // Load packs + gitops context.
  const reload = () => getPacks().then(setPacks).catch((e) => setLoadErr(String(e)));
  useEffect(() => {
    reload();
    getGitops().then(setGitops).catch(() => {});
  }, []);

  // Seed version selections + initial dry-run mode from the cluster context.
  useEffect(() => {
    if (packs) setVers((s) => ({ ...Object.fromEntries(packs.map((p) => [p.id, p.version])), ...s }));
  }, [packs]);
  const installEnabled = gitops?.installEnabled ?? false;
  const effectiveMode: Mode = installEnabled ? mode : "dry-run";

  // Theme: toggle the .dark class from the choice + OS preference.
  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => {
      const dark = theme === "dark" || (theme === "auto" && mq.matches);
      document.documentElement.classList.toggle("dark", dark);
    };
    apply();
    if (theme === "auto") {
      mq.addEventListener("change", apply);
      return () => mq.removeEventListener("change", apply);
    }
  }, [theme]);

  // ⌘K / Ctrl-K focuses search.
  useEffect(() => {
    const k = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener("keydown", k);
    return () => window.removeEventListener("keydown", k);
  }, []);

  const setVer = (id: string, v: string) => setVers((s) => ({ ...s, [id]: v }));
  const toggle = (arr: string[], setArr: (v: string[]) => void, v: string) =>
    setArr(arr.includes(v) ? arr.filter((x) => x !== v) : [...arr, v]);

  const run = async (pack: Pack, kind: "preview" | "install") => {
    const dryRun = kind === "preview" ? true : effectiveMode === "dry-run";
    setBusy((b) => ({ ...b, [pack.id]: kind }));
    try {
      const res = await postInstall({ pack: pack.id, version: vers[pack.id] ?? pack.version, dryRun });
      setResults((r) => ({ ...r, [pack.id]: res }));
      if (kind === "install" && res.ok && !res.dryRun) {
        reload();
        getGitops().then(setGitops).catch(() => {});
      }
    } catch (e) {
      setResults((r) => ({ ...r, [pack.id]: { ok: false, dryRun, pack: pack.id, version: vers[pack.id] ?? pack.version, message: String(e) } }));
    } finally {
      setBusy((b) => {
        const n = { ...b };
        delete n[pack.id];
        return n;
      });
    }
  };

  const all = packs ?? [];
  const catCounts = useMemo(() => countBy(all, (p) => p.category), [all]);
  const matCounts = useMemo(() => countBy(all, (p) => p.maturity), [all]);
  const categories = useMemo(() => [...catCounts.keys()].sort(), [catCounts]);
  const maturities = useMemo(() => [...matCounts.keys()].sort((a, b) => maturityRank(a) - maturityRank(b)), [matCounts]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = all.filter((p) => {
      const okQ = !q || p.name.toLowerCase().includes(q) || p.category.toLowerCase().includes(q) || p.description.toLowerCase().includes(q);
      const okC = cats.length === 0 || cats.includes(p.category);
      const okM = mats.length === 0 || mats.includes(p.maturity);
      return okQ && okC && okM;
    });
    return [...list].sort((a, b) => {
      if (sort === "name") return a.name.localeCompare(b.name);
      if (sort === "category") return a.category.localeCompare(b.category) || a.name.localeCompare(b.name);
      return maturityRank(a.maturity) - maturityRank(b.maturity) || a.name.localeCompare(b.name);
    });
  }, [all, query, cats, mats, sort]);

  const installedCount = all.filter((p) => p.installed).length;
  const anyFilter = cats.length || mats.length || query;
  const dot = (color: string) => <span className="sd" style={{ background: color }} />;

  const packProps = (p: Pack) => ({
    pack: p,
    mode: effectiveMode,
    installed: p.installed,
    busy: busy[p.id],
    ver: vers[p.id] ?? p.version,
    setVer,
    onPreview: (x: Pack) => run(x, "preview"),
    onInstall: (x: Pack) => run(x, "install"),
    onConfigure: (x: Pack) => setConfigure(x),
    result: results[p.id],
  });

  const themeBtns: [Theme, typeof Monitor, string][] = [
    ["auto", Monitor, "Auto"],
    ["light", Sun, "Light"],
    ["dark", Moon, "Dark"],
  ];

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <NebariMark className="brandmark" />
          <div>
            <h1>Nebari Pack Catalog</h1>
            <p className="sub">
              Browse and install packs from <span className="mono-chip">{gitops?.registryRef ?? "quay.io/nebari/charts"}</span>
            </p>
          </div>
        </div>
        <div className="spacer" />
        <div className="seg" role="group" aria-label="Theme">
          {themeBtns.map(([v, Ic, l]) => (
            <button key={v} className={theme === v ? "on" : ""} aria-pressed={theme === v} onClick={() => setTheme(v)}>
              <Ic />
              {l}
            </button>
          ))}
        </div>
        <label className="drymode" title="Dry-run renders the ArgoCD manifest without committing it to the GitOps repo">
          <span className="drymode-label">Dry run</span>
          <Switch
            checked={effectiveMode === "dry-run"}
            disabled={!installEnabled}
            onCheckedChange={(v: boolean) => setMode(v ? "dry-run" : "ready")}
          />
        </label>
      </header>

      <div className="gitops">
        <span className="seg-i">
          <span className="k">repo</span>
          <span className="v">{gitops?.repo || "—"}</span>
        </span>
        <span className="div" />
        <span className="seg-i">
          <GitBranch size={13} />
          <span className="v">{gitops?.branch || "main"}</span>
        </span>
        <span className="div" />
        <span className="seg-i">
          <span className="k">argocd</span>
          {dot(gitops?.argocdEnabled ? "var(--success-foreground)" : "var(--muted-foreground)")}
          <span className="v">{gitops?.argocdEnabled ? (gitops.argocdHealth || "healthy").toLowerCase() : "off"}</span>
        </span>
        <span className="div" />
        <span className="seg-i">
          <span className="k">mode</span>
          {dot(effectiveMode === "dry-run" ? "var(--warning-foreground)" : "var(--success-foreground)")}
          <span className="v">{effectiveMode === "dry-run" ? "dry-run" : "install-ready"}</span>
        </span>
        <span className="spacer" style={{ flex: 1 }} />
        <span className="seg-i">
          <span className="k">{installedCount} installed</span>
        </span>
      </div>

      <div className="body">
        <aside className="rail">
          <div>
            <h3>Category</h3>
            <div className="facet">
              {categories.map((c) => (
                <label key={c} className={"opt" + (cats.includes(c) ? " on" : "")} onClick={() => toggle(cats, setCats, c)}>
                  <span className="box">{cats.includes(c) ? "✓" : ""}</span>
                  {c}
                  <span className="ct">{catCounts.get(c)}</span>
                </label>
              ))}
            </div>
          </div>
          <div>
            <h3>Maturity</h3>
            <div className="facet">
              {maturities.map((m) => (
                <label key={m} className={"opt" + (mats.includes(m) ? " on" : "")} onClick={() => toggle(mats, setMats, m)}>
                  <span className="box">{mats.includes(m) ? "✓" : ""}</span>
                  {maturityLabel(m)}
                  <span className="ct">{matCounts.get(m)}</span>
                </label>
              ))}
            </div>
          </div>
          <div style={{ marginTop: "auto" }}>
            <h3>Sort</h3>
            <span className="selct" style={{ display: "block" }}>
              <select value={sort} onChange={(e) => setSort(e.target.value as typeof sort)} style={{ width: "100%" }}>
                <option value="maturity">Maturity</option>
                <option value="name">Name (A–Z)</option>
                <option value="category">Category</option>
              </select>
            </span>
          </div>
        </aside>

        <main className="main">
          <div className="toolrow">
            <div className="search">
              <Search />
              <input
                ref={searchRef}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search packs by name, category, or description…"
              />
              <span className="kbd">⌘K</span>
            </div>
            <div className="seg" role="group" aria-label="View">
              <button className={view === "grid" ? "on" : ""} aria-pressed={view === "grid"} onClick={() => setView("grid")}>
                <LayoutGrid />
                Grid
              </button>
              <button className={view === "table" ? "on" : ""} aria-pressed={view === "table"} onClick={() => setView("table")}>
                <List />
                Table
              </button>
            </div>
            <span className="count">
              <b>{filtered.length}</b>
              {anyFilter ? ` of ${all.length}` : ""} packs
            </span>
          </div>

          {loadErr ? (
            <div className="empty">
              <Inbox size={34} />
              <div>Could not reach the registry.</div>
            </div>
          ) : packs === null ? (
            <div className="empty">
              <div>Loading packs…</div>
            </div>
          ) : filtered.length === 0 ? (
            <div className="empty">
              <Inbox size={34} />
              <div>No packs match your filters.</div>
            </div>
          ) : view === "grid" ? (
            <div className="grid">
              {filtered.map((p) => (
                <GridCard key={p.id} {...packProps(p)} />
              ))}
            </div>
          ) : (
            <div className="tablewrap">
              <table>
                <thead>
                  <tr>
                    <th>Pack</th>
                    <th>Category</th>
                    <th>Maturity</th>
                    <th>Version</th>
                    <th>Cluster state</th>
                    <th style={{ textAlign: "right" }}>Action</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((p) => (
                    <TableRow key={p.id} {...packProps(p)} />
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </main>
      </div>

      <footer className="foot">
        <span>
          {all.length} packs · {installedCount} installed · {all.length - installedCount} available
        </span>
        <span>nebari-catalog-pack</span>
      </footer>

      <ValuesDrawer
        pack={configure}
        mode={effectiveMode}
        onClose={() => setConfigure(null)}
        onInstalled={() => {
          reload();
          getGitops().then(setGitops).catch(() => {});
          setConfigure(null);
        }}
      />
    </div>
  );
}
