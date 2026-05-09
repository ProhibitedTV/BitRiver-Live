import assert from "node:assert/strict";
import test from "node:test";

import {
    applyInstallExperience,
    buildInstallerPreflightPayload,
    buildReviewSections,
    buildSystemChecks,
    computeInstallerScript,
    createInstallerExecution,
    createInstallerState,
    deriveSuccessSummary,
    fetchInstallerPreflight,
    normalizeInstallerPreflightResponse,
    normalizeInstallerDraft,
    setupInstallerWizard,
    validateInstallerDraft,
} from "./installer.js";

function createInstallerDomHarness({ fetchImpl } = {}) {
    const listeners = new Map();
    const container = {
        innerHTML: "",
        addEventListener(type, handler) {
            listeners.set(type, handler);
        },
    };

    const previousDocument = globalThis.document;
    const previousFetch = globalThis.fetch;
    globalThis.document = {
        getElementById(id) {
            return id === "installer" ? container : null;
        },
    };
    globalThis.fetch = fetchImpl;

    return {
        container,
        async click(action) {
            const handler = listeners.get("click");
            await handler?.({
                target: {
                    closest() {
                        return { dataset: { action } };
                    },
                },
            });
        },
        restore() {
            globalThis.document = previousDocument;
            globalThis.fetch = previousFetch;
        },
    };
}

test("installer quick defaults are beginner-friendly and secure", () => {
    const state = createInstallerState();
    const draft = normalizeInstallerDraft(state.draft);

    assert.equal(draft.installExperience, "quick");
    assert.equal(draft.storageDriver, "json");
    assert.equal(draft.sessionStore, "memory");
    assert.equal(draft.addr, ":8080");
    assert.equal(draft.enableLogs, true);
    assert.equal(draft.logDir, "/var/lib/bitriver-live/logs");
    assert.match(draft.adminPassword, /[A-Z]/);
    assert.match(draft.adminPassword, /[a-z]/);
    assert.match(draft.adminPassword, /\d/);
    assert.ok(draft.adminPassword.length >= 16);
});

test("switching back to quick install clears advanced-only storage and tls fields", () => {
    const next = applyInstallExperience(
        {
            installExperience: "advanced",
            storageDriver: "postgres",
            postgresDsn: "postgres://user:strongpass@localhost:5432/bitriver?sslmode=disable",
            sessionStore: "postgres",
            tlsCert: "/etc/ssl/certs/live.pem",
            tlsKey: "/etc/ssl/private/live.key",
            redisAddr: "redis:6379",
            redisPassword: "RedisPassword1234",
        },
        "quick",
    );

    assert.equal(next.installExperience, "quick");
    assert.equal(next.storageDriver, "json");
    assert.equal(next.postgresDsn, "");
    assert.equal(next.sessionStore, "memory");
    assert.equal(next.tlsCert, "");
    assert.equal(next.tlsKey, "");
    assert.equal(next.redisAddr, "");
    assert.equal(next.redisPassword, "");
});

test("advanced validation blocks missing postgres configuration and incomplete tls pairs", () => {
    const validation = validateInstallerDraft({
        installExperience: "advanced",
        adminEmail: "admin@example.com",
        adminPassword: "StrongPassword1234",
        storageDriver: "postgres",
        sessionStore: "postgres",
        postgresDsn: "",
        sessionStoreDsn: "",
        tlsCert: "/etc/ssl/certs/live.pem",
        tlsKey: "",
    });

    assert.equal(validation.hasErrors, true);
    assert.equal(validation.fieldErrors.postgresDsn, "Postgres needs a real DSN before the installer can run.");
    assert.equal(validation.fieldErrors.sessionStoreDsn, "Postgres sessions need a DSN or they must reuse the main Postgres connection.");
    assert.equal(validation.fieldErrors.tlsCert, "Add both the TLS certificate and key, or leave both blank.");
    assert.equal(validation.fieldErrors.tlsKey, "Add both the TLS certificate and key, or leave both blank.");
});

test("system checks surface warning and fail states with actionable messaging", () => {
    const quickChecks = buildSystemChecks(createInstallerState().draft);
    assert.equal(quickChecks[0].status, "pass");
    assert.equal(quickChecks[1].status, "warning");
    assert.equal(quickChecks[2].status, "pass");
    assert.equal(quickChecks[3].status, "pass");

    const advancedChecks = buildSystemChecks({
        installExperience: "advanced",
        storageDriver: "postgres",
        postgresDsn: "",
        addr: ":80",
    });

    assert.equal(advancedChecks[2].status, "warning");
    assert.equal(advancedChecks[3].status, "fail");
});

test("preflight payload keeps only the host-facing installer fields", () => {
    const payload = buildInstallerPreflightPayload({
        installExperience: "advanced",
        adminEmail: "admin@example.com",
        adminPassword: "StrongPassword1234",
        storageDriver: "postgres",
        sessionStore: "postgres",
        postgresDsn: "postgres://user:strongpass@db.example.com:5432/bitriver?sslmode=disable",
        redisAddr: "redis.example.com:6379",
    });

    assert.deepEqual(payload, {
        installDir: "/opt/bitriver-live",
        dataDir: "/var/lib/bitriver-live",
        serviceUser: "bitriver",
        addr: ":8080",
        tlsCert: "",
        tlsKey: "",
        storageDriver: "postgres",
        postgresDsn: "postgres://user:strongpass@db.example.com:5432/bitriver?sslmode=disable",
        sessionStore: "postgres",
        sessionStoreDsn: "",
        redisAddr: "redis.example.com:6379",
    });
});

test("preflight response normalization keeps actionable technical details", () => {
    const normalized = normalizeInstallerPreflightResponse({
        status: "warning",
        checkedAt: "2026-03-28T12:00:00Z",
        checks: [
            {
                id: "port-readiness",
                title: "Port readiness",
                status: "warning",
                summary: "Port 80 needs privileged-port support.",
                action: "Switch to :8080 for the simplest first run.",
                technicalDetails: ["addr=:80", "setcap missing"],
            },
        ],
    });

    assert.equal(normalized.status, "warning");
    assert.equal(normalized.checkedAt, "2026-03-28T12:00:00Z");
    assert.deepEqual(normalized.checks[0].technicalDetails, ["addr=:80", "setcap missing"]);
});

test("fetchInstallerPreflight posts the normalized draft and returns live checks", async () => {
    const requests = [];
    const result = await fetchInstallerPreflight(
        {
            storageDriver: "postgres",
            postgresDsn: "postgres://user:strongpass@db.example.com:5432/bitriver?sslmode=disable",
        },
        async (url, options) => {
            requests.push({ url, options });
            return {
                ok: true,
                json: async () => ({
                    status: "pass",
                    checkedAt: "2026-03-28T12:00:00Z",
                    checks: [{ id: "supported-target", title: "Supported target", status: "pass", summary: "Ubuntu detected" }],
                }),
            };
        },
    );

    assert.equal(requests[0].url, "/api/install/preflight");
    assert.equal(requests[0].options.method, "POST");
    assert.equal(requests[0].options.credentials, "include");
    assert.match(requests[0].options.body, /"storageDriver":"postgres"/);
    assert.equal(result.checks[0].title, "Supported target");
});

test("fetchInstallerPreflight surfaces API errors so the wizard can fall back cleanly", async () => {
    await assert.rejects(
        fetchInstallerPreflight(
            {},
            async () => ({
                ok: false,
                statusText: "Bad Request",
                json: async () => ({ error: { message: "preflight unavailable" } }),
            }),
        ),
        /preflight unavailable/,
    );
});

test("setupInstallerWizard advances into System Check and swaps in live preflight results", async () => {
    const harness = createInstallerDomHarness({
        fetchImpl: async () => ({
            ok: true,
            json: async () => ({
                status: "pass",
                checkedAt: "2026-03-28T12:00:00Z",
                checks: [{ id: "supported-target", title: "Supported target", status: "pass", summary: "Ubuntu detected on this host." }],
            }),
        }),
    });

    try {
        setupInstallerWizard({ showToast: () => {} });
        assert.match(harness.container.innerHTML, /Run system check/);

        await harness.click("next");
        await new Promise((resolve) => setTimeout(resolve, 0));

        assert.match(harness.container.innerHTML, /Refresh checks/);
        assert.match(harness.container.innerHTML, /Ubuntu detected on this host\./);
    } finally {
        harness.restore();
    }
});

test("setupInstallerWizard keeps the flow usable when live preflight falls back", async () => {
    const harness = createInstallerDomHarness({
        fetchImpl: async () => ({
            ok: false,
            statusText: "Service Unavailable",
            json: async () => ({ error: { message: "preflight unavailable" } }),
        }),
    });

    try {
        setupInstallerWizard({ showToast: () => {} });
        await harness.click("next");
        await new Promise((resolve) => setTimeout(resolve, 0));

        assert.match(harness.container.innerHTML, /fell back to safe default guidance/);
        assert.match(harness.container.innerHTML, /This browser cannot inspect the target machine directly\./);
    } finally {
        harness.restore();
    }
});

test("script generation and success summary preserve the installer contract", () => {
    const draft = {
        installExperience: "advanced",
        adminEmail: "admin@example.com",
        adminPassword: "StrongPassword1234",
        hostname: "stream.example.com",
        storageDriver: "postgres",
        postgresDsn: "postgres://user:strongpass@localhost:5432/bitriver?sslmode=disable",
        sessionStore: "postgres",
        enableLogs: true,
    };

    const script = computeInstallerScript(draft);
    assert.match(script, /--storage-driver "\$STORAGE_DRIVER"/);
    assert.match(script, /--postgres-dsn "\$POSTGRES_DSN"/);
    assert.match(script, /--bootstrap-admin-email "\$ADMIN_EMAIL"/);
    assert.match(script, /ADMIN_PASSWORD="StrongPassword1234"/);

    const success = deriveSuccessSummary(draft);
    assert.equal(success.appUrl, "http://stream.example.com:8080");
    assert.equal(success.adminUrl, "http://stream.example.com:8080/admin");
    assert.equal(success.configPath, "/opt/bitriver-live/.env");
    assert.equal(success.dataPath, "/var/lib/bitriver-live");

    const execution = createInstallerExecution(draft);
    assert.equal(execution.status, "ready");
    assert.ok(execution.technicalLog.includes("Generated install command:"));
    assert.equal(execution.success.adminUrl, success.adminUrl);
});

test("review sections summarize the updated handoff information", () => {
    const sections = buildReviewSections({
        adminEmail: "admin@example.com",
        adminPassword: "StrongPassword1234",
        hostname: "stream.example.com",
    });

    assert.ok(sections.length >= 3);
    assert.equal(sections[0].title, "Install plan");
    assert.equal(sections[1].title, "Sign-in and access");
    assert.ok(sections[1].rows.some((row) => row.label === "Admin URL" && row.value === "http://stream.example.com:8080/admin"));
});
