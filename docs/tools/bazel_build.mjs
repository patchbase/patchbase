import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const docsRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
process.chdir(docsRoot);

if (!existsSync("docusaurus.config.ts")) {
  console.error(
    "docs_build_tool is an internal Bazel action tool. Use //docs:build instead.",
  );
  process.exit(2);
}

function runNodeScript(script, args) {
  const result = spawnSync(process.execPath, [script, ...args], {
    stdio: "inherit",
  });

  if (result.error) {
    throw result.error;
  }

  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

const binCandidates = [
  "node_modules/.bin/docusaurus",
  "node_modules/@docusaurus/core/bin/docusaurus.mjs",
];

let docusaurusScript = null;
for (const candidate of binCandidates) {
  if (existsSync(candidate)) {
    docusaurusScript = path.resolve(candidate);
    break;
  }
}

if (!docusaurusScript) {
  throw new Error("unable to find docusaurus binary in node_modules");
}

runNodeScript(docusaurusScript, ["build"]);