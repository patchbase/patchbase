import { spawnSync } from "node:child_process";
import { existsSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dashboardRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
process.chdir(dashboardRoot);

if (!existsSync("svelte.config.js")) {
  console.error(
    "dashboard_build_tool is an internal Bazel action tool. Use //dashboard:dashboard or //dashboard:build_dashboard_assets instead.",
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

function findPackageScript(prefix, packageScriptPath) {
  const base = "node_modules/.aspect_rules_js";
  if (existsSync(base)) {
    for (const entry of readdirSync(base)) {
      if (!entry.startsWith(prefix)) {
        continue;
      }
      const candidate = path.join(base, entry, packageScriptPath);
      if (existsSync(candidate)) {
        return candidate;
      }
    }
  }

  const direct = path.join("node_modules", packageScriptPath);
  if (existsSync(direct)) {
    return direct;
  }

  throw new Error(`unable to resolve script for ${prefix}${packageScriptPath}`);
}

const svelteKitScript = findPackageScript(
  "@sveltejs+kit@",
  "node_modules/@sveltejs/kit/svelte-kit.js",
);
const viteScript = findPackageScript("vite@", "node_modules/vite/bin/vite.js");

runNodeScript(svelteKitScript, ["sync"]);
if (!existsSync(".svelte-kit/tsconfig.json")) {
  throw new Error("svelte-kit sync did not generate .svelte-kit/tsconfig.json");
}
runNodeScript(viteScript, ["build"]);
