const INSTALLER_SCRIPT_URL = "https://raw.githubusercontent.com/BitRiver-Live/BitRiver-Live/main/deploy/install/ubuntu.sh";
const INSTALLER_GUIDE_URL = "https://github.com/BitRiver-Live/BitRiver-Live/blob/main/docs/installing-on-ubuntu.md";
const INSTALLER_PREFLIGHT_URL = "/api/install/preflight";

export const INSTALLER_STEPS = [
    {
        id: "welcome",
        label: "Welcome",
        title: "Set up BitRiver Live with a guided install",
        description: "We will keep the first run simple, check the plan up front, and build the exact Ubuntu install command for you.",
    },
    {
        id: "system-check",
        label: "System Check",
        title: "Check the install plan before you configure anything",
        description: "These checks use the recommended quick-install defaults so you can spot common blockers before you fill out the wizard.",
    },
    {
        id: "install-mode",
        label: "Install Mode",
        title: "Choose how much control you want",
        description: "Quick Install is the shortest path. Advanced Install opens storage, network, and service details when you are ready for them.",
    },
    {
        id: "core-settings",
        label: "Core Settings",
        title: "Add the details your server actually needs",
        description: "Every field explains what it does and when you would want to change it.",
    },
    {
        id: "review",
        label: "Review",
        title: "Review your choices before anything runs",
        description: "You can still go back and change any setting. Nothing is executed until you start the handoff step.",
    },
    {
        id: "installing",
        label: "Installing",
        title: "Prepare the install handoff",
        description: "We are validating your selections, building the install command, and packaging the recovery details you will want later.",
    },
    {
        id: "success",
        label: "Success",
        title: "Your install plan is ready",
        description: "Use the links and paths below after the Ubuntu helper finishes on your server.",
    },
];

const DEFAULT_DRAFT = Object.freeze({
    installExperience: "quick",
    adminEmail: "",
    adminPassword: "",
    hostname: "",
    installDir: "/opt/bitriver-live",
    dataDir: "/var/lib/bitriver-live",
    serviceUser: "bitriver",
    runtimeMode: "production",
    addr: ":8080",
    storageDriver: "json",
    postgresDsn: "",
    sessionStore: "memory",
    sessionStoreDsn: "",
    allowSelfSignup: false,
    enableLogs: true,
    logDir: "",
    tlsCert: "",
    tlsKey: "",
    rateGlobalRps: "50",
    rateLoginLimit: "5",
    rateLoginWindow: "2m",
    redisAddr: "",
    redisPassword: "",
});

function createProgressItems() {
    return [
        {
            key: "validate",
            title: "Validate selections",
            detail: "Check the fields and keep any existing answers in place if something needs attention.",
            status: "pending",
        },
        {
            key: "command",
            title: "Build install command",
            detail: "Generate the Ubuntu helper script with the exact flags for this setup.",
            status: "pending",
        },
        {
            key: "handoff",
            title: "Prepare recovery details",
            detail: "Capture the URLs, paths, and next steps you will want after the installer finishes.",
            status: "pending",
        },
        {
            key: "ready",
            title: "Ready for server handoff",
            detail: "Copy the command, run it on your Ubuntu server, then continue to the success screen.",
            status: "pending",
        },
    ];
}

function createPreflightState(draft) {
    return {
        status: "idle",
        source: "fallback",
        checks: buildSystemChecks(draft),
        error: "",
        checkedAt: "",
        lastLoadedSignature: "",
    };
}

function trimString(value) {
    return typeof value === "string" ? value.trim() : "";
}

export function generateStrongPassword(length = 24) {
    const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789";
    const bytes = new Uint8Array(Math.max(length, 16));
    if (globalThis.crypto?.getRandomValues) {
        globalThis.crypto.getRandomValues(bytes);
    } else {
        for (let index = 0; index < bytes.length; index += 1) {
            bytes[index] = Math.floor(Math.random() * 256);
        }
    }

    const characters = [];
    for (let index = 0; index < bytes.length; index += 1) {
        characters.push(alphabet[bytes[index] % alphabet.length]);
    }

    if (!/[A-Z]/.test(characters.join(""))) {
        characters[0] = "A";
    }
    if (!/[a-z]/.test(characters.join(""))) {
        characters[1] = "a";
    }
    if (!/\d/.test(characters.join(""))) {
        characters[2] = "7";
    }

    return characters.join("");
}

function cloneDraft(overrides = {}) {
    const base = {
        ...DEFAULT_DRAFT,
        adminPassword: generateStrongPassword(),
    };
    return {
        ...base,
        ...overrides,
        adminPassword: trimString(overrides.adminPassword) || base.adminPassword,
    };
}

export function createInstallerState(overrides = {}) {
    const draft = cloneDraft(overrides.draft);
    return {
        stepIndex: 0,
        draft,
        fieldErrors: {},
        generalErrors: [],
        preflight: {
            ...createPreflightState(draft),
            ...(overrides.preflight || {}),
        },
        execution: {
            status: "idle",
            progress: createProgressItems(),
            script: "",
            technicalLog: "",
            copied: false,
            error: "",
            success: null,
        },
    };
}

function normalizeUnixPath(value) {
    const trimmed = trimString(value);
    if (!trimmed) {
        return "";
    }
    if (trimmed === "/") {
        return "/";
    }
    return trimmed.replace(/\/+$/, "");
}

function escapeHTML(value) {
    return String(value ?? "")
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#39;");
}

function escapeShellDoubleQuoted(value) {
    return String(value ?? "")
        .replaceAll("\\", "\\\\")
        .replaceAll('"', '\\"')
        .replaceAll("$", "\\$")
        .replaceAll("`", "\\`");
}

function formatBoolean(value, trueText, falseText) {
    return value ? trueText : falseText;
}

function formatStepCount(stepIndex) {
    return `${stepIndex + 1} of ${INSTALLER_STEPS.length}`;
}

function formatCheckedAt(value) {
    const trimmed = trimString(value);
    if (!trimmed) {
        return "";
    }
    const date = new Date(trimmed);
    if (Number.isNaN(date.getTime())) {
        return "";
    }
    return date.toLocaleString();
}

function extractPort(addr) {
    const value = trimString(addr);
    let match = value.match(/^\[.+\]:(\d+)$/);
    if (match) {
        return match[1];
    }
    match = value.match(/^:(\d+)$/);
    if (match) {
        return match[1];
    }
    match = value.match(/^.+:(\d+)$/);
    if (match) {
        return match[1];
    }
    return "";
}

function isAbsoluteUnixPath(value) {
    return trimString(value).startsWith("/");
}

function isValidEmail(value) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimString(value));
}

function isValidHostname(value) {
    const trimmed = trimString(value);
    if (!trimmed) {
        return true;
    }
    if (trimmed.includes("://") || trimmed.includes("/") || /\s/.test(trimmed)) {
        return false;
    }
    return /^[A-Za-z0-9.-]+$/.test(trimmed);
}

function isValidRedisAddress(value) {
    const trimmed = trimString(value);
    if (!trimmed) {
        return true;
    }
    if (/^\[[^\]]+\]:\d+$/.test(trimmed)) {
        return true;
    }
    return /^[^:\s]+:\d+$/.test(trimmed);
}

function isPositiveIntegerString(value) {
    const trimmed = trimString(value);
    return trimmed === "" || /^\d+$/.test(trimmed);
}

function isValidDuration(value) {
    const trimmed = trimString(value);
    return trimmed === "" || /^\d+[smhd]$/i.test(trimmed);
}

function isStrongPassword(value) {
    const trimmed = trimString(value);
    return trimmed.length >= 16 && /[A-Z]/.test(trimmed) && /[a-z]/.test(trimmed) && /\d/.test(trimmed);
}

function containsPlaceholderSecret(value) {
    const trimmed = trimString(value).toLowerCase();
    return trimmed.includes("bitriver:changeme") || trimmed.includes("bitriver:bitriver");
}

function maskSecret(value) {
    const trimmed = trimString(value);
    if (!trimmed) {
        return "Not set";
    }
    if (trimmed.length <= 4) {
        return "****";
    }
    return `${"*".repeat(Math.max(8, trimmed.length - 4))}${trimmed.slice(-4)}`;
}

export function buildInstallerPreflightPayload(draft) {
    const data = normalizeInstallerDraft(draft);
    return {
        installDir: data.installDir,
        dataDir: data.dataDir,
        serviceUser: data.serviceUser,
        addr: data.addr,
        tlsCert: data.tlsCert,
        tlsKey: data.tlsKey,
        storageDriver: data.storageDriver,
        postgresDsn: data.postgresDsn,
        sessionStore: data.sessionStore,
        sessionStoreDsn: data.sessionStoreDsn,
        redisAddr: data.redisAddr,
    };
}

function normalizeSystemCheckStatus(value) {
    switch (trimString(value).toLowerCase()) {
        case "pass":
        case "warning":
        case "fail":
            return trimString(value).toLowerCase();
        default:
            return "warning";
    }
}

export function normalizeInstallerPreflightResponse(payload) {
    const checks = Array.isArray(payload?.checks)
        ? payload.checks.map((check) => ({
              id: trimString(check?.id),
              title: trimString(check?.title) || "System check",
              status: normalizeSystemCheckStatus(check?.status),
              summary: trimString(check?.summary),
              action: trimString(check?.action),
              technicalDetails: Array.isArray(check?.technicalDetails)
                  ? check.technicalDetails.map((item) => trimString(item)).filter(Boolean)
                  : [],
          }))
        : [];

    return {
        status: normalizeSystemCheckStatus(payload?.status),
        checkedAt: trimString(payload?.checkedAt),
        checks,
    };
}

export async function fetchInstallerPreflight(draft, fetchImpl = globalThis.fetch) {
    if (typeof fetchImpl !== "function") {
        throw new Error("Fetch API unavailable");
    }

    const response = await fetchImpl(INSTALLER_PREFLIGHT_URL, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(buildInstallerPreflightPayload(draft)),
    });

    const payload = await response.json().catch(() => null);
    if (!response.ok) {
        throw new Error(trimString(payload?.error?.message) || response.statusText || "Live system check failed.");
    }

    return normalizeInstallerPreflightResponse(payload);
}

export function normalizeInstallerDraft(draft = {}) {
    const merged = {
        ...cloneDraft(),
        ...draft,
    };

    const runtimeMode = merged.runtimeMode === "development" ? "development" : "production";
    const storageDriver = merged.storageDriver === "postgres" ? "postgres" : "json";
    const sessionStore = merged.sessionStore === "postgres" ? "postgres" : "memory";
    const installDir = normalizeUnixPath(merged.installDir) || DEFAULT_DRAFT.installDir;
    const dataDir = normalizeUnixPath(merged.dataDir) || DEFAULT_DRAFT.dataDir;
    const explicitLogDir = normalizeUnixPath(merged.logDir);

    return {
        installExperience: merged.installExperience === "advanced" ? "advanced" : "quick",
        adminEmail: trimString(merged.adminEmail),
        adminPassword: trimString(merged.adminPassword),
        hostname: trimString(merged.hostname),
        installDir,
        dataDir,
        serviceUser: trimString(merged.serviceUser) || DEFAULT_DRAFT.serviceUser,
        runtimeMode,
        addr: trimString(merged.addr) || DEFAULT_DRAFT.addr,
        storageDriver,
        postgresDsn: trimString(merged.postgresDsn),
        sessionStore: storageDriver === "json" ? "memory" : sessionStore,
        sessionStoreDsn: trimString(merged.sessionStoreDsn),
        allowSelfSignup: Boolean(merged.allowSelfSignup),
        enableLogs: Boolean(merged.enableLogs),
        logDir: Boolean(merged.enableLogs) ? explicitLogDir || `${dataDir}/logs` : "",
        tlsCert: trimString(merged.tlsCert),
        tlsKey: trimString(merged.tlsKey),
        rateGlobalRps: trimString(merged.rateGlobalRps),
        rateLoginLimit: trimString(merged.rateLoginLimit),
        rateLoginWindow: trimString(merged.rateLoginWindow),
        redisAddr: trimString(merged.redisAddr),
        redisPassword: trimString(merged.redisPassword),
    };
}

export function applyInstallExperience(currentDraft, installExperience) {
    const nextDraft = normalizeInstallerDraft(currentDraft);
    if (installExperience === "advanced") {
        return {
            ...nextDraft,
            installExperience: "advanced",
        };
    }

    return {
        ...nextDraft,
        installExperience: "quick",
        runtimeMode: "production",
        addr: DEFAULT_DRAFT.addr,
        storageDriver: "json",
        postgresDsn: "",
        sessionStore: "memory",
        sessionStoreDsn: "",
        allowSelfSignup: false,
        enableLogs: true,
        logDir: "",
        tlsCert: "",
        tlsKey: "",
        redisAddr: "",
        redisPassword: "",
    };
}

export function applyStorageDriver(currentDraft, storageDriver) {
    const nextDraft = normalizeInstallerDraft(currentDraft);
    if (storageDriver === "postgres") {
        return {
            ...nextDraft,
            storageDriver: "postgres",
            sessionStore: nextDraft.sessionStore === "memory" ? "postgres" : nextDraft.sessionStore,
        };
    }

    return {
        ...nextDraft,
        storageDriver: "json",
        postgresDsn: "",
        sessionStore: "memory",
        sessionStoreDsn: "",
    };
}

export function applySessionStore(currentDraft, sessionStore) {
    const nextDraft = normalizeInstallerDraft(currentDraft);
    if (sessionStore === "postgres") {
        return {
            ...nextDraft,
            sessionStore: "postgres",
        };
    }
    return {
        ...nextDraft,
        sessionStore: "memory",
        sessionStoreDsn: "",
    };
}

function describeInstallExperience(installExperience) {
    return installExperience === "advanced" ? "Advanced Install" : "Quick Install (recommended)";
}

function describeRuntimeMode(runtimeMode) {
    return runtimeMode === "development" ? "Development mode" : "Standard production mode";
}

function describeStorage(data) {
    if (data.storageDriver === "postgres") {
        return "Postgres";
    }
    return "Built-in file storage";
}

function describeSessionStore(data) {
    if (data.sessionStore === "postgres") {
        return data.sessionStoreDsn ? "Postgres session storage" : "Postgres session storage (reuses primary database)";
    }
    return "In-memory sessions";
}

function describeRateLimits(data) {
    if (!data.rateGlobalRps && !data.rateLoginLimit && !data.rateLoginWindow) {
        return "Not configured";
    }
    return `${data.rateGlobalRps || "0"} req/s global, ${data.rateLoginLimit || "0"} login attempts every ${data.rateLoginWindow || "window"}`;
}

function deriveUrlDetails(data) {
    const port = extractPort(data.addr);
    const scheme = data.tlsCert && data.tlsKey ? "https" : "http";
    const host = data.hostname || "your-server";
    const omitPort = (scheme === "http" && port === "80") || (scheme === "https" && port === "443");
    const portSuffix = !port || omitPort ? "" : `:${port}`;
    const baseUrl = `${scheme}://${host}${portSuffix}`;
    return {
        baseUrl,
        appUrl: baseUrl,
        adminUrl: `${baseUrl}/admin`,
    };
}

export function deriveSuccessSummary(draft) {
    const data = normalizeInstallerDraft(draft);
    const urls = deriveUrlDetails(data);
    return {
        appUrl: urls.appUrl,
        adminUrl: urls.adminUrl,
        configPath: `${data.installDir}/.env`,
        dataPath: data.dataDir,
        logPath: data.enableLogs ? data.logDir : "",
        nextSteps: [
            "SSH into your Ubuntu server with an account that can use sudo.",
            "Paste the generated command into the terminal and let the helper finish staging the service.",
            `Sign in with ${data.adminEmail} at ${urls.adminUrl} once the helper finishes.`,
            "Rotate the temporary admin password after the first successful sign-in.",
        ],
    };
}

export function buildSystemChecks(draft) {
    const data = normalizeInstallerDraft(draft);
    const port = extractPort(data.addr);
    const privilegedPort = port && Number(port) < 1024;
    const postgresReady = data.storageDriver === "json" || Boolean(data.postgresDsn);

    return [
        {
            title: "Supported target",
            status: "pass",
            summary: "This guided install targets Ubuntu 22.04+ servers that use systemd.",
            action: "If you need macOS or Windows service files instead, use the CLI installers in the Ubuntu guide.",
        },
        {
            title: "Server access",
            status: "warning",
            summary: "This browser cannot inspect the target machine directly.",
            action: "Before you continue, make sure you can SSH in and run sudo on the server you plan to install.",
        },
        {
            title: "Port readiness",
            status: privilegedPort ? "warning" : "pass",
            summary: privilegedPort
                ? `Port ${port} is public-friendly, but the server needs privileged-port support to bind it safely.`
                : `Default port ${port || "8080"} avoids privileged-port setup on the first run.`,
            action: privilegedPort
                ? "Use Advanced Install later if you want to switch to :8080 for the safest first run."
                : "You can move to :80 or :443 later once you add a reverse proxy or TLS setup.",
        },
        {
            title: "Storage plan",
            status: postgresReady ? "pass" : "fail",
            summary:
                data.storageDriver === "postgres"
                    ? "Postgres is selected, so the final review will require a real database connection string."
                    : "Quick Install keeps data on the server with built-in storage and no external database.",
            action:
                data.storageDriver === "postgres"
                    ? "Add a real Postgres DSN in Advanced Install before you start the handoff."
                    : "You can move to Postgres later without learning the whole stack on day one.",
        },
        {
            title: "Recovery path",
            status: "pass",
            summary: "You can go back, retry, and keep your answers if validation or command preparation fails.",
            action: "The wizard never clears the form for you after an error.",
        },
    ];
}

export function validateInstallerDraft(draft) {
    const data = normalizeInstallerDraft(draft);
    const fieldErrors = {};
    const generalErrors = [];

    if (!data.adminEmail) {
        fieldErrors.adminEmail = "Add the email address you want to use for the first admin sign-in.";
    } else if (!isValidEmail(data.adminEmail)) {
        fieldErrors.adminEmail = "Enter a full email address like admin@example.com.";
    }

    if (!data.adminPassword) {
        fieldErrors.adminPassword = "Use the generated password or enter your own temporary admin password.";
    } else if (!isStrongPassword(data.adminPassword)) {
        fieldErrors.adminPassword = "Use at least 16 characters with uppercase, lowercase, and numbers.";
    }

    if (!data.installDir || !isAbsoluteUnixPath(data.installDir)) {
        fieldErrors.installDir = "Use an absolute Linux path such as /opt/bitriver-live.";
    }

    if (!data.dataDir || !isAbsoluteUnixPath(data.dataDir)) {
        fieldErrors.dataDir = "Use an absolute Linux path such as /var/lib/bitriver-live.";
    }

    if (!/^[a-z_][a-z0-9_-]*[$]?$/i.test(data.serviceUser)) {
        fieldErrors.serviceUser = "Use a simple Linux service account name such as bitriver.";
    }

    if (!data.addr || !extractPort(data.addr)) {
        fieldErrors.addr = "Enter a listen address like :8080, :80, or 0.0.0.0:8080.";
    }

    if (data.hostname && !isValidHostname(data.hostname)) {
        fieldErrors.hostname = "Enter a hostname or IP address without http:// or any path.";
    }

    if (data.tlsCert && !isAbsoluteUnixPath(data.tlsCert)) {
        fieldErrors.tlsCert = "Use an absolute Linux path to the certificate file.";
    }

    if (data.tlsKey && !isAbsoluteUnixPath(data.tlsKey)) {
        fieldErrors.tlsKey = "Use an absolute Linux path to the private key file.";
    }

    if ((data.tlsCert && !data.tlsKey) || (!data.tlsCert && data.tlsKey)) {
        fieldErrors.tlsCert = "Add both the TLS certificate and key, or leave both blank.";
        fieldErrors.tlsKey = "Add both the TLS certificate and key, or leave both blank.";
    }

    if (data.storageDriver === "postgres") {
        if (!data.postgresDsn) {
            fieldErrors.postgresDsn = "Postgres needs a real DSN before the installer can run.";
        } else if (containsPlaceholderSecret(data.postgresDsn)) {
            fieldErrors.postgresDsn = "Replace placeholder Postgres credentials with a real username and password.";
        }
    }

    if (data.sessionStore === "postgres") {
        const sessionDsn = data.sessionStoreDsn || data.postgresDsn;
        if (!sessionDsn) {
            fieldErrors.sessionStoreDsn = "Postgres sessions need a DSN or they must reuse the main Postgres connection.";
        } else if (containsPlaceholderSecret(sessionDsn)) {
            fieldErrors.sessionStoreDsn = "Replace placeholder session-store credentials with a real DSN.";
        }
    }

    if (!isPositiveIntegerString(data.rateGlobalRps)) {
        fieldErrors.rateGlobalRps = "Use a whole number such as 50, or leave it blank.";
    }

    if (!isPositiveIntegerString(data.rateLoginLimit)) {
        fieldErrors.rateLoginLimit = "Use a whole number such as 5, or leave it blank.";
    }

    if (!isValidDuration(data.rateLoginWindow)) {
        fieldErrors.rateLoginWindow = "Use a short duration such as 2m, 30s, or leave it blank.";
    }

    if (data.redisAddr && !isValidRedisAddress(data.redisAddr)) {
        fieldErrors.redisAddr = "Use host:port format such as redis:6379.";
    }

    if (data.redisPassword && !data.redisAddr) {
        fieldErrors.redisAddr = "Add a Redis address if you want to save a Redis password.";
    }

    if (Object.keys(fieldErrors).length > 0) {
        generalErrors.push("Fix the highlighted fields before you continue.");
    }

    return {
        data,
        fieldErrors,
        generalErrors,
        hasErrors: Object.keys(fieldErrors).length > 0,
    };
}

export function buildReviewSections(draft) {
    const data = normalizeInstallerDraft(draft);
    const success = deriveSuccessSummary(data);

    const sections = [
        {
            title: "Install plan",
            rows: [
                { label: "Path", value: describeInstallExperience(data.installExperience) },
                { label: "App mode", value: describeRuntimeMode(data.runtimeMode) },
                { label: "Install folder", value: data.installDir },
                { label: "Data folder", value: data.dataDir },
                { label: "Config file", value: success.configPath },
                { label: "Service account", value: data.serviceUser },
                { label: "Logs", value: data.enableLogs ? data.logDir : "Not created" },
            ],
        },
        {
            title: "Sign-in and access",
            rows: [
                { label: "Admin email", value: data.adminEmail || "Not set" },
                { label: "Temporary admin password", value: maskSecret(data.adminPassword) },
                { label: "Public hostname", value: data.hostname || "Set this later" },
                { label: "App URL", value: success.appUrl },
                { label: "Admin URL", value: success.adminUrl },
                { label: "Listen address", value: data.addr },
            ],
        },
        {
            title: "Storage and services",
            rows: [
                { label: "Primary storage", value: describeStorage(data) },
                { label: "Session storage", value: describeSessionStore(data) },
                { label: "Self-signup", value: formatBoolean(data.allowSelfSignup, "Enabled", "Disabled") },
                { label: "Rate limits", value: describeRateLimits(data) },
                { label: "Redis", value: data.redisAddr || "Not used" },
            ],
        },
    ];

    if (data.storageDriver === "postgres" || data.tlsCert || data.tlsKey) {
        sections.push({
            title: "Technical choices",
            rows: [
                { label: "Postgres", value: data.postgresDsn ? "Configured" : "Not configured" },
                {
                    label: "Session-store DSN",
                    value: data.sessionStore === "postgres" ? (data.sessionStoreDsn ? "Custom DSN saved" : "Reuses primary DSN") : "Memory only",
                },
                { label: "TLS certificate", value: data.tlsCert || "Not configured" },
                { label: "TLS key", value: data.tlsKey || "Not configured" },
            ],
        });
    }

    return sections;
}

export function computeInstallerScript(draft) {
    const data = normalizeInstallerDraft(draft);
    const flags = [
        '--install-dir "$INSTALL_DIR"',
        '--data-dir "$DATA_DIR"',
        '--service-user "$SERVICE_USER"',
        '--mode "$RUNTIME_MODE"',
        '--addr "$ADDR"',
        '--storage-driver "$STORAGE_DRIVER"',
        '--allow-self-signup "$ALLOW_SELF_SIGNUP"',
        '--bootstrap-admin-email "$ADMIN_EMAIL"',
        '--bootstrap-admin-password "$ADMIN_PASSWORD"',
    ];

    if (data.enableLogs) {
        flags.push("--enable-logs");
        flags.push('--log-dir "$LOG_DIR"');
    }
    if (data.hostname) {
        flags.push('--hostname "$HOSTNAME"');
    }
    if (data.tlsCert && data.tlsKey) {
        flags.push('--tls-cert "$TLS_CERT"');
        flags.push('--tls-key "$TLS_KEY"');
    }
    if (data.storageDriver === "postgres") {
        flags.push('--postgres-dsn "$POSTGRES_DSN"');
    }
    if (data.sessionStore === "postgres") {
        flags.push('--session-store "$SESSION_STORE"');
        if (data.sessionStoreDsn) {
            flags.push('--session-store-dsn "$SESSION_STORE_DSN"');
        }
    }
    if (data.rateGlobalRps) {
        flags.push('--rate-global-rps "$RATE_GLOBAL_RPS"');
    }
    if (data.rateLoginLimit) {
        flags.push('--rate-login-limit "$RATE_LOGIN_LIMIT"');
    }
    if (data.rateLoginWindow) {
        flags.push('--rate-login-window "$RATE_LOGIN_WINDOW"');
    }
    if (data.redisAddr) {
        flags.push('--redis-addr "$REDIS_ADDR"');
    }
    if (data.redisPassword) {
        flags.push('--redis-password "$REDIS_PASSWORD"');
    }

    const flagBlock = flags
        .map((flag, index) => {
            const suffix = index === flags.length - 1 ? "" : " \\";
            return `  ${flag}${suffix}`;
        })
        .join("\n");

    return `#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${escapeShellDoubleQuoted(data.installDir)}"
DATA_DIR="${escapeShellDoubleQuoted(data.dataDir)}"
SERVICE_USER="${escapeShellDoubleQuoted(data.serviceUser)}"
RUNTIME_MODE="${escapeShellDoubleQuoted(data.runtimeMode)}"
ADDR="${escapeShellDoubleQuoted(data.addr)}"
STORAGE_DRIVER="${escapeShellDoubleQuoted(data.storageDriver)}"
ALLOW_SELF_SIGNUP="${data.allowSelfSignup ? "true" : "false"}"
ADMIN_EMAIL="${escapeShellDoubleQuoted(data.adminEmail)}"
ADMIN_PASSWORD="${escapeShellDoubleQuoted(data.adminPassword)}"
LOG_DIR="${escapeShellDoubleQuoted(data.logDir)}"
HOSTNAME="${escapeShellDoubleQuoted(data.hostname)}"
TLS_CERT="${escapeShellDoubleQuoted(data.tlsCert)}"
TLS_KEY="${escapeShellDoubleQuoted(data.tlsKey)}"
POSTGRES_DSN="${escapeShellDoubleQuoted(data.postgresDsn)}"
SESSION_STORE="${escapeShellDoubleQuoted(data.sessionStore)}"
SESSION_STORE_DSN="${escapeShellDoubleQuoted(data.sessionStoreDsn)}"
RATE_GLOBAL_RPS="${escapeShellDoubleQuoted(data.rateGlobalRps)}"
RATE_LOGIN_LIMIT="${escapeShellDoubleQuoted(data.rateLoginLimit)}"
RATE_LOGIN_WINDOW="${escapeShellDoubleQuoted(data.rateLoginWindow)}"
REDIS_ADDR="${escapeShellDoubleQuoted(data.redisAddr)}"
REDIS_PASSWORD="${escapeShellDoubleQuoted(data.redisPassword)}"
SCRIPT_URL="${INSTALLER_SCRIPT_URL}"
SCRIPT_PATH="$(mktemp)"

trap 'rm -f "$SCRIPT_PATH"' EXIT

curl -fsSL "$SCRIPT_URL" -o "$SCRIPT_PATH"
chmod +x "$SCRIPT_PATH"
"$SCRIPT_PATH" \\
${flagBlock}
`;
}

export function createInstallerExecution(draft) {
    const data = normalizeInstallerDraft(draft);
    const success = deriveSuccessSummary(data);
    const script = computeInstallerScript(data);
    const technicalLog = [
        `Install path: ${describeInstallExperience(data.installExperience)}`,
        `Runtime mode: ${data.runtimeMode}`,
        `Storage driver: ${data.storageDriver}`,
        `Session store: ${data.sessionStore}`,
        `Listen address: ${data.addr}`,
        `Config file: ${success.configPath}`,
        `Data folder: ${success.dataPath}`,
        `Guide: ${INSTALLER_GUIDE_URL}`,
        `Helper source: ${INSTALLER_SCRIPT_URL}`,
        "",
        "Generated install command:",
        script.trimEnd(),
    ].join("\n");

    return {
        status: "ready",
        progress: createProgressItems().map((item) => ({ ...item, status: "complete" })),
        script,
        technicalLog,
        copied: false,
        error: "",
        success,
    };
}

function renderStepHeader(stepIndex) {
    const step = INSTALLER_STEPS[stepIndex];
    return `
        <header class="installer__step-header">
            <p class="installer__eyebrow">${escapeHTML(formatStepCount(stepIndex))}</p>
            <h4>${escapeHTML(step.title)}</h4>
            <p>${escapeHTML(step.description)}</p>
        </header>
    `;
}

function renderProgressStepper(stepIndex) {
    const items = INSTALLER_STEPS.map((step, index) => {
        let status = "upcoming";
        if (index < stepIndex) {
            status = "complete";
        } else if (index === stepIndex) {
            status = "current";
        }
        return `
            <li class="installer__step-chip" data-state="${status}">
                <span class="installer__step-number">${index + 1}</span>
                <span class="installer__step-label">${escapeHTML(step.label)}</span>
            </li>
        `;
    }).join("");

    return `
        <ol class="installer__stepper" aria-label="Installer steps">
            ${items}
        </ol>
    `;
}

function renderSidebar(state) {
    const data = normalizeInstallerDraft(state.draft);
    const success = deriveSuccessSummary(data);
    return `
        <aside class="installer__sidebar">
            <div class="installer__sidebar-card">
                <p class="installer__sidebar-label">Current plan</p>
                <h4>${escapeHTML(describeInstallExperience(data.installExperience))}</h4>
                <dl class="installer__sidebar-list">
                    <div>
                        <dt>App URL</dt>
                        <dd>${escapeHTML(success.appUrl)}</dd>
                    </div>
                    <div>
                        <dt>Admin URL</dt>
                        <dd>${escapeHTML(success.adminUrl)}</dd>
                    </div>
                    <div>
                        <dt>Config file</dt>
                        <dd>${escapeHTML(success.configPath)}</dd>
                    </div>
                    <div>
                        <dt>Data folder</dt>
                        <dd>${escapeHTML(success.dataPath)}</dd>
                    </div>
                </dl>
            </div>
            <div class="installer__sidebar-card">
                <p class="installer__sidebar-label">Why this feels safer</p>
                <ul class="installer__sidebar-points">
                    <li>Quick Install stays on the simplest supported path.</li>
                    <li>Validation and review happen before the install handoff.</li>
                    <li>Technical details stay tucked away until you ask for them.</li>
                </ul>
            </div>
            <a class="installer__docs" href="${INSTALLER_GUIDE_URL}" target="_blank" rel="noreferrer">Open the Ubuntu install guide</a>
        </aside>
    `;
}

function renderErrorPanel(messages) {
    if (!Array.isArray(messages) || messages.length === 0) {
        return "";
    }
    const items = messages.map((message) => `<li>${escapeHTML(message)}</li>`).join("");
    return `
        <section class="installer__error-panel" role="alert" aria-live="assertive">
            <h5>Something needs attention</h5>
            <ul>${items}</ul>
        </section>
    `;
}

function renderTechnicalDetails(summary, body) {
    return `
        <details class="installer__details">
            <summary>${escapeHTML(summary)}</summary>
            <div class="installer__details-body">
                ${body}
            </div>
        </details>
    `;
}

function renderTechnicalList(items) {
    const rows = items.map((item) => `<li>${escapeHTML(item)}</li>`).join("");
    return `<ul class="installer__technical-list">${rows}</ul>`;
}

function renderStatusList(items) {
    const rows = items
        .map(
            (item) => `
                <li class="installer__status-item" data-status="${escapeHTML(item.status)}">
                    <div class="installer__status-head">
                        <span class="installer__status-badge">${escapeHTML(item.status)}</span>
                        <strong>${escapeHTML(item.title)}</strong>
                    </div>
                    <p>${escapeHTML(item.summary || item.detail || "")}</p>
                    ${item.action ? `<p class="installer__status-action">${escapeHTML(item.action)}</p>` : ""}
                    ${Array.isArray(item.technicalDetails) && item.technicalDetails.length > 0 ? renderTechnicalDetails("Show technical details", renderTechnicalList(item.technicalDetails)) : ""}
                </li>
            `,
        )
        .join("");

    return `<ul class="installer__status-list">${rows}</ul>`;
}

function renderField({ name, label, help, value = "", type = "text", placeholder = "", error = "", required = false }) {
    const inputId = `installer-${name}`;
    return `
        <label class="installer__field${error ? " has-error" : ""}" for="${inputId}">
            <span class="installer__field-label">
                ${escapeHTML(label)}
                ${required ? '<span class="installer__required">Required</span>' : ""}
            </span>
            <span class="installer__field-help">${help}</span>
            <input
                id="${inputId}"
                name="${escapeHTML(name)}"
                type="${escapeHTML(type)}"
                value="${escapeHTML(value)}"
                placeholder="${escapeHTML(placeholder)}"
                autocomplete="${type === "password" ? "new-password" : "off"}"
            />
            ${error ? `<span class="installer__field-error">${escapeHTML(error)}</span>` : ""}
        </label>
    `;
}

function renderSelectField({ name, label, help, value = "", options = [], error = "" }) {
    const inputId = `installer-${name}`;
    const optionMarkup = options
        .map(
            (option) => `
                <option value="${escapeHTML(option.value)}"${option.value === value ? " selected" : ""}>
                    ${escapeHTML(option.label)}
                </option>
            `,
        )
        .join("");

    return `
        <label class="installer__field${error ? " has-error" : ""}" for="${inputId}">
            <span class="installer__field-label">${escapeHTML(label)}</span>
            <span class="installer__field-help">${help}</span>
            <select id="${inputId}" name="${escapeHTML(name)}">
                ${optionMarkup}
            </select>
            ${error ? `<span class="installer__field-error">${escapeHTML(error)}</span>` : ""}
        </label>
    `;
}

function renderCheckboxField({ name, label, help, checked = false }) {
    const inputId = `installer-${name}`;
    return `
        <label class="installer__checkbox" for="${inputId}">
            <input id="${inputId}" name="${escapeHTML(name)}" type="checkbox"${checked ? " checked" : ""} />
            <span>
                <strong>${escapeHTML(label)}</strong>
                <small>${help}</small>
            </span>
        </label>
    `;
}

function renderPasswordField({ label, help, value, error }) {
    const field = renderField({
        name: "adminPassword",
        label,
        help,
        value,
        type: "password",
        error,
        required: true,
    });
    return `
        <div class="installer__password-row">
            ${field}
            <button type="button" class="secondary installer__mini-button" data-action="generate-password">Generate a new password</button>
        </div>
    `;
}

function renderWelcomeStep() {
    return `
        <section class="installer__panel">
            <div class="installer__hero">
                <div>
                    <h5>What this wizard does</h5>
                    <p>It keeps the install linear, uses safer defaults for a first self-hosted setup, and still hands everything off to the existing Ubuntu installer under the hood.</p>
                </div>
                <div class="installer__hero-card">
                    <p class="installer__sidebar-label">You will get</p>
                    <ul class="installer__sidebar-points">
                        <li>A simpler Quick Install path for non-expert operators.</li>
                        <li>A full review before the install command is generated.</li>
                        <li>A success handoff with URLs, config paths, and next steps.</li>
                    </ul>
                </div>
            </div>
            <div class="installer__button-row">
                <button type="button" class="primary" data-action="next">Run system check</button>
            </div>
        </section>
    `;
}

function renderSystemCheckStep(state) {
    const preflight = state.preflight || createPreflightState(state.draft);
    const checks = Array.isArray(preflight.checks) && preflight.checks.length > 0 ? preflight.checks : buildSystemChecks(state.draft);
    const checkedAt = formatCheckedAt(preflight.checkedAt);
    const note =
        preflight.status === "loading"
            ? "Refreshing live checks from this host now. You can stay on this step while the results update."
            : preflight.source === "server"
              ? checkedAt
                    ? `Live host checks updated ${checkedAt}. Revisit this step any time if you want to refresh them.`
                    : "Live host checks loaded from this server."
              : "Live host checks are unavailable right now, so the wizard is showing safe fallback guidance instead.";
    const technicalSummary = [
        `Check source: ${preflight.source === "server" ? "server preflight endpoint" : "browser fallback guidance"}`,
        checkedAt ? `Last checked: ${checkedAt}` : "",
        `Helper source: ${INSTALLER_SCRIPT_URL}`,
    ].filter(Boolean);
    return `
        <section class="installer__panel">
            ${preflight.error ? renderErrorPanel([preflight.error]) : ""}
            <p class="installer__note">${escapeHTML(note)}</p>
            ${renderStatusList(checks)}
            ${renderTechnicalDetails("Show technical details", renderTechnicalList(technicalSummary))}
            <div class="installer__button-row">
                <button type="button" class="secondary" data-action="back">Back</button>
                <button type="button" class="secondary" data-action="retry-preflight">Refresh checks</button>
                <button type="button" class="primary" data-action="next">Continue</button>
            </div>
        </section>
    `;
}

function renderInstallModeStep(state) {
    const installExperience = normalizeInstallerDraft(state.draft).installExperience;
    return `
        <section class="installer__panel">
            <div class="installer__choice-grid">
                <label class="installer__choice${installExperience === "quick" ? " is-selected" : ""}">
                    <input type="radio" name="installExperience" value="quick"${installExperience === "quick" ? " checked" : ""} />
                    <span class="installer__choice-label">Quick Install</span>
                    <span class="installer__choice-tag">Recommended</span>
                    <p>Start with built-in storage, a safe default port, generated admin credentials, and fewer decisions.</p>
                </label>
                <label class="installer__choice${installExperience === "advanced" ? " is-selected" : ""}">
                    <input type="radio" name="installExperience" value="advanced"${installExperience === "advanced" ? " checked" : ""} />
                    <span class="installer__choice-label">Advanced Install</span>
                    <span class="installer__choice-tag">More control</span>
                    <p>Open database, TLS, Redis, session, and service details when you need to tailor the host.</p>
                </label>
            </div>
            <div class="installer__note">
                ${installExperience === "quick"
                    ? "Quick Install stays on the cleanest first-run path and hides the lower-level service knobs until you ask for them."
                    : "Advanced Install keeps the same flow, but you will see the storage, network, and service details before review."}
            </div>
            <div class="installer__button-row">
                <button type="button" class="secondary" data-action="back">Back</button>
                <button type="button" class="primary" data-action="next">Continue</button>
            </div>
        </section>
    `;
}

function renderQuickSettings(data, fieldErrors) {
    const technicalRows = [
        { label: "Install mode", value: describeInstallExperience(data.installExperience) },
        { label: "Runtime mode", value: describeRuntimeMode(data.runtimeMode) },
        { label: "Listen address", value: data.addr },
        { label: "Storage", value: describeStorage(data) },
        { label: "Session store", value: describeSessionStore(data) },
        { label: "Logs", value: data.enableLogs ? data.logDir : "Not created" },
        { label: "Self-signup", value: data.allowSelfSignup ? "Enabled" : "Disabled" },
    ]
        .map(
            (row) => `
                <div class="installer__review-row">
                    <dt>${escapeHTML(row.label)}</dt>
                    <dd>${escapeHTML(row.value)}</dd>
                </div>
            `,
        )
        .join("");

    return `
        <div class="installer__field-grid">
            ${renderField({
                name: "adminEmail",
                label: "Admin email",
                help: "This creates the first administrator account. Change it only if you already have a preferred sign-in address.",
                value: data.adminEmail,
                error: fieldErrors.adminEmail,
                required: true,
            })}
            ${renderPasswordField({
                label: "Temporary admin password",
                help: "A strong password is generated for you. Change it only if you already have a temporary password you want to use.",
                value: data.adminPassword,
                error: fieldErrors.adminPassword,
            })}
            ${renderField({
                name: "hostname",
                label: "Public hostname (optional)",
                help: "Use the DNS name or IP people will open later. Leave it blank if you want to decide that after install.",
                value: data.hostname,
                placeholder: "stream.example.com",
                error: fieldErrors.hostname,
            })}
            ${renderField({
                name: "installDir",
                label: "Install folder",
                help: "This is where BitRiver Live stores its binaries and generated service files. Change it only if you already manage software elsewhere.",
                value: data.installDir,
                error: fieldErrors.installDir,
                required: true,
            })}
            ${renderField({
                name: "dataDir",
                label: "Data folder",
                help: "This is where BitRiver Live keeps app data, uploads, and generated state. Change it only if you already use another storage mount.",
                value: data.dataDir,
                error: fieldErrors.dataDir,
                required: true,
            })}
        </div>
        ${renderTechnicalDetails(
            "Show technical details",
            `<dl class="installer__review-list">${technicalRows}</dl>`,
        )}
    `;
}

function renderAdvancedSettings(data, fieldErrors) {
    return `
        <div class="installer__stack">
            <section class="installer__subsection">
                <h5>Account and paths</h5>
                <div class="installer__field-grid">
                    ${renderField({
                        name: "adminEmail",
                        label: "Admin email",
                        help: "This creates the first administrator account so you can sign in right away after install.",
                        value: data.adminEmail,
                        error: fieldErrors.adminEmail,
                        required: true,
                    })}
                    ${renderPasswordField({
                        label: "Temporary admin password",
                        help: "Use the generated password or enter your own strong temporary password for the first sign-in.",
                        value: data.adminPassword,
                        error: fieldErrors.adminPassword,
                    })}
                    ${renderField({
                        name: "installDir",
                        label: "Install folder",
                        help: "This stores the app binaries and generated service files.",
                        value: data.installDir,
                        error: fieldErrors.installDir,
                        required: true,
                    })}
                    ${renderField({
                        name: "dataDir",
                        label: "Data folder",
                        help: "This stores application data, uploads, and logs.",
                        value: data.dataDir,
                        error: fieldErrors.dataDir,
                        required: true,
                    })}
                    ${renderField({
                        name: "serviceUser",
                        label: "Service account",
                        help: "The Linux account that should own the files and run the systemd service.",
                        value: data.serviceUser,
                        error: fieldErrors.serviceUser,
                        required: true,
                    })}
                    ${renderField({
                        name: "hostname",
                        label: "Public hostname (optional)",
                        help: "Use the hostname or IP you expect operators to open later.",
                        value: data.hostname,
                        placeholder: "stream.example.com",
                        error: fieldErrors.hostname,
                    })}
                </div>
            </section>
            <section class="installer__subsection">
                <h5>Network and storage</h5>
                <div class="installer__field-grid">
                    ${renderSelectField({
                        name: "runtimeMode",
                        label: "Runtime mode",
                        help: "Production is the normal path for a real install. Development is better when you are testing locally.",
                        value: data.runtimeMode,
                        options: [
                            { value: "production", label: "Production" },
                            { value: "development", label: "Development" },
                        ],
                    })}
                    ${renderField({
                        name: "addr",
                        label: "Listen address",
                        help: "Use a value like :8080 or :80. Change it if you already know which host port the service should bind.",
                        value: data.addr,
                        error: fieldErrors.addr,
                        required: true,
                    })}
                    ${renderSelectField({
                        name: "storageDriver",
                        label: "Primary storage",
                        help: "Built-in storage is easier for a first run. Postgres is for operators who already have a database ready.",
                        value: data.storageDriver,
                        options: [
                            { value: "json", label: "Built-in storage" },
                            { value: "postgres", label: "Postgres" },
                        ],
                    })}
                    ${renderSelectField({
                        name: "sessionStore",
                        label: "Session storage",
                        help: "Keep sessions in memory for the simplest setup, or store them in Postgres when you already use a database.",
                        value: data.sessionStore,
                        options: [
                            { value: "memory", label: "Memory" },
                            { value: "postgres", label: "Postgres" },
                        ],
                    })}
                    ${data.storageDriver === "postgres"
                        ? renderField({
                              name: "postgresDsn",
                              label: "Postgres DSN",
                              help: "Use the real connection string for the database that will back BitRiver Live.",
                              value: data.postgresDsn,
                              placeholder: "postgres://user:strong-password@localhost:5432/bitriver_live?sslmode=disable",
                              error: fieldErrors.postgresDsn,
                              required: true,
                          })
                        : ""}
                    ${data.sessionStore === "postgres"
                        ? renderField({
                              name: "sessionStoreDsn",
                              label: "Session-store DSN (optional)",
                              help: "Leave this blank to reuse the primary Postgres DSN, or set a separate database for sessions.",
                              value: data.sessionStoreDsn,
                              placeholder: "postgres://user:strong-password@localhost:5432/bitriver_sessions?sslmode=disable",
                              error: fieldErrors.sessionStoreDsn,
                          })
                        : ""}
                </div>
            </section>
            <section class="installer__subsection">
                <h5>TLS and service behavior</h5>
                <div class="installer__field-grid">
                    ${renderField({
                        name: "tlsCert",
                        label: "TLS certificate path",
                        help: "Set this when the BitRiver Live service should serve HTTPS directly from the host.",
                        value: data.tlsCert,
                        placeholder: "/etc/letsencrypt/live/stream.example.com/fullchain.pem",
                        error: fieldErrors.tlsCert,
                    })}
                    ${renderField({
                        name: "tlsKey",
                        label: "TLS key path",
                        help: "This is the private key that pairs with the certificate above.",
                        value: data.tlsKey,
                        placeholder: "/etc/letsencrypt/live/stream.example.com/privkey.pem",
                        error: fieldErrors.tlsKey,
                    })}
                    ${renderField({
                        name: "rateGlobalRps",
                        label: "Global request limit",
                        help: "Use a whole number to cap total API requests per second. Leave it blank to skip this limit.",
                        value: data.rateGlobalRps,
                        error: fieldErrors.rateGlobalRps,
                    })}
                    ${renderField({
                        name: "rateLoginLimit",
                        label: "Login attempt limit",
                        help: "This caps repeated login attempts from the same client during each window.",
                        value: data.rateLoginLimit,
                        error: fieldErrors.rateLoginLimit,
                    })}
                    ${renderField({
                        name: "rateLoginWindow",
                        label: "Login window",
                        help: "Use a short duration such as 2m or 30s for the login rate-limit window.",
                        value: data.rateLoginWindow,
                        error: fieldErrors.rateLoginWindow,
                    })}
                    ${renderField({
                        name: "redisAddr",
                        label: "Redis address (optional)",
                        help: "Set this only if you already have Redis and want to use it for rate limiting.",
                        value: data.redisAddr,
                        placeholder: "redis:6379",
                        error: fieldErrors.redisAddr,
                    })}
                    ${renderField({
                        name: "redisPassword",
                        label: "Redis password (optional)",
                        help: "Only set this when the Redis server above already requires authentication.",
                        value: data.redisPassword,
                        type: "password",
                    })}
                </div>
                <div class="installer__checkbox-grid">
                    ${renderCheckboxField({
                        name: "enableLogs",
                        label: "Create a managed log folder",
                        help: "This makes a log directory and keeps the service output in one predictable place.",
                        checked: data.enableLogs,
                    })}
                    ${renderCheckboxField({
                        name: "allowSelfSignup",
                        label: "Allow public self-signup",
                        help: "Leave this off if you want new accounts to stay under admin control at first.",
                        checked: data.allowSelfSignup,
                    })}
                </div>
                ${data.enableLogs
                    ? `<div class="installer__field-grid">${renderField({
                          name: "logDir",
                          label: "Log folder",
                          help: "Leave the default if you just want the installer to keep logs under your data folder.",
                          value: data.logDir,
                          error: "",
                      })}</div>`
                    : ""}
            </section>
        </div>
    `;
}

function renderCoreSettingsStep(state) {
    const data = normalizeInstallerDraft(state.draft);
    const fieldErrors = state.fieldErrors || {};

    return `
        <section class="installer__panel">
            ${renderErrorPanel(state.generalErrors)}
            ${data.installExperience === "advanced" ? renderAdvancedSettings(data, fieldErrors) : renderQuickSettings(data, fieldErrors)}
            <div class="installer__button-row">
                <button type="button" class="secondary" data-action="back">Back</button>
                <button type="button" class="primary" data-action="next">Continue</button>
            </div>
        </section>
    `;
}

function renderReviewRows(section) {
    const rows = section.rows
        .map(
            (row) => `
                <div class="installer__review-row">
                    <dt>${escapeHTML(row.label)}</dt>
                    <dd>${escapeHTML(row.value)}</dd>
                </div>
            `,
        )
        .join("");

    return `
        <section class="installer__review-section">
            <h5>${escapeHTML(section.title)}</h5>
            <dl class="installer__review-list">
                ${rows}
            </dl>
        </section>
    `;
}

function renderReviewStep(state) {
    const sections = buildReviewSections(state.draft).map((section) => renderReviewRows(section)).join("");
    const technicalSummary = createInstallerExecution(state.draft).technicalLog;
    return `
        <section class="installer__panel">
            ${sections}
            ${renderTechnicalDetails(
                "Show technical details",
                `<pre class="installer__code-block">${escapeHTML(technicalSummary)}</pre>`,
            )}
            <div class="installer__button-row">
                <button type="button" class="secondary" data-action="back">Back</button>
                <button type="button" class="primary" data-action="start-install">Start handoff</button>
            </div>
        </section>
    `;
}

function renderInstallingStep(state) {
    const execution = state.execution;
    const copiedMessage = execution.copied
        ? `<p class="installer__note">The install command is copied. Paste it into your Ubuntu server terminal and keep the success handoff open for reference.</p>`
        : "";
    return `
        <section class="installer__panel">
            ${execution.error ? renderErrorPanel([execution.error]) : ""}
            ${renderStatusList(execution.progress)}
            ${copiedMessage}
            ${execution.status === "ready"
                ? `
                    <div class="installer__button-row">
                        <button type="button" class="secondary" data-action="copy-script">Copy install command</button>
                        <button type="button" class="primary" data-action="go-success">Continue to success</button>
                    </div>
                `
                : `
                    <div class="installer__button-row">
                        <button type="button" class="secondary" data-action="retry-install">Retry</button>
                        <button type="button" class="secondary" data-action="back">Back</button>
                    </div>
                `}
            ${execution.script
                ? renderTechnicalDetails(
                      "Show technical details",
                      `<pre class="installer__code-block">${escapeHTML(execution.technicalLog)}</pre>`,
                  )
                : ""}
        </section>
    `;
}

function renderSuccessPanel(success, draft) {
    const data = normalizeInstallerDraft(draft);
    const rows = [
        { label: "App URL", value: success.appUrl },
        { label: "Admin URL", value: success.adminUrl },
        { label: "Config file", value: success.configPath },
        { label: "Data folder", value: success.dataPath },
    ];
    if (success.logPath) {
        rows.push({ label: "Log folder", value: success.logPath });
    }

    const rowMarkup = rows
        .map(
            (row) => `
                <div class="installer__review-row">
                    <dt>${escapeHTML(row.label)}</dt>
                    <dd>${escapeHTML(row.value)}</dd>
                </div>
            `,
        )
        .join("");

    const nextSteps = success.nextSteps.map((item) => `<li>${escapeHTML(item)}</li>`).join("");

    return `
        <section class="installer__success-panel">
            <div class="installer__success-copy">
                <p class="installer__success-badge">Ready to use</p>
                <h5>Keep this handoff nearby while the server finishes the install</h5>
                <p>The wizard has already prepared the command and the recovery details you are most likely to want later.</p>
            </div>
            <dl class="installer__review-list">
                ${rowMarkup}
                <div class="installer__review-row">
                    <dt>Admin sign-in</dt>
                    <dd>${escapeHTML(data.adminEmail)}</dd>
                </div>
                <div class="installer__review-row">
                    <dt>Temporary password</dt>
                    <dd>${escapeHTML(maskSecret(data.adminPassword))}</dd>
                </div>
            </dl>
            <div class="installer__success-next">
                <h6>Next steps</h6>
                <ol>${nextSteps}</ol>
            </div>
        </section>
    `;
}

function renderSuccessStep(state) {
    const success = state.execution.success || deriveSuccessSummary(state.draft);
    return `
        <section class="installer__panel">
            ${renderSuccessPanel(success, state.draft)}
            ${renderTechnicalDetails(
                "Show technical details",
                `<pre class="installer__code-block">${escapeHTML(state.execution.technicalLog || createInstallerExecution(state.draft).technicalLog)}</pre>`,
            )}
            <div class="installer__button-row">
                <button type="button" class="secondary" data-action="copy-script">Copy install command</button>
                <button type="button" class="secondary" data-action="back">Back</button>
                <button type="button" class="primary" data-action="start-over">Start over</button>
            </div>
        </section>
    `;
}

function renderStepBody(state) {
    switch (INSTALLER_STEPS[state.stepIndex]?.id) {
        case "welcome":
            return renderWelcomeStep();
        case "system-check":
            return renderSystemCheckStep(state);
        case "install-mode":
            return renderInstallModeStep(state);
        case "core-settings":
            return renderCoreSettingsStep(state);
        case "review":
            return renderReviewStep(state);
        case "installing":
            return renderInstallingStep(state);
        case "success":
            return renderSuccessStep(state);
        default:
            return "";
    }
}

function renderInstaller(state) {
    return `
        <section class="installer">
            <header class="installer__header">
                <span class="installer__eyebrow">Home server installer</span>
                <h3>A conventional, lower-stress install flow for self-hosting</h3>
                <p class="installer__intro">Quick Install is the default. Advanced Install stays available whenever you want to open the lower-level service details.</p>
            </header>
            ${renderProgressStepper(state.stepIndex)}
            <div class="installer__layout">
                <div class="installer__main">
                    ${renderStepHeader(state.stepIndex)}
                    ${renderStepBody(state)}
                </div>
                ${renderSidebar(state)}
            </div>
        </section>
    `;
}

function updateField(state, name, value) {
    state.draft = {
        ...state.draft,
        [name]: value,
    };
    delete state.fieldErrors[name];
    state.generalErrors = [];
}

function setProgressStatus(state, key, status) {
    state.execution.progress = state.execution.progress.map((item) =>
        item.key === key
            ? {
                  ...item,
                  status,
              }
            : item,
    );
}

function delay(ms) {
    return new Promise((resolve) => {
        setTimeout(resolve, ms);
    });
}

function installerPreflightSignature(draft) {
    return JSON.stringify(buildInstallerPreflightPayload(draft));
}

async function ensureInstallerPreflight(state, render, { force = false } = {}) {
    const signature = installerPreflightSignature(state.draft);
    if (state.preflight?.status === "loading") {
        return;
    }
    if (!force) {
        const sameSignature = state.preflight?.lastLoadedSignature === signature;
        if (sameSignature && state.preflight?.source === "server" && state.preflight?.status === "ready") {
            return;
        }
        if (sameSignature && state.preflight?.status === "error") {
            return;
        }
    }

    state.preflight = {
        ...createPreflightState(state.draft),
        ...state.preflight,
        status: "loading",
        error: "",
    };
    render();

    try {
        const result = await fetchInstallerPreflight(state.draft);
        state.preflight = {
            status: "ready",
            source: "server",
            checks: Array.isArray(result.checks) && result.checks.length > 0 ? result.checks : buildSystemChecks(state.draft),
            error: "",
            checkedAt: result.checkedAt,
            lastLoadedSignature: signature,
        };
    } catch (error) {
        state.preflight = {
            status: "error",
            source: "fallback",
            checks: buildSystemChecks(state.draft),
            error: "We could not load live host checks just now. The wizard kept your place and fell back to safe default guidance.",
            checkedAt: "",
            lastLoadedSignature: signature,
        };
    }

    render();
}

async function prepareInstall(state, render) {
    state.execution = {
        status: "running",
        progress: createProgressItems(),
        script: "",
        technicalLog: "",
        copied: false,
        error: "",
        success: null,
    };
    state.stepIndex = 5;
    render();

    const validation = validateInstallerDraft(state.draft);
    if (validation.hasErrors) {
        state.fieldErrors = validation.fieldErrors;
        state.generalErrors = validation.generalErrors;
        setProgressStatus(state, "validate", "fail");
        state.execution.status = "failed";
        state.execution.error = "We kept your answers, but a few fields still need to be fixed before the install handoff can be prepared.";
        render();
        return false;
    }

    setProgressStatus(state, "validate", "complete");
    render();
    await delay(120);

    setProgressStatus(state, "command", "active");
    render();
    await delay(120);

    const execution = createInstallerExecution(validation.data);
    setProgressStatus(state, "command", "complete");
    setProgressStatus(state, "handoff", "complete");
    setProgressStatus(state, "ready", "complete");
    state.execution = {
        ...execution,
        status: "ready",
    };
    render();
    return true;
}

async function copyInstallCommand(state, render, showToast) {
    if (!state.execution.script) {
        return;
    }
    try {
        if (!globalThis.navigator?.clipboard?.writeText) {
            throw new Error("Clipboard API unavailable");
        }
        await globalThis.navigator.clipboard.writeText(state.execution.script);
        state.execution.copied = true;
        render();
        showToast("Install command copied. Paste it into your Ubuntu server terminal.");
    } catch (error) {
        showToast("Copy did not complete automatically. Expand technical details and copy the command manually.", "error");
    }
}

export function setupInstallerWizard({ showToast } = {}) {
    const container = document.getElementById("installer");
    if (!container) {
        return;
    }

    const notify = typeof showToast === "function" ? showToast : () => {};
    const state = createInstallerState();

    const render = () => {
        container.innerHTML = renderInstaller(state);
    };

    container.addEventListener("input", (event) => {
        const target = event.target;
        if (!target || typeof target.name !== "string" || !target.name) {
            return;
        }

        const value = target.type === "checkbox" ? Boolean(target.checked) : target.value;
        updateField(state, target.name, value);
    });

    container.addEventListener("change", (event) => {
        const target = event.target;
        if (!target || typeof target.name !== "string" || !target.name) {
            return;
        }

        if (target.name === "installExperience") {
            state.draft = applyInstallExperience(state.draft, target.value);
            state.fieldErrors = {};
            state.generalErrors = [];
            render();
            return;
        }

        if (target.name === "storageDriver") {
            state.draft = applyStorageDriver(state.draft, target.value);
            render();
            return;
        }

        if (target.name === "sessionStore") {
            state.draft = applySessionStore(state.draft, target.value);
            render();
            return;
        }

        if (target.name === "enableLogs") {
            updateField(state, "enableLogs", Boolean(target.checked));
            render();
        }
    });

    container.addEventListener("click", async (event) => {
        const actionTarget = event.target?.closest?.("[data-action]");
        if (!actionTarget) {
            return;
        }

        const { action } = actionTarget.dataset;
        if (!action) {
            return;
        }

        switch (action) {
            case "next": {
                if (state.stepIndex === 3) {
                    const validation = validateInstallerDraft(state.draft);
                    state.fieldErrors = validation.fieldErrors;
                    state.generalErrors = validation.generalErrors;
                    if (validation.hasErrors) {
                        render();
                        return;
                    }
                }
                state.stepIndex = Math.min(state.stepIndex + 1, INSTALLER_STEPS.length - 1);
                render();
                if (state.stepIndex === 1) {
                    void ensureInstallerPreflight(state, render);
                }
                return;
            }
            case "back": {
                if (state.stepIndex === 6) {
                    state.stepIndex = 5;
                } else {
                    state.stepIndex = Math.max(state.stepIndex - 1, 0);
                }
                render();
                if (state.stepIndex === 1) {
                    void ensureInstallerPreflight(state, render);
                }
                return;
            }
            case "retry-preflight": {
                await ensureInstallerPreflight(state, render, { force: true });
                return;
            }
            case "generate-password": {
                updateField(state, "adminPassword", generateStrongPassword());
                render();
                return;
            }
            case "start-install": {
                const ready = await prepareInstall(state, render);
                if (ready) {
                    notify("Install handoff ready. Copy the command and run it on your Ubuntu server.");
                }
                return;
            }
            case "retry-install": {
                await prepareInstall(state, render);
                return;
            }
            case "copy-script": {
                await copyInstallCommand(state, render, notify);
                return;
            }
            case "go-success": {
                state.stepIndex = 6;
                state.execution.status = "completed";
                render();
                return;
            }
            case "start-over": {
                const reset = createInstallerState();
                state.stepIndex = reset.stepIndex;
                state.draft = reset.draft;
                state.fieldErrors = reset.fieldErrors;
                state.generalErrors = reset.generalErrors;
                state.preflight = reset.preflight;
                state.execution = reset.execution;
                render();
                return;
            }
            default:
                return;
        }
    });

    render();
}
