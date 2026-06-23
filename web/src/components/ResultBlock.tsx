import { CheckCircle2, ChevronRight, XCircle } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/ui/alert";
import type { InstallResult } from "@/api";

// The preview/install outcome, shown inline under a card or table row.
export function ResultBlock({ result }: { result?: InstallResult }) {
  if (!result) return null;
  const title = !result.ok ? "Failed" : result.dryRun ? "Preview ready" : "Installed";
  return (
    <div className="result">
      <Alert variant={result.ok ? "success" : "destructive"}>
        {result.ok ? <CheckCircle2 /> : <XCircle />}
        <AlertTitle>{title}</AlertTitle>
        <AlertDescription>
          <span>
            {result.message}
            {result.file ? (
              <>
                {" · "}
                <code>{result.file}</code>
              </>
            ) : null}
            {result.commitHash ? (
              <>
                {" @ "}
                <code>{result.commitHash.slice(0, 7)}</code>
              </>
            ) : null}
            {result.health ? ` · ${result.sync ?? ""} ${result.health}` : ""}
          </span>
          {result.manifest ? (
            <details className="manifest">
              <summary>
                <ChevronRight className="chev" />
                Application manifest
              </summary>
              <pre>{result.manifest}</pre>
            </details>
          ) : null}
        </AlertDescription>
      </Alert>
    </div>
  );
}
