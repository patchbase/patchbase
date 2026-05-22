CREATE TABLE product_streams(
  id TEXT PRIMARY KEY,
  vendor TEXT NOT NULL,
  distro_family TEXT NOT NULL,
  distro_name TEXT NOT NULL,
  major_version INTEGER NOT NULL,
  minor_version TEXT,
  architecture TEXT,
  repo_family TEXT NOT NULL,
  repo_id_pattern TEXT,
  cpe TEXT,
  status TEXT NOT NULL DEFAULT 'active'
);
CREATE TABLE advisories(
  id TEXT PRIMARY KEY,
  source_system TEXT NOT NULL,
  raw_source_id TEXT NOT NULL,
  source_url TEXT,
  vendor TEXT NOT NULL,
  advisory_type TEXT NOT NULL,
  severity TEXT,
  summary TEXT,
  description TEXT,
  published_at TEXT,
  updated_at TEXT,
  evidence_tier TEXT NOT NULL,
  is_security INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE advisory_source_sync_state(
  source TEXT PRIMARY KEY,
  synced_at TEXT NOT NULL,
  source_cursor_at TEXT
);
CREATE TABLE advisory_references(
  id TEXT PRIMARY KEY,
  advisory_id TEXT NOT NULL,
  ref_type TEXT NOT NULL,
  ref_value TEXT NOT NULL,
  severity_vendor TEXT,
  severity_cvss REAL,
  title TEXT,
  url TEXT,
  FOREIGN KEY(advisory_id) REFERENCES advisories(id)
);
CREATE TABLE advisory_product_streams(
  advisory_id TEXT NOT NULL,
  product_stream_id TEXT NOT NULL,
  PRIMARY KEY(advisory_id, product_stream_id),
  FOREIGN KEY(advisory_id) REFERENCES advisories(id),
  FOREIGN KEY(product_stream_id) REFERENCES product_streams(id)
);
CREATE TABLE affected_package_rules(
  id TEXT PRIMARY KEY,
  advisory_id TEXT NOT NULL,
  product_stream_id TEXT NOT NULL,
  package_name TEXT NOT NULL,
  source_rpm TEXT,
  arch TEXT,
  epoch_constraint TEXT,
  version_constraint TEXT,
  release_constraint TEXT,
  rpm_evr_rule TEXT,
  context TEXT NOT NULL DEFAULT 'installed_package',
  evidence_tier TEXT NOT NULL,
  FOREIGN KEY(advisory_id) REFERENCES advisories(id),
  FOREIGN KEY(product_stream_id) REFERENCES product_streams(id)
);
CREATE TABLE fixed_packages(
  id TEXT PRIMARY KEY,
  advisory_id TEXT NOT NULL,
  product_stream_id TEXT NOT NULL,
  package_name TEXT NOT NULL,
  epoch INTEGER NOT NULL DEFAULT 0,
  version TEXT NOT NULL,
  release TEXT NOT NULL,
  arch TEXT,
  nevra TEXT NOT NULL,
  source_rpm TEXT,
  repo_family TEXT,
  evidence_tier TEXT NOT NULL,
  FOREIGN KEY(advisory_id) REFERENCES advisories(id),
  FOREIGN KEY(product_stream_id) REFERENCES product_streams(id)
);
CREATE UNIQUE INDEX idx_advisories_source_unique
ON advisories(
  source_system,
  raw_source_id
);
CREATE INDEX idx_affected_package_rules_advisory_stream_package
ON affected_package_rules(
  advisory_id,
  product_stream_id,
  package_name
);
CREATE INDEX idx_fixed_packages_advisory_stream_package
ON fixed_packages(
  advisory_id,
  product_stream_id,
  package_name
);
CREATE INDEX idx_product_streams_vendor_major
ON product_streams(
  vendor,
  major_version
);
CREATE INDEX idx_advisory_product_streams_product_stream_id
ON advisory_product_streams(
  product_stream_id
);
CREATE INDEX idx_fixed_packages_product_stream_id
ON fixed_packages(
  product_stream_id
);
