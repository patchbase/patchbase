interface HealthResponse {
  version?: string;
}

export async function getVersion(): Promise<string> {
  const response = await fetch("/api/v1/health");
  if (!response.ok) {
    throw new Error(`version request failed: ${response.status}`);
  }

  const data = (await response.json()) as HealthResponse;
  if (!data.version || data.version.trim() == "") {
    return "unknown";
  }

  return data.version;
}
