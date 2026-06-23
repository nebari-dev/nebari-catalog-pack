// Typed client for the catalog's Go JSON API. URLs are relative so the SPA
// works when served under a sub-path gateway.

export type Pack = {
  id: string;
  name: string;
  chart: string;
  description: string;
  category: string;
  maturity: string; // experimental | alpha | beta | ga
  version: string;
  versions: string[];
  icon?: string;
  homepage?: string;
  deprecated?: boolean;
  installed: boolean;
  health?: string;
  sync?: string;
};

export type Gitops = {
  installEnabled: boolean;
  dryRun: boolean;
  repo: string;
  branch: string;
  rootApp: string;
  argocdEnabled: boolean;
  argocdHealth?: string;
  registryRef: string;
  installedCount: number;
};

export type InstallResult = {
  ok: boolean;
  dryRun: boolean;
  pack: string;
  version: string;
  message: string;
  file?: string;
  commitHash?: string;
  manifest?: string;
  health?: string;
  sync?: string;
};

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url, { headers: { Accept: "application/json" } });
  if (!res.ok) throw new Error(`${url}: ${res.status}`);
  return res.json() as Promise<T>;
}

export async function getPacks(): Promise<Pack[]> {
  const data = await getJSON<{ packs: Pack[] }>("api/packs");
  return data.packs;
}

export function getGitops(): Promise<Gitops> {
  return getJSON<Gitops>("api/gitops");
}

export async function postInstall(body: {
  pack: string;
  version: string;
  dryRun: boolean;
}): Promise<InstallResult> {
  const res = await fetch("api/install", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return res.json() as Promise<InstallResult>;
}
