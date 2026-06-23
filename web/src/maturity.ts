import type { BadgeProps } from "@/ui/badge";

type Variant = NonNullable<BadgeProps["variant"]>;

// Maturity -> Badge variant + label, keyed by the API's level strings.
const VARIANT: Record<string, Variant> = {
  ga: "success",
  beta: "info",
  alpha: "warning",
  experimental: "amber",
};
const RANK: Record<string, number> = { ga: 0, beta: 1, alpha: 2, experimental: 3 };

export function maturityVariant(m: string): Variant {
  return VARIANT[m] ?? "secondary";
}
export function maturityLabel(m: string): string {
  return m ? m.toUpperCase() : "—";
}
export function maturityRank(m: string): number {
  return RANK[m] ?? 99;
}
