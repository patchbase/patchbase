-- +goose Up

CREATE TABLE product_streams (
    id text PRIMARY KEY,
    vendor text NOT NULL,
    distro_family text NOT NULL,
    distro_name text NOT NULL,
    major_version integer NOT NULL,
    minor_version text,
    architecture text,
    repo_family text NOT NULL,
    repo_id_pattern text,
    cpe text,
    status text NOT NULL DEFAULT 'active'
);

CREATE TABLE advisories (
    id text PRIMARY KEY,
    source_system text NOT NULL,
    raw_source_id text NOT NULL,
    source_url text,
    vendor text NOT NULL,
    advisory_type text NOT NULL,
    severity text,
    summary text,
    description text,
    published_at text,
    updated_at text,
    evidence_tier text NOT NULL,
    is_security boolean NOT NULL DEFAULT true
);

CREATE TABLE advisory_references (
    id text PRIMARY KEY,
    advisory_id text NOT NULL REFERENCES advisories(id) ON DELETE CASCADE,
    ref_type text NOT NULL,
    ref_value text NOT NULL,
    severity_vendor text,
    severity_cvss double precision,
    title text,
    url text
);

CREATE TABLE advisory_product_streams (
    advisory_id text NOT NULL REFERENCES advisories(id) ON DELETE CASCADE,
    product_stream_id text NOT NULL REFERENCES product_streams(id) ON DELETE CASCADE,
    PRIMARY KEY (advisory_id, product_stream_id)
);

CREATE TABLE affected_package_rules (
    id text PRIMARY KEY,
    advisory_id text NOT NULL REFERENCES advisories(id) ON DELETE CASCADE,
    product_stream_id text NOT NULL REFERENCES product_streams(id) ON DELETE CASCADE,
    package_name text NOT NULL,
    source_rpm text,
    arch text,
    epoch_constraint text,
    version_constraint text,
    release_constraint text,
    rpm_evr_rule text,
    context text NOT NULL DEFAULT 'installed_package',
    evidence_tier text NOT NULL
);

CREATE TABLE fixed_packages (
    id text PRIMARY KEY,
    advisory_id text NOT NULL REFERENCES advisories(id) ON DELETE CASCADE,
    product_stream_id text NOT NULL REFERENCES product_streams(id) ON DELETE CASCADE,
    package_name text NOT NULL,
    epoch integer NOT NULL DEFAULT 0,
    version text NOT NULL,
    release text NOT NULL,
    arch text,
    nevra text NOT NULL,
    source_rpm text,
    repo_family text,
    evidence_tier text NOT NULL
);

CREATE TABLE decision_records (
    id text PRIMARY KEY,
    host_id text NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    snapshot_id text NOT NULL REFERENCES host_snapshots(id) ON DELETE CASCADE,
    advisory_id text NOT NULL REFERENCES advisories(id) ON DELETE CASCADE,
    installed_package_id text,
    product_stream_id text REFERENCES product_streams(id) ON DELETE SET NULL,
    package_name text NOT NULL,
    installed_nevra text,
    fixed_nevra text,
    status text NOT NULL,
    action text NOT NULL,
    severity text,
    evidence_tier text NOT NULL,
    reason_code text NOT NULL,
    reason_text text,
    computed_at text NOT NULL
);

CREATE INDEX decision_records_host_id_idx ON decision_records (host_id);
CREATE INDEX decision_records_snapshot_id_idx ON decision_records (snapshot_id);
CREATE INDEX decision_records_advisory_id_idx ON decision_records (advisory_id);

-- +goose Down
DROP TABLE IF EXISTS decision_records;
DROP TABLE IF EXISTS fixed_packages;
DROP TABLE IF EXISTS affected_package_rules;
DROP TABLE IF EXISTS advisory_product_streams;
DROP TABLE IF EXISTS advisory_references;
DROP TABLE IF EXISTS advisories;
DROP TABLE IF EXISTS product_streams;
