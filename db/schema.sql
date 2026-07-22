--
-- PostgreSQL database dump
--



SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: citext; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;


--
-- Name: EXTENSION citext; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION citext IS 'data type for case-insensitive character strings';


--
-- Name: river_job_state; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.river_job_state AS ENUM (
    'available',
    'cancelled',
    'completed',
    'discarded',
    'pending',
    'retryable',
    'running',
    'scheduled'
);


--
-- Name: river_job_state_in_bitmask(bit, public.river_job_state); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.river_job_state_in_bitmask(bitmask bit, state public.river_job_state) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT CASE state
        WHEN 'available' THEN get_bit(bitmask, 7)
        WHEN 'cancelled' THEN get_bit(bitmask, 6)
        WHEN 'completed' THEN get_bit(bitmask, 5)
        WHEN 'discarded' THEN get_bit(bitmask, 4)
        WHEN 'pending'   THEN get_bit(bitmask, 3)
        WHEN 'retryable' THEN get_bit(bitmask, 2)
        WHEN 'running'   THEN get_bit(bitmask, 1)
        WHEN 'scheduled' THEN get_bit(bitmask, 0)
        ELSE 0
    END = 1;
$$;


--
-- Name: set_updated_at(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.set_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: advisories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.advisories (
    id text NOT NULL,
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
    is_security boolean DEFAULT true NOT NULL
);


--
-- Name: advisory_product_streams; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.advisory_product_streams (
    advisory_id text NOT NULL,
    product_stream_id text NOT NULL
);


--
-- Name: advisory_references; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.advisory_references (
    id text NOT NULL,
    advisory_id text NOT NULL,
    ref_type text NOT NULL,
    ref_value text NOT NULL,
    severity_vendor text,
    severity_cvss double precision,
    title text,
    url text
);


--
-- Name: advisory_scopes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.advisory_scopes (
    scope_key text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    last_sync_at timestamp with time zone,
    last_success_at timestamp with time zone,
    last_error text,
    advisory_count integer DEFAULT 0 NOT NULL,
    sha256 text,
    size_bytes bigint DEFAULT 0 NOT NULL,
    local_path text,
    next_refresh_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: affected_package_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.affected_package_rules (
    id text NOT NULL,
    advisory_id text NOT NULL,
    product_stream_id text NOT NULL,
    package_name text NOT NULL,
    source_rpm text,
    arch text,
    epoch_constraint text,
    version_constraint text,
    release_constraint text,
    rpm_evr_rule text,
    context text DEFAULT 'installed_package'::text NOT NULL,
    evidence_tier text NOT NULL
);


--
-- Name: audit_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audit_log (
    id text NOT NULL,
    actor_id text,
    actor_email text NOT NULL,
    action text NOT NULL,
    target_type text NOT NULL,
    target_id text,
    metadata jsonb,
    ip_address text,
    user_agent text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT audit_log_id_prefix_check CHECK ((id ~~ like_escape('audit\_%'::text, '\'::text)))
);


--
-- Name: decision_records; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.decision_records (
    id text NOT NULL,
    host_id text NOT NULL,
    snapshot_id text NOT NULL,
    advisory_id text NOT NULL,
    installed_package_id text,
    product_stream_id text,
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


--
-- Name: fixed_packages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fixed_packages (
    id text NOT NULL,
    advisory_id text NOT NULL,
    product_stream_id text NOT NULL,
    package_name text NOT NULL,
    epoch integer DEFAULT 0 NOT NULL,
    version text NOT NULL,
    release text NOT NULL,
    arch text,
    nevra text NOT NULL,
    source_rpm text,
    repo_family text,
    evidence_tier text NOT NULL
);


--
-- Name: goose_db_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.goose_db_version (
    id integer NOT NULL,
    version_id bigint NOT NULL,
    is_applied boolean NOT NULL,
    tstamp timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: goose_db_version_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.goose_db_version ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.goose_db_version_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: host_access_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.host_access_tokens (
    id text NOT NULL,
    host_id text NOT NULL,
    token_hash text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone,
    last_used_at timestamp with time zone,
    CONSTRAINT host_access_tokens_id_prefix_check CHECK ((id ~~ like_escape('htok\_%'::text, '\'::text)))
);


--
-- Name: host_current_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.host_current_state (
    host_id text NOT NULL,
    snapshot_id text NOT NULL,
    overall_action text DEFAULT 'none'::text NOT NULL,
    critical_count integer DEFAULT 0 NOT NULL,
    important_count integer DEFAULT 0 NOT NULL,
    moderate_count integer DEFAULT 0 NOT NULL,
    actionable_count integer DEFAULT 0 NOT NULL,
    available_updates integer DEFAULT 0 NOT NULL,
    needs_reboot integer DEFAULT 0 NOT NULL,
    needs_restart integer DEFAULT 0 NOT NULL,
    no_fix integer DEFAULT 0 NOT NULL,
    unknown integer DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: host_snapshots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.host_snapshots (
    id text NOT NULL,
    host_id text NOT NULL,
    collected_at timestamp with time zone NOT NULL,
    received_at timestamp with time zone DEFAULT now() NOT NULL,
    payload bytea NOT NULL,
    running_kernel_nevra text DEFAULT ''::text NOT NULL,
    boot_time timestamp with time zone,
    has_process_data boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT host_snapshots_id_prefix_check CHECK ((id ~~ like_escape('snap\_%'::text, '\'::text)))
);


--
-- Name: host_ssh_pull; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.host_ssh_pull (
    host_id text NOT NULL,
    pull_ssh_user text,
    pull_frequency_minutes integer,
    pull_public_key text,
    pull_private_key text,
    pull_last_run_at timestamp with time zone,
    pull_last_run_status text,
    pull_last_run_error text,
    onboarded boolean DEFAULT false NOT NULL,
    pull_hostname text NOT NULL
);


--
-- Name: host_ssh_pull_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.host_ssh_pull_jobs (
    id text NOT NULL,
    host_id text NOT NULL,
    status text NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    error text
);


--
-- Name: hosts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.hosts (
    id text NOT NULL,
    onboarding_mode text NOT NULL,
    approval_status text DEFAULT 'waiting_approval'::text NOT NULL,
    approved_at timestamp with time zone,
    display_name text,
    machine_id text,
    hostname text,
    ip_address text,
    os_family text DEFAULT 'unknown'::text NOT NULL,
    os_name text DEFAULT 'Unknown'::text NOT NULL,
    os_major integer DEFAULT 0 NOT NULL,
    os_version text DEFAULT 'unknown'::text NOT NULL,
    architecture text DEFAULT 'unknown'::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    last_seen_at timestamp with time zone,
    last_advisory_check_at timestamp with time zone,
    first_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    last_snapshot_id text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    advisory_scope_key text,
    CONSTRAINT hosts_approval_status_check CHECK ((approval_status = ANY (ARRAY['waiting_approval'::text, 'approved'::text, 'rejected'::text]))),
    CONSTRAINT hosts_id_prefix_check CHECK ((id ~~ like_escape('h\_%'::text, '\'::text))),
    CONSTRAINT hosts_onboarding_mode_check CHECK ((onboarding_mode = ANY (ARRAY['agent'::text, 'ssh'::text, 'manual'::text])))
);


--
-- Name: product_streams; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.product_streams (
    id text NOT NULL,
    vendor text NOT NULL,
    distro_family text NOT NULL,
    distro_name text NOT NULL,
    major_version integer NOT NULL,
    minor_version text,
    architecture text,
    repo_family text NOT NULL,
    repo_id_pattern text,
    cpe text,
    status text DEFAULT 'active'::text NOT NULL
);


--
-- Name: registration_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.registration_tokens (
    id text NOT NULL,
    name text NOT NULL,
    token_hash text NOT NULL,
    created_by_user_id text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone,
    last_used_at timestamp with time zone,
    CONSTRAINT registration_tokens_id_prefix_check CHECK ((id ~~ like_escape('rtok\_%'::text, '\'::text)))
);


--
-- Name: river_client; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE public.river_client (
    id text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    paused_at timestamp with time zone,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT name_length CHECK (((char_length(id) > 0) AND (char_length(id) < 128)))
);


--
-- Name: river_client_queue; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE public.river_client_queue (
    river_client_id text NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    max_workers bigint DEFAULT 0 NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    num_jobs_completed bigint DEFAULT 0 NOT NULL,
    num_jobs_running bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT name_length CHECK (((char_length(name) > 0) AND (char_length(name) < 128))),
    CONSTRAINT num_jobs_completed_zero_or_positive CHECK ((num_jobs_completed >= 0)),
    CONSTRAINT num_jobs_running_zero_or_positive CHECK ((num_jobs_running >= 0))
);


--
-- Name: river_job; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_job (
    id bigint NOT NULL,
    state public.river_job_state DEFAULT 'available'::public.river_job_state NOT NULL,
    attempt smallint DEFAULT 0 NOT NULL,
    max_attempts smallint NOT NULL,
    attempted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    finalized_at timestamp with time zone,
    scheduled_at timestamp with time zone DEFAULT now() NOT NULL,
    priority smallint DEFAULT 1 NOT NULL,
    args jsonb NOT NULL,
    attempted_by text[],
    errors jsonb[],
    kind text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    queue text DEFAULT 'default'::text NOT NULL,
    tags character varying(255)[] DEFAULT '{}'::character varying[] NOT NULL,
    unique_key bytea,
    unique_states bit(8),
    CONSTRAINT finalized_or_finalized_at_null CHECK ((((finalized_at IS NULL) AND (state <> ALL (ARRAY['cancelled'::public.river_job_state, 'completed'::public.river_job_state, 'discarded'::public.river_job_state]))) OR ((finalized_at IS NOT NULL) AND (state = ANY (ARRAY['cancelled'::public.river_job_state, 'completed'::public.river_job_state, 'discarded'::public.river_job_state]))))),
    CONSTRAINT kind_length CHECK (((char_length(kind) > 0) AND (char_length(kind) < 128))),
    CONSTRAINT max_attempts_is_positive CHECK ((max_attempts > 0)),
    CONSTRAINT priority_in_range CHECK (((priority >= 1) AND (priority <= 4))),
    CONSTRAINT queue_length CHECK (((char_length(queue) > 0) AND (char_length(queue) < 128)))
);


--
-- Name: river_job_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.river_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: river_job_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.river_job_id_seq OWNED BY public.river_job.id;


--
-- Name: river_leader; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE public.river_leader (
    elected_at timestamp with time zone NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    leader_id text NOT NULL,
    name text DEFAULT 'default'::text NOT NULL,
    CONSTRAINT leader_id_length CHECK (((char_length(leader_id) > 0) AND (char_length(leader_id) < 128))),
    CONSTRAINT name_length CHECK ((name = 'default'::text))
);


--
-- Name: river_migration; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_migration (
    line text NOT NULL,
    version bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT line_length CHECK (((char_length(line) > 0) AND (char_length(line) < 128))),
    CONSTRAINT version_gte_1 CHECK ((version >= 1))
);


--
-- Name: river_queue; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_queue (
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    paused_at timestamp with time zone,
    updated_at timestamp with time zone NOT NULL
);


--
-- Name: settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.settings (
    key public.citext NOT NULL,
    value jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id text NOT NULL,
    email text NOT NULL,
    name text NOT NULL,
    password_hash text NOT NULL,
    is_admin boolean DEFAULT false NOT NULL,
    password_reset_required boolean DEFAULT false NOT NULL,
    last_login_at timestamp with time zone,
    archived_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT users_id_prefix_check CHECK ((id ~~ like_escape('u\_%'::text, '\'::text)))
);


--
-- Name: river_job id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_job ALTER COLUMN id SET DEFAULT nextval('public.river_job_id_seq'::regclass);


--
-- Name: advisories advisories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.advisories
    ADD CONSTRAINT advisories_pkey PRIMARY KEY (id);


--
-- Name: advisory_product_streams advisory_product_streams_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.advisory_product_streams
    ADD CONSTRAINT advisory_product_streams_pkey PRIMARY KEY (advisory_id, product_stream_id);


--
-- Name: advisory_references advisory_references_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.advisory_references
    ADD CONSTRAINT advisory_references_pkey PRIMARY KEY (id);


--
-- Name: advisory_scopes advisory_scopes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.advisory_scopes
    ADD CONSTRAINT advisory_scopes_pkey PRIMARY KEY (scope_key);


--
-- Name: affected_package_rules affected_package_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.affected_package_rules
    ADD CONSTRAINT affected_package_rules_pkey PRIMARY KEY (id);


--
-- Name: audit_log audit_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_log
    ADD CONSTRAINT audit_log_pkey PRIMARY KEY (id);


--
-- Name: decision_records decision_records_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.decision_records
    ADD CONSTRAINT decision_records_pkey PRIMARY KEY (id);


--
-- Name: fixed_packages fixed_packages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fixed_packages
    ADD CONSTRAINT fixed_packages_pkey PRIMARY KEY (id);


--
-- Name: goose_db_version goose_db_version_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.goose_db_version
    ADD CONSTRAINT goose_db_version_pkey PRIMARY KEY (id);


--
-- Name: host_access_tokens host_access_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_access_tokens
    ADD CONSTRAINT host_access_tokens_pkey PRIMARY KEY (id);


--
-- Name: host_current_state host_current_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_current_state
    ADD CONSTRAINT host_current_state_pkey PRIMARY KEY (host_id);


--
-- Name: host_snapshots host_snapshots_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_snapshots
    ADD CONSTRAINT host_snapshots_pkey PRIMARY KEY (id);


--
-- Name: host_ssh_pull_jobs host_ssh_pull_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_ssh_pull_jobs
    ADD CONSTRAINT host_ssh_pull_jobs_pkey PRIMARY KEY (id);


--
-- Name: host_ssh_pull host_ssh_pull_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_ssh_pull
    ADD CONSTRAINT host_ssh_pull_pkey PRIMARY KEY (host_id);


--
-- Name: hosts hosts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hosts
    ADD CONSTRAINT hosts_pkey PRIMARY KEY (id);


--
-- Name: product_streams product_streams_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.product_streams
    ADD CONSTRAINT product_streams_pkey PRIMARY KEY (id);


--
-- Name: registration_tokens registration_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.registration_tokens
    ADD CONSTRAINT registration_tokens_pkey PRIMARY KEY (id);


--
-- Name: river_client river_client_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_client
    ADD CONSTRAINT river_client_pkey PRIMARY KEY (id);


--
-- Name: river_client_queue river_client_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_client_queue
    ADD CONSTRAINT river_client_queue_pkey PRIMARY KEY (river_client_id, name);


--
-- Name: river_job river_job_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_job
    ADD CONSTRAINT river_job_pkey PRIMARY KEY (id);


--
-- Name: river_leader river_leader_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_leader
    ADD CONSTRAINT river_leader_pkey PRIMARY KEY (name);


--
-- Name: river_migration river_migration_pkey1; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_migration
    ADD CONSTRAINT river_migration_pkey1 PRIMARY KEY (line, version);


--
-- Name: river_queue river_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_queue
    ADD CONSTRAINT river_queue_pkey PRIMARY KEY (name);


--
-- Name: settings settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settings
    ADD CONSTRAINT settings_pkey PRIMARY KEY (key);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: audit_log_action_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_log_action_idx ON public.audit_log USING btree (action);


--
-- Name: audit_log_actor_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_log_actor_id_idx ON public.audit_log USING btree (actor_id);


--
-- Name: audit_log_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_log_created_at_idx ON public.audit_log USING btree (created_at DESC);


--
-- Name: audit_log_target_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_log_target_idx ON public.audit_log USING btree (target_type, target_id);


--
-- Name: decision_records_advisory_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX decision_records_advisory_id_idx ON public.decision_records USING btree (advisory_id);


--
-- Name: decision_records_host_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX decision_records_host_id_idx ON public.decision_records USING btree (host_id);


--
-- Name: decision_records_snapshot_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX decision_records_snapshot_id_idx ON public.decision_records USING btree (snapshot_id);


--
-- Name: decision_records_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX decision_records_unique_idx ON public.decision_records USING btree (snapshot_id, advisory_id, package_name, COALESCE(installed_nevra, ''::text));


--
-- Name: host_access_tokens_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX host_access_tokens_active_idx ON public.host_access_tokens USING btree (host_id, created_at DESC) WHERE (revoked_at IS NULL);


--
-- Name: host_access_tokens_host_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX host_access_tokens_host_idx ON public.host_access_tokens USING btree (host_id);


--
-- Name: host_access_tokens_token_hash_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX host_access_tokens_token_hash_unique_idx ON public.host_access_tokens USING btree (token_hash);


--
-- Name: host_snapshots_host_collected_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX host_snapshots_host_collected_idx ON public.host_snapshots USING btree (host_id, collected_at DESC);


--
-- Name: host_ssh_pull_pull_hostname_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX host_ssh_pull_pull_hostname_unique_idx ON public.host_ssh_pull USING btree (pull_hostname);


--
-- Name: hosts_advisory_scope_key_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX hosts_advisory_scope_key_idx ON public.hosts USING btree (advisory_scope_key);


--
-- Name: hosts_approval_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX hosts_approval_status_idx ON public.hosts USING btree (approval_status);


--
-- Name: hosts_display_name_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX hosts_display_name_unique_idx ON public.hosts USING btree (display_name);


--
-- Name: hosts_hostname_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX hosts_hostname_idx ON public.hosts USING btree (hostname);


--
-- Name: idx_advisory_references_advisory_id_ref_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_advisory_references_advisory_id_ref_type ON public.advisory_references USING btree (advisory_id, ref_type);


--
-- Name: registration_tokens_active_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX registration_tokens_active_idx ON public.registration_tokens USING btree (created_at DESC) WHERE (revoked_at IS NULL);


--
-- Name: registration_tokens_token_hash_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX registration_tokens_token_hash_unique_idx ON public.registration_tokens USING btree (token_hash);


--
-- Name: river_job_args_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_args_index ON public.river_job USING gin (args);


--
-- Name: river_job_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_kind ON public.river_job USING btree (kind);


--
-- Name: river_job_metadata_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_metadata_index ON public.river_job USING gin (metadata);


--
-- Name: river_job_prioritized_fetching_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_prioritized_fetching_index ON public.river_job USING btree (state, queue, priority, scheduled_at, id);


--
-- Name: river_job_state_and_finalized_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_state_and_finalized_at_index ON public.river_job USING btree (state, finalized_at) WHERE (finalized_at IS NOT NULL);


--
-- Name: river_job_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX river_job_unique_idx ON public.river_job USING btree (unique_key) WHERE ((unique_key IS NOT NULL) AND (unique_states IS NOT NULL) AND public.river_job_state_in_bitmask(unique_states, state));


--
-- Name: users_email_active_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX users_email_active_unique_idx ON public.users USING btree (email) WHERE (archived_at IS NULL);


--
-- Name: advisory_scopes advisory_scopes_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER advisory_scopes_set_updated_at BEFORE UPDATE ON public.advisory_scopes FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: hosts hosts_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER hosts_set_updated_at BEFORE UPDATE ON public.hosts FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: settings settings_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER settings_set_updated_at BEFORE UPDATE ON public.settings FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: users users_set_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: advisory_product_streams advisory_product_streams_advisory_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.advisory_product_streams
    ADD CONSTRAINT advisory_product_streams_advisory_id_fkey FOREIGN KEY (advisory_id) REFERENCES public.advisories(id) ON DELETE CASCADE;


--
-- Name: advisory_product_streams advisory_product_streams_product_stream_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.advisory_product_streams
    ADD CONSTRAINT advisory_product_streams_product_stream_id_fkey FOREIGN KEY (product_stream_id) REFERENCES public.product_streams(id) ON DELETE CASCADE;


--
-- Name: advisory_references advisory_references_advisory_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.advisory_references
    ADD CONSTRAINT advisory_references_advisory_id_fkey FOREIGN KEY (advisory_id) REFERENCES public.advisories(id) ON DELETE CASCADE;


--
-- Name: affected_package_rules affected_package_rules_advisory_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.affected_package_rules
    ADD CONSTRAINT affected_package_rules_advisory_id_fkey FOREIGN KEY (advisory_id) REFERENCES public.advisories(id) ON DELETE CASCADE;


--
-- Name: affected_package_rules affected_package_rules_product_stream_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.affected_package_rules
    ADD CONSTRAINT affected_package_rules_product_stream_id_fkey FOREIGN KEY (product_stream_id) REFERENCES public.product_streams(id) ON DELETE CASCADE;


--
-- Name: decision_records decision_records_advisory_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.decision_records
    ADD CONSTRAINT decision_records_advisory_id_fkey FOREIGN KEY (advisory_id) REFERENCES public.advisories(id) ON DELETE CASCADE;


--
-- Name: decision_records decision_records_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.decision_records
    ADD CONSTRAINT decision_records_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: decision_records decision_records_product_stream_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.decision_records
    ADD CONSTRAINT decision_records_product_stream_id_fkey FOREIGN KEY (product_stream_id) REFERENCES public.product_streams(id) ON DELETE SET NULL;


--
-- Name: decision_records decision_records_snapshot_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.decision_records
    ADD CONSTRAINT decision_records_snapshot_id_fkey FOREIGN KEY (snapshot_id) REFERENCES public.host_snapshots(id) ON DELETE CASCADE;


--
-- Name: fixed_packages fixed_packages_advisory_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fixed_packages
    ADD CONSTRAINT fixed_packages_advisory_id_fkey FOREIGN KEY (advisory_id) REFERENCES public.advisories(id) ON DELETE CASCADE;


--
-- Name: fixed_packages fixed_packages_product_stream_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fixed_packages
    ADD CONSTRAINT fixed_packages_product_stream_id_fkey FOREIGN KEY (product_stream_id) REFERENCES public.product_streams(id) ON DELETE CASCADE;


--
-- Name: host_access_tokens host_access_tokens_host_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_access_tokens
    ADD CONSTRAINT host_access_tokens_host_fk FOREIGN KEY (host_id) REFERENCES public.hosts(id);


--
-- Name: host_current_state host_current_state_host_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_current_state
    ADD CONSTRAINT host_current_state_host_fk FOREIGN KEY (host_id) REFERENCES public.hosts(id);


--
-- Name: host_current_state host_current_state_snapshot_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_current_state
    ADD CONSTRAINT host_current_state_snapshot_fk FOREIGN KEY (snapshot_id) REFERENCES public.host_snapshots(id);


--
-- Name: host_snapshots host_snapshots_host_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_snapshots
    ADD CONSTRAINT host_snapshots_host_fk FOREIGN KEY (host_id) REFERENCES public.hosts(id);


--
-- Name: host_ssh_pull host_ssh_pull_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_ssh_pull
    ADD CONSTRAINT host_ssh_pull_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: host_ssh_pull_jobs host_ssh_pull_jobs_host_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.host_ssh_pull_jobs
    ADD CONSTRAINT host_ssh_pull_jobs_host_id_fkey FOREIGN KEY (host_id) REFERENCES public.hosts(id) ON DELETE CASCADE;


--
-- Name: hosts hosts_advisory_scope_key_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hosts
    ADD CONSTRAINT hosts_advisory_scope_key_fkey FOREIGN KEY (advisory_scope_key) REFERENCES public.advisory_scopes(scope_key) ON DELETE SET NULL;


--
-- Name: hosts hosts_last_snapshot_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.hosts
    ADD CONSTRAINT hosts_last_snapshot_fk FOREIGN KEY (last_snapshot_id) REFERENCES public.host_snapshots(id);


--
-- Name: registration_tokens registration_tokens_created_by_user_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.registration_tokens
    ADD CONSTRAINT registration_tokens_created_by_user_fk FOREIGN KEY (created_by_user_id) REFERENCES public.users(id);


--
-- Name: river_client_queue river_client_queue_river_client_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_client_queue
    ADD CONSTRAINT river_client_queue_river_client_id_fkey FOREIGN KEY (river_client_id) REFERENCES public.river_client(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


