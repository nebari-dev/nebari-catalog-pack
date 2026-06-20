package registry

// Fixtures returns a fixed, offline set of packs that mirror the real Nebari
// registry. Used by the server's demo mode for deterministic screenshots and
// quick local demos without network access. Icons are intentionally omitted so
// cards render their self-contained initials fallback (no external image loads).
func Fixtures() []Pack {
	mk := func(name, display, desc, category, level, latest string, versions ...string) Pack {
		all := append([]string{latest}, versions...)
		return Pack{
			Name:        name,
			DisplayName: display,
			Description: desc,
			Category:    category,
			Level:       level,
			Latest:      latest,
			Versions:    all,
			OCIRef:      "oci://quay.io/nebari/charts/" + name,
		}
	}
	return []Pack{
		mk("nebari-data-science-pack", "Data Science Pack",
			"Curated JupyterLab, conda environments, and data tooling for analysts.",
			"Data Science", "ga", "0.1.0"),
		mk("nebari-lgtm-pack", "LGTM Pack",
			"Loki, Grafana, Tempo, and Mimir — full observability for the cluster.",
			"Observability", "beta", "0.1.3", "0.1.2"),
		mk("nebari-superset", "Superset",
			"Apache Superset for exploring, visualizing, and sharing datasets.",
			"Analytics", "beta", "0.2.1", "0.2.0"),
		mk("nebari-langfuse", "Langfuse",
			"Open-source LLM engineering platform: tracing, evals, and prompt management.",
			"AI/ML", "alpha", "0.1.0"),
		mk("nebari-nebi-pack", "Nebi Pack",
			"The Nebari AI assistant and its supporting services.",
			"AI/ML", "alpha", "0.1.1", "0.1.0"),
		mk("provenance-collector", "Provenance Collector",
			"Compliance-grade supply-chain reports for every image running on the cluster.",
			"Security", "alpha", "0.1.0"),
		mk("skillsctl", "Skills Control",
			"Manage and serve reusable agent skills across the platform.",
			"AI/ML", "experimental", "0.1.0"),
		mk("nebari-chat", "Nebari Chat",
			"A collaborative chat workspace wired into platform identity.",
			"Collaboration", "experimental", "0.1.0"),
	}
}
