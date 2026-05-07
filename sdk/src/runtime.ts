import { createInterface } from "node:readline";
import { pathToFileURL } from "node:url";
import { readdir } from "node:fs/promises";
import { join, extname } from "node:path";
import type { RuleDefinition, RuleContext, Label } from "./index.js";

interface RPCRequest {
  id: number;
  method: string;
  params: Record<string, unknown>;
}

interface RPCResponse {
  id: number;
  result?: unknown;
  error?: { message: string };
}

const rules: RuleDefinition[] = [];

async function loadRules(rulesDir: string): Promise<void> {
  let entries;
  try {
    entries = await readdir(rulesDir);
  } catch {
    return;
  }

  const ruleFiles = entries.filter(
    (f) => (extname(f) === ".ts" || extname(f) === ".js") && !f.startsWith("_")
  );

  for (const file of ruleFiles.sort()) {
    const fullPath = join(rulesDir, file);
    try {
      const mod = await import(pathToFileURL(fullPath).href);
      const def: RuleDefinition = mod.default ?? mod;
      if (def && typeof def.evaluate === "function" && def.name) {
        rules.push(def);
      } else {
        process.stderr.write(
          `[openbpl] skip ${file}: must export a RuleDefinition with name and evaluate\n`
        );
      }
    } catch (err) {
      process.stderr.write(`[openbpl] error loading ${file}: ${err}\n`);
    }
  }
}

async function handleEvaluate(
  params: Record<string, unknown>
): Promise<Label[]> {
  const ctx = params as unknown as RuleContext;
  const allLabels: Label[] = [];

  for (const rule of rules) {
    try {
      const result = await rule.evaluate(ctx);
      if (result === null || result === undefined) continue;
      const labels = Array.isArray(result) ? result : [result];
      for (const label of labels) {
        allLabels.push({ ...label, name: label.name || rule.name });
      }
    } catch (err) {
      process.stderr.write(`[openbpl] rule ${rule.name} error: ${err}\n`);
    }
  }

  return allLabels;
}

function send(msg: RPCResponse): void {
  process.stdout.write(JSON.stringify(msg) + "\n");
}

async function handleRequest(req: RPCRequest): Promise<void> {
  try {
    switch (req.method) {
      case "evaluate": {
        const labels = await handleEvaluate(req.params);
        send({ id: req.id, result: labels });
        break;
      }
      case "list": {
        const list = rules.map((r) => ({
          name: r.name,
          description: r.description || "",
        }));
        send({ id: req.id, result: list });
        break;
      }
      case "ping": {
        send({ id: req.id, result: "pong" });
        break;
      }
      default:
        send({ id: req.id, error: { message: `unknown method: ${req.method}` } });
    }
  } catch (err) {
    send({ id: req.id, error: { message: String(err) } });
  }
}

async function main(): Promise<void> {
  const rulesDir = process.argv[2] || process.cwd();
  await loadRules(rulesDir);

  // Signal ready to the Go process
  send({ id: 0, result: { ready: true, rules: rules.length } });

  const rl = createInterface({ input: process.stdin });

  for await (const line of rl) {
    if (!line.trim()) continue;
    try {
      const req: RPCRequest = JSON.parse(line);
      await handleRequest(req);
    } catch (err) {
      process.stderr.write(`[openbpl] parse error: ${err}\n`);
    }
  }
}

main().catch((err) => {
  process.stderr.write(`[openbpl] fatal: ${err}\n`);
  process.exit(1);
});
