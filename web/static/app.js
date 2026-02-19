import { ChatClient } from "./chat-client.js";

class UnauthorizedError extends Error {
    constructor(message) {
        super(message);
        this.name = "UnauthorizedError";
    }
}

const state = {
    users: [],
    channels: [],
    sessions: {},
    chat: {},
    profiles: [],
    profileIndex: new Map(),
    selectedProfileId: null,
    currentUser: null,
    chatClient: null,
    moderation: {
        queue: [],
        actions: [],
        automod: [],
        appeals: [],
        filters: {},
        queueMeta: null,
        actionsMeta: null,
        automodMeta: null,
    },
    legal: {
        dmca: [],
        dataSubject: [],
    },
    analytics: { summary: null, perChannel: [] },
    uploads: new Map(),
    statusReport: null,
    mfaStatus: null,
};

let moderationLoaded = false;
let analyticsLoaded = false;
const moderationFilterCache = new Map();
const moderationFilterInvalidations = new Set();

function isCurrentUserAdmin() {
    const roles = state.currentUser?.roles;
    return Array.isArray(roles) && roles.includes("admin");
}

function escapeHTML(value) {
    const div = document.createElement("div");
    div.textContent = value ?? "";
    return div.innerHTML;
}

function createElement(tag, options = {}) {
    const element = document.createElement(tag);
    const { className, textContent, dataset, attributes } = options;
    if (className) {
        element.className = className;
    }
    if (textContent !== undefined) {
        element.textContent = textContent;
    }
    if (dataset) {
        for (const [key, value] of Object.entries(dataset)) {
            if (value !== undefined) {
                element.dataset[key] = value;
            }
        }
    }
    if (attributes) {
        for (const [name, value] of Object.entries(attributes)) {
            if (value !== undefined) {
                element.setAttribute(name, value);
            }
        }
    }
    return element;
}

function clearElement(element) {
    if (!element) {
        return;
    }
    while (element.firstChild) {
        element.removeChild(element.firstChild);
    }
}

function initChatClient() {
    if (state.chatClient) {
        return state.chatClient;
    }
    state.chatClient = new ChatClient({
        onEvent: handleChatEvent,
        onError: (error) => {
            const message = error instanceof Error ? error.message : "chat connection lost";
            showToast(`Chat error: ${message}`, "error");
        },
        onOpen: () => {
            syncChatSubscriptions();
        },
    });
    return state.chatClient;
}

function syncChatSubscriptions() {
    const client = state.chatClient;
    if (!client) {
        return;
    }
    const desired = new Set(state.channels.map((channel) => channel.id));
    if (client.joined) {
        for (const channelId of client.joined) {
            if (!desired.has(channelId)) {
                client.leave(channelId);
            }
        }
    }
    for (const channelId of desired) {
        client.join(channelId);
    }
}

function handleChatEvent(event) {
    if (!event) {
        return;
    }
    if (event.type === "message" && event.message) {
        const channelId = event.message.channelId;
        if (!state.chat[channelId]) {
            state.chat[channelId] = [];
        }
        const exists = state.chat[channelId].some((item) => item.id === event.message.id);
        if (!exists) {
            state.chat[channelId].unshift({
                id: event.message.id,
                channelId,
                userId: event.message.userId,
                content: event.message.content,
                createdAt: event.message.createdAt,
            });
            renderChat();
            renderDashboard();
        }
        return;
    }
    if (event.type === "moderation" && event.moderation) {
        const action = event.moderation.action.replace(/_/g, " ");
        const target = event.moderation.targetId;
        showToast(`Moderation ${action} for ${target}`, "info");
    }
    if (event.type === "automod" && event.automod) {
        const target = event.automod.userId || "unknown user";
        showToast(`Automod blocked a message from ${target}`, "warning");
        moderationLoaded = false;
    }
}

async function sendChatMessage(channelId, userId, content) {
    const client = initChatClient();
    if (client && state.currentUser && state.currentUser.id === userId) {
        client.join(channelId);
        client.message(channelId, content);
        return;
    }
    await apiRequest(`/api/channels/${channelId}/chat`, {
        method: "POST",
        body: JSON.stringify({ userId, content }),
    });
    await loadChatHistory(channelId, 50);
    renderChat();
}

async function sendModerationAction(channelId, action, targetId, durationMs = 0) {
    const client = initChatClient();
    if (client) {
        client.join(channelId);
        switch (action) {
            case "timeout":
                client.timeout(channelId, targetId, durationMs);
                return;
            case "remove_timeout":
                client.clearTimeout(channelId, targetId);
                return;
            case "ban":
                client.ban(channelId, targetId);
                return;
            case "unban":
                client.unban(channelId, targetId);
                return;
            default:
                throw new Error(`Unknown moderation action: ${action}`);
        }
    }
    await apiRequest(`/api/channels/${channelId}/chat/moderation`, {
        method: "POST",
        body: JSON.stringify({ action, targetId, durationMs }),
    });
}

async function createChatFilter(channelId, payload) {
    await apiRequest(`/api/channels/${channelId}/chat/moderation/filters`, {
        method: "POST",
        body: JSON.stringify(payload),
    });
    moderationLoaded = false;
    moderationFilterInvalidations.add(channelId);
    await loadModeration(true);
}

async function updateChatFilter(channelId, filterId, payload) {
    await apiRequest(`/api/channels/${channelId}/chat/moderation/filters/${filterId}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
    });
    moderationLoaded = false;
    moderationFilterInvalidations.add(channelId);
    await loadModeration(true);
}

async function deleteChatFilter(channelId, filterId) {
    await apiRequest(`/api/channels/${channelId}/chat/moderation/filters/${filterId}`, {
        method: "DELETE",
    });
    moderationLoaded = false;
    moderationFilterInvalidations.add(channelId);
    await loadModeration(true);
}

const modal = document.getElementById("modal");
const modalTitle = document.getElementById("modal-title");
const modalBody = document.getElementById("modal-body");
const overviewCards = document.getElementById("overview-cards");
const statusBoard = document.getElementById("status-board");
const overviewIncidents = document.getElementById("overview-incidents");
const overviewStreams = document.getElementById("overview-streams");
const streamsList = document.getElementById("streams-list");
const systemHealthBoard = document.getElementById("system-health-board");
const adminShortcutButtons = Array.from(document.querySelectorAll("#admin-shortcuts button"));
const profileDetail = document.getElementById("profile-detail");
const accountActions = document.getElementById("account-actions");
const accountName = document.getElementById("current-user-name");
const signOutButton = document.getElementById("sign-out-button");
const heroNavButtons = Array.from(document.querySelectorAll(".hero__nav button"));
const themeToggle = document.getElementById("theme-toggle");

const PRIMARY_VIEWS = new Set(["dashboard", "streams", "system-health", "administration"]);

const THEME_STORAGE_KEY = "bitriver-theme";

function setActiveHeroNav(activeView) {
    const primaryView = PRIMARY_VIEWS.has(activeView) ? activeView : "administration";
    for (const button of heroNavButtons) {
        const isActive = button.dataset.view === primaryView;
        button.classList.toggle("is-active", isActive);
        if (isActive) {
            button.setAttribute("aria-current", "page");
        } else {
            button.removeAttribute("aria-current");
        }
    }
}

function readStoredTheme() {
    try {
        return localStorage.getItem(THEME_STORAGE_KEY);
    } catch {
        return null;
    }
}

function applyTheme(theme, { persist = true } = {}) {
    const normalized = theme === "light" ? "light" : "dark";
    document.documentElement.dataset.theme = normalized;
    if (themeToggle) {
        const isLight = normalized === "light";
        themeToggle.setAttribute("aria-pressed", String(isLight));
        themeToggle.setAttribute("aria-label", isLight ? "Switch to dark theme" : "Switch to light theme");
    }
    if (!persist) {
        return;
    }
    try {
        localStorage.setItem(THEME_STORAGE_KEY, normalized);
    } catch {
        // Ignore storage failures (e.g., Safari private mode)
    }
}

function initializeTheme() {
    const storedTheme = readStoredTheme();
    const mediaQuery = typeof window.matchMedia === "function" ? window.matchMedia("(prefers-color-scheme: light)") : null;
    const initialTheme = storedTheme || (mediaQuery?.matches ? "light" : "dark");
    applyTheme(initialTheme, { persist: false });

    if (themeToggle) {
        themeToggle.addEventListener("click", () => {
            const nextTheme = document.documentElement.dataset.theme === "light" ? "dark" : "light";
            applyTheme(nextTheme);
        });
    }

    if (!storedTheme && mediaQuery) {
        const syncWithSystemPreference = (event) => {
            if (readStoredTheme()) {
                return;
            }
            applyTheme(event.matches ? "light" : "dark", { persist: false });
        };
        if (typeof mediaQuery.addEventListener === "function") {
            mediaQuery.addEventListener("change", syncWithSystemPreference);
        } else if (typeof mediaQuery.addListener === "function") {
            mediaQuery.addListener(syncWithSystemPreference);
        }
    }
}

function switchView(id) {
    for (const panel of document.querySelectorAll(".panel")) {
        panel.classList.toggle("active", panel.id === id);
    }
    setActiveHeroNav(id);
}

heroNavButtons.forEach((btn) => {
    btn.addEventListener("click", () => {
        const view = btn.dataset.view;
        switchView(view);
    });
});

adminShortcutButtons.forEach((btn) => {
    btn.addEventListener("click", () => {
        const view = btn.dataset.view;
        switchView(view);
        if (view === "moderation") {
            void loadModeration();
        } else if (view === "analytics") {
            void loadAnalytics();
        } else if (view === "legal") {
            void loadLegal();
        }
    });
});

setActiveHeroNav("dashboard");

initializeTheme();

async function apiRequest(path, options = {}) {
    const headers = new Headers(options.headers || {});
    if (!headers.has("Content-Type")) {
        headers.set("Content-Type", "application/json");
    }
    const response = await fetch(path, {
        ...options,
        headers,
        credentials: "include",
    });
    if (response.status === 204) {
        return null;
    }
    const contentType = response.headers.get("content-type") || "";
    const isJSON = contentType.includes("application/json");
    const payload = isJSON ? await response.json().catch(() => ({})) : null;
    if (!response.ok) {
        if (response.status === 401) {
            throw new UnauthorizedError(payload?.error || response.statusText);
        }
        throw new Error(payload?.error || response.statusText);
    }
    return payload;
}

function resolveDestinationPath() {
    const { pathname, search, hash } = window.location;
    const currentPath = `${pathname}${search}${hash}`;
    if (!currentPath || currentPath.startsWith("/signup")) {
        return "/viewer";
    }
    return currentPath;
}

function redirectToAuth() {
    const next = resolveDestinationPath();
    const params = new URLSearchParams();
    if (next) {
        params.set("next", next);
    }
    const query = params.toString();
    const destination = query ? `/signup?${query}` : "/signup";
    window.location.replace(destination);
}

async function requireSession() {
    try {
        const session = await apiRequest("/api/auth/session");
        return session;
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            redirectToAuth();
        }
        throw error;
    }
}

function renderAccountStatus() {
    if (!accountActions) {
        return;
    }
    if (state.currentUser) {
        accountActions.hidden = false;
        if (accountName) {
            accountName.textContent = `Signed in as ${state.currentUser.displayName}`;
        }
    } else {
        accountActions.hidden = true;
        if (accountName) {
            accountName.textContent = "";
        }
    }
    for (const button of heroNavButtons) {
        if (button.dataset.adminOnly === "true") {
            button.hidden = !isCurrentUserAdmin();
        }
    }
}

async function loadMFAStatus() {
    try {
        const status = await apiRequest("/api/auth/mfa");
        state.mfaStatus = status;
    } catch (error) {
        console.warn("Failed to load MFA status", error);
        state.mfaStatus = null;
    }
    renderMFASettings();
}

function renderMFASettings() {
    const container = document.getElementById("mfa-settings");
    if (!container) {
        return;
    }
    clearElement(container);
    const template = document.getElementById("mfa-settings-template");
    if (!template) {
        return;
    }
    container.appendChild(template.content.cloneNode(true));
    const statusText = container.querySelector("#mfa-status-text");
    const recoveryHint = container.querySelector("#mfa-recovery-hint");
    const actionButton = container.querySelector("#mfa-action-button");
    const status = state.mfaStatus;
    if (!statusText || !recoveryHint || !actionButton) {
        return;
    }
    if (!status) {
        statusText.textContent = "MFA status is unavailable.";
        recoveryHint.textContent = "";
        actionButton.disabled = true;
        return;
    }
    if (status.enabled) {
        statusText.textContent = "MFA is enabled for your account.";
        recoveryHint.textContent = `Recovery codes remaining: ${status.recoveryCodesRemaining ?? 0}.`;
        actionButton.textContent = "Disable MFA";
        actionButton.addEventListener("click", () => openDisableMFA());
    } else {
        statusText.textContent = status.pending
            ? "MFA enrollment is pending verification."
            : "MFA is not enabled for your account.";
        recoveryHint.textContent = status.pending
            ? "Complete setup by verifying a code from your authenticator app."
            : "Enable MFA to protect privileged access.";
        actionButton.textContent = status.pending ? "Verify MFA" : "Enable MFA";
        actionButton.addEventListener("click", () => openEnrollMFA());
    }
}

function renderRecoveryCodes(container, codes) {
    if (!container) {
        return;
    }
    clearElement(container);
    if (!Array.isArray(codes)) {
        return;
    }
    for (const code of codes) {
        const item = createElement("li", { textContent: code });
        container.appendChild(item);
    }
}

function openEnrollMFA() {
    openModal("Enable multi-factor authentication", "mfa-enroll-template", {
        confirmLabel: "Verify",
        onOpen: async (form) => {
            if (!form) {
                return;
            }
            const secretInput = form.querySelector('input[name="secret"]');
            const urlInput = form.querySelector('input[name="otpauthUrl"]');
            const recoveryList = form.querySelector("#mfa-recovery-codes");
            if (secretInput) {
                secretInput.value = "Loading...";
            }
            if (urlInput) {
                urlInput.value = "Loading...";
            }
            try {
                const enrollment = await apiRequest("/api/auth/mfa/enroll", { method: "POST", body: JSON.stringify({}) });
                if (secretInput) {
                    secretInput.value = enrollment.secret || "";
                }
                if (urlInput) {
                    urlInput.value = enrollment.otpauthUrl || "";
                }
                renderRecoveryCodes(recoveryList, enrollment.recoveryCodes || []);
            } catch (error) {
                showToast(error.message, "error");
                if (secretInput) {
                    secretInput.value = "";
                }
                if (urlInput) {
                    urlInput.value = "";
                }
            }
        },
        onSubmit: async (values) => {
            await apiRequest("/api/auth/mfa/verify", {
                method: "POST",
                body: JSON.stringify({ code: values.code }),
            });
            await loadMFAStatus();
            showToast("MFA enabled", "info");
        },
    });
}

function openDisableMFA() {
    openModal("Disable multi-factor authentication", "mfa-disable-template", {
        confirmLabel: "Disable",
        onSubmit: async (values) => {
            await apiRequest("/api/auth/mfa/disable", {
                method: "POST",
                body: JSON.stringify({ code: values.code }),
            });
            await loadMFAStatus();
            showToast("MFA disabled", "info");
        },
    });
}

async function handleSignOut() {
    try {
        await apiRequest("/api/auth/session", { method: "DELETE" });
    } catch (error) {
        console.warn("Failed to revoke session", error);
    } finally {
        state.currentUser = null;
        renderAccountStatus();
        redirectToAuth();
    }
}

function showToast(message, variant = "info") {
    const toast = document.createElement("div");
    toast.className = `toast toast--${variant}`;
    toast.textContent = message;
    document.body.appendChild(toast);
    requestAnimationFrame(() => toast.classList.add("visible"));
    setTimeout(() => {
        toast.classList.remove("visible");
        toast.addEventListener("transitionend", () => toast.remove(), { once: true });
    }, 3400);
}

function openModal(title, templateId, options = {}) {
    const { onSubmit, onOpen, confirmLabel = "Save" } = options;
    modalTitle.textContent = title;
    const template = document.getElementById(templateId);
    modalBody.innerHTML = "";
    modalBody.appendChild(template.content.cloneNode(true));
    const form = modal.querySelector("form");
    const confirmButton = modal.querySelector('button[value="confirm"]');
    if (confirmButton) {
        confirmButton.textContent = confirmLabel;
    }
    if (typeof onOpen === "function") {
        onOpen(form);
    }
    modal.addEventListener(
        "close",
        async () => {
            if (modal.returnValue !== "confirm" || typeof onSubmit !== "function") {
                return;
            }
            try {
                const formData = new FormData(form);
                const values = Object.fromEntries(formData.entries());
                await onSubmit(values, form);
            } catch (error) {
                showToast(error.message, "error");
            }
        },
        { once: true },
    );
    modal.showModal();
}

function confirmAction(message) {
    return window.confirm(message);
}

function formatDate(iso) {
    if (!iso) {
        return "—";
    }
    return new Date(iso).toLocaleString();
}

function formatRelativeTime(date) {
    if (!date) {
        return "—";
    }
    const value = typeof date === "string" ? new Date(date) : date;
    const diffMs = value.getTime() - Date.now();
    const absMs = Math.abs(diffMs);
    const units = [
        { ms: 1000 * 60 * 60 * 24, label: "day" },
        { ms: 1000 * 60 * 60, label: "hour" },
        { ms: 1000 * 60, label: "minute" },
    ];
    for (const unit of units) {
        if (absMs >= unit.ms) {
            const count = Math.round(diffMs / unit.ms);
            const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
            return rtf.format(count, unit.label);
        }
    }
    return "just now";
}

function formatDuration(ms) {
    if (ms <= 0) {
        return "0m";
    }
    const totalMinutes = Math.floor(ms / 60000);
    const hours = Math.floor(totalMinutes / 60);
    const minutes = totalMinutes % 60;
    if (hours > 0) {
        return `${hours}h ${minutes}m`;
    }
    return `${minutes}m`;
}

function formatStatusName(value) {
    if (!value) {
        return "Unknown";
    }
    return value
        .split(/[_\s]+/)
        .filter(Boolean)
        .map((segment) => segment[0].toUpperCase() + segment.slice(1))
        .join(" ");
}

function createStatusBadge(status) {
    const normalized = (status || "degraded").toLowerCase();
    const badge = createElement("span", {
        className: `badge status-badge status-badge--${normalized}`,
        textContent: normalized[0].toUpperCase() + normalized.slice(1),
    });
    return badge;
}

const numberFormatter = new Intl.NumberFormat();

function formatNumber(value) {
    return numberFormatter.format(Number.isFinite(value) ? value : 0);
}

function collectSelectedValues(select) {
    return Array.from(select.selectedOptions).map((option) => option.value);
}

function parseDonationLines(input) {
    const lines = input
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean);
    return lines.map((line) => {
        const [currency, address, note = ""] = line.split("|").map((part) => part.trim());
        if (!currency || !address) {
            throw new Error("Donation entries must include both currency and address");
        }
        return { currency, address, note };
    });
}

function donationLinesFromProfile(profile) {
    if (!profile || !profile.donationAddresses.length) {
        return "";
    }
    return profile.donationAddresses
        .map((item) => {
            const parts = [item.currency, item.address];
            if (item.note) {
                parts.push(item.note);
            }
            return parts.join("|");
        })
        .join("\n");
}

function exportSnapshot() {
    const snapshot = {
        generatedAt: new Date().toISOString(),
        users: state.users,
        channels: state.channels,
        sessions: state.sessions,
        chat: state.chat,
        profiles: state.profiles,
        status: state.statusReport,
    };
    const blob = new Blob([JSON.stringify(snapshot, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `bitriver-live-snapshot-${Date.now()}.json`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
    showToast("Snapshot downloaded");
}

function pruneChannelState() {
    const channelIds = new Set(state.channels.map((channel) => channel.id));
    for (const id of Object.keys(state.sessions)) {
        if (!channelIds.has(id)) {
            delete state.sessions[id];
        }
    }
    for (const id of Object.keys(state.chat)) {
        if (!channelIds.has(id)) {
            delete state.chat[id];
        }
    }
}

async function loadStatus() {
    try {
        const response = await fetch("/api/status", { credentials: "include" });
        const contentType = response.headers.get("content-type") || "";
        const isJSON = contentType.includes("application/json");
        const payload = isJSON ? await response.json().catch(() => null) : null;
        if (!response.ok) {
            throw new Error(payload?.error || response.statusText);
        }
        state.statusReport = payload;
    } catch (error) {
        state.statusReport = {
            status: "down",
            checkedAt: new Date().toISOString(),
            checks: [],
            recentFailures: [],
            logHints: [],
            error: error.message,
        };
        showToast(`Status check failed: ${error.message}`, "error");
    }
    renderDashboard();
    renderSystemHealth();
}

async function loadUsers() {
    state.users = await apiRequest("/api/users");
    renderUsers();
    renderDashboard();
}

function renderUsers() {
    const list = document.getElementById("users-list");
    const empty = document.getElementById("users-empty");
    clearElement(list);
    if (!state.users.length) {
        empty.style.display = "block";
        return;
    }
    empty.style.display = "none";
    for (const user of state.users) {
        const card = createElement("article", { className: "card" });

        const header = createElement("div", { className: "card__header" });
        header.append(
            createElement("h3", { textContent: user.displayName }),
            createElement("span", {
                className: "card__meta",
                textContent: `Joined ${formatRelativeTime(user.createdAt)}`,
            }),
        );
        card.appendChild(header);

        card.appendChild(createElement("div", { className: "card__meta", textContent: user.email }));

        const pillGroup = createElement("div", { className: "pill-group" });
        if (user.roles.length) {
            for (const role of user.roles) {
                pillGroup.appendChild(createElement("span", { className: "pill", textContent: role }));
            }
        } else {
            pillGroup.appendChild(createElement("span", { className: "card__meta", textContent: "viewer" }));
        }
        card.appendChild(pillGroup);

        const actions = createElement("div", { className: "card__actions" });
        actions.append(
            createElement("button", {
                className: "secondary",
                textContent: "Edit",
                dataset: { action: "edit-user", user: user.id },
            }),
            createElement("button", {
                className: "secondary",
                textContent: "Profile",
                dataset: { action: "profile-user", user: user.id },
            }),
            createElement("button", {
                className: "danger",
                textContent: "Remove",
                dataset: { action: "delete-user", user: user.id },
            }),
        );
        card.appendChild(actions);

        list.appendChild(card);
    }

    list.querySelectorAll("[data-action=edit-user]").forEach((btn) => {
        btn.addEventListener("click", () => handleEditUser(btn.dataset.user));
    });
    list.querySelectorAll("[data-action=delete-user]").forEach((btn) => {
        btn.addEventListener("click", () => handleDeleteUser(btn.dataset.user));
    });
    list.querySelectorAll("[data-action=profile-user]").forEach((btn) => {
        btn.addEventListener("click", () => {
            state.selectedProfileId = btn.dataset.user;
            renderProfileDetail(state.selectedProfileId);
            switchView("profiles");
        });
    });
}

async function handleCreateUser() {
    openModal("Create user", "user-form", {
        confirmLabel: "Create",
        onSubmit: async (values) => {
            const payload = {
                displayName: values.displayName.trim(),
                email: values.email.trim(),
                roles: values.roles
                    .split(",")
                    .map((role) => role.trim())
                    .filter(Boolean),
            };
            await apiRequest("/api/users", { method: "POST", body: JSON.stringify(payload) });
            showToast("User created");
            await loadUsers();
            await loadChannels();
        },
    });
}

async function handleEditUser(userId) {
    const user = state.users.find((item) => item.id === userId);
    if (!user) {
        showToast("User not found", "error");
        return;
    }
    openModal(`Edit ${user.displayName}`, "user-form", {
        confirmLabel: "Update",
        onOpen: () => {
            modal.querySelector('[name="displayName"]').value = user.displayName;
            modal.querySelector('[name="email"]').value = user.email;
            modal.querySelector('[name="roles"]').value = user.roles.join(",");
        },
        onSubmit: async (values) => {
            const payload = {
                displayName: values.displayName.trim(),
                email: values.email.trim(),
                roles: values.roles
                    .split(",")
                    .map((role) => role.trim())
                    .filter(Boolean),
            };
            await apiRequest(`/api/users/${userId}`, {
                method: "PATCH",
                body: JSON.stringify(payload),
            });
            showToast("User updated");
            await loadUsers();
            await loadChannels();
            await loadProfiles();
        },
    });
}

async function handleDeleteUser(userId) {
    const user = state.users.find((item) => item.id === userId);
    if (!user) {
        return;
    }
    if (!confirmAction(`Remove ${user.displayName}? This also clears their chats.`)) {
        return;
    }
    await apiRequest(`/api/users/${userId}`, { method: "DELETE" });
    showToast("User removed");
    await loadUsers();
    await loadChannels();
    await loadProfiles();
}

async function loadChannels(options = {}) {
    const { hydrate = false } = options;
    state.channels = await apiRequest("/api/channels");
    pruneChannelState();
    if (hydrate && state.channels.length) {
        await Promise.allSettled(
            state.channels.map((channel) => loadSessionsForChannel(channel.id)),
        );
        await Promise.allSettled(
            state.channels.map((channel) => loadChatHistory(channel.id, 50)),
        );
    }
    if (state.channels.length) {
        await Promise.allSettled(state.channels.map((channel) => loadUploadsForChannel(channel.id)));
    } else {
        state.uploads.clear();
    }
    renderChannels();
    renderStreamControls();
    renderDashboard();
    renderStreams();
    renderSystemHealth();
    renderSessions();
    renderUploads();
    renderChat();
    initChatClient();
    syncChatSubscriptions();
}

async function loadAllUploads() {
    if (!state.channels.length) {
        state.uploads.clear();
        renderUploads();
        return;
    }
    await Promise.allSettled(state.channels.map((channel) => loadUploadsForChannel(channel.id)));
    renderUploads();
}

function renderChannels() {
    const list = document.getElementById("channels-list");
    const empty = document.getElementById("channels-empty");
    clearElement(list);
    if (!state.channels.length) {
        empty.style.display = "block";
        return;
    }
    empty.style.display = "none";
    for (const channel of state.channels) {
        const owner = state.users.find((user) => user.id === channel.ownerId);
        const updated = formatRelativeTime(channel.updatedAt);
        const liveClass = channel.liveState === "live" ? "status-live" : "status-offline";
        const card = createElement("article", { className: "card" });

        const header = createElement("div", { className: "card__header" });
        header.append(
            createElement("h3", { textContent: channel.title }),
            createElement("span", {
                className: "card__meta",
                textContent: channel.category || "General",
            }),
        );
        card.appendChild(header);

        card.appendChild(
            createElement("div", {
                className: "card__meta",
                textContent: `Owner: ${owner ? owner.displayName : channel.ownerId}`,
            }),
        );

        const tagContainer = createElement("div", { className: "pill-group" });
        if (channel.tags.length) {
            for (const tag of channel.tags) {
                tagContainer.appendChild(createElement("span", { className: "pill", textContent: tag }));
            }
        } else {
            tagContainer.appendChild(createElement("span", { className: "card__meta", textContent: "No tags" }));
        }
        card.appendChild(tagContainer);

        const channelMeta = createElement("div", { className: "channel-meta" });
        channelMeta.appendChild(
            createElement("span", {
                className: "card__meta",
                textContent: `Updated ${updated}`,
            }),
        );
        const stateIndicator = createElement("span", { className: "card__meta" });
        stateIndicator.append(
            "State: ",
            createElement("span", { className: liveClass, textContent: channel.liveState }),
        );
        channelMeta.appendChild(stateIndicator);
        card.appendChild(channelMeta);

        if (channel.streamKey) {
            const details = document.createElement("details");
            const summary = createElement("summary", { textContent: "Stream key & ingest tips" });
            details.appendChild(summary);
            const streamKey = createElement("div", { className: "stream-key" });
            const copyButton = createElement("button", {
                className: "secondary",
                textContent: "Copy",
            });
            copyButton.addEventListener("click", async () => {
                try {
                    await navigator.clipboard.writeText(channel.streamKey);
                    showToast("Stream key copied");
                } catch (error) {
                    showToast("Clipboard not available", "error");
                }
            });
            streamKey.append(
                createElement("code", { textContent: channel.streamKey }),
                copyButton,
            );
            details.appendChild(streamKey);
            const ingest = createElement("p", { className: "card__meta" });
            ingest.append(
                "Use ",
                createElement("code", { textContent: "rtmp://YOUR_INGEST_SERVER/live" }),
                " with the key above.",
            );
            details.appendChild(ingest);
            card.appendChild(details);
        } else {
            card.appendChild(
                createElement("p", {
                    className: "card__meta",
                    textContent: "Stream key unavailable for this channel.",
                }),
            );
        }

        const actions = createElement("div", { className: "card__actions" });
        actions.append(
            createElement("button", {
                className: "secondary",
                textContent: "Edit",
                dataset: { action: "edit-channel", channel: channel.id },
            }),
            createElement("button", {
                className: "secondary",
                textContent: "Rotate key",
                dataset: { action: "rotate-stream-key", channel: channel.id },
            }),
            createElement("button", {
                className: "danger",
                textContent: "Delete",
                dataset: { action: "delete-channel", channel: channel.id },
            }),
        );
        card.appendChild(actions);

        list.appendChild(card);
    }

    list.querySelectorAll("[data-action=edit-channel]").forEach((btn) => {
        btn.addEventListener("click", () => handleEditChannel(btn.dataset.channel));
    });
    list.querySelectorAll("[data-action=delete-channel]").forEach((btn) => {
        btn.addEventListener("click", () => handleDeleteChannel(btn.dataset.channel));
    });
    list.querySelectorAll("[data-action=rotate-stream-key]").forEach((btn) => {
        btn.addEventListener("click", () => {
            void rotateStreamKey(btn.dataset.channel);
        });
    });
}

function populateOwnerSelect(select, selected) {
    clearElement(select);
    for (const user of state.users) {
        const option = createElement("option", {
            textContent: user.displayName,
            attributes: { value: user.id },
        });
        if (selected === user.id) {
            option.selected = true;
        }
        select.appendChild(option);
    }
}

function populateChannelSelect(select, selected) {
    clearElement(select);
    for (const channel of state.channels) {
        const option = createElement("option", {
            textContent: channel.title,
            attributes: { value: channel.id },
        });
        if (selected === channel.id) {
            option.selected = true;
        }
        select.appendChild(option);
    }
}

async function handleCreateChannel() {
    if (!state.users.length) {
        showToast("Create a user before provisioning channels", "error");
        return;
    }
    openModal("Create channel", "channel-form", {
        confirmLabel: "Create",
        onOpen: () => {
            const select = modal.querySelector("select[name=ownerId]");
            populateOwnerSelect(select);
        },
        onSubmit: async (values) => {
            const payload = {
                ownerId: values.ownerId,
                title: values.title.trim(),
                category: values.category.trim(),
                tags: values.tags
                    .split(",")
                    .map((tag) => tag.trim())
                    .filter(Boolean),
            };
            await apiRequest("/api/channels", {
                method: "POST",
                body: JSON.stringify(payload),
            });
            showToast("Channel created");
            await loadChannels({ hydrate: true });
            await loadProfiles();
        },
    });
}

async function handleEditChannel(channelId) {
    const channel = state.channels.find((item) => item.id === channelId);
    if (!channel) {
        showToast("Channel not found", "error");
        return;
    }
    openModal(`Edit ${channel.title}`, "channel-form", {
        confirmLabel: "Update",
        onOpen: () => {
            const select = modal.querySelector("select[name=ownerId]");
            populateOwnerSelect(select, channel.ownerId);
            modal.querySelector('[name="title"]').value = channel.title;
            modal.querySelector('[name="category"]').value = channel.category || "";
            modal.querySelector('[name="tags"]').value = channel.tags.join(",");
            select.disabled = true;
        },
        onSubmit: async (values) => {
            const payload = {};
            if (values.title.trim() !== channel.title) {
                payload.title = values.title.trim();
            }
            if ((values.category || "").trim() !== (channel.category || "")) {
                payload.category = values.category.trim();
            }
            const tags = values.tags
                .split(",")
                .map((tag) => tag.trim())
                .filter(Boolean);
            if (tags.join(",") !== channel.tags.join(",")) {
                payload.tags = tags;
            }
            if (!Object.keys(payload).length) {
                showToast("No changes to apply");
                return;
            }
            await apiRequest(`/api/channels/${channelId}`, {
                method: "PATCH",
                body: JSON.stringify(payload),
            });
            showToast("Channel updated");
            await loadChannels({ hydrate: true });
            await loadProfiles();
        },
    });
}

function parseUploadMetadata(input) {
    if (!input) {
        return undefined;
    }
    try {
        const parsed = JSON.parse(input);
        if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
            return parsed;
        }
        throw new Error("Metadata must be a JSON object");
    } catch (error) {
        throw new Error("Metadata must be valid JSON");
    }
}

async function handleCreateUpload() {
    if (!state.channels.length) {
        showToast("Create a channel before uploading", "error");
        return;
    }
    openModal("Upload recording", "upload-form", {
        confirmLabel: "Upload",
        onOpen: (form) => {
            const select = form.querySelector("select[name=channelId]");
            if (select) {
                populateChannelSelect(select);
            }
        },
        onSubmit: async (values, form) => {
            const channelId = values.channelId;
            if (!channelId) {
                throw new Error("Channel is required");
            }
            let metadata;
            if (values.metadata) {
                metadata = parseUploadMetadata(values.metadata);
            }
            const fileInput = form.querySelector('input[name="file"]');
            const file = fileInput && fileInput.files ? fileInput.files[0] : undefined;
            const size = file ? file.size : Number.parseInt(values.sizeBytes || "0", 10) || 0;
            const payload = {
                channelId,
                title: values.title || (file ? file.name : ""),
                filename: values.filename || (file ? file.name : ""),
                sizeBytes: size,
                playbackUrl: values.playbackUrl || "",
                metadata: metadata || undefined,
            };
            await apiRequest("/api/uploads", {
                method: "POST",
                body: JSON.stringify(payload),
            });
            showToast("Upload registered");
            await loadUploadsForChannel(channelId);
        },
    });
}

async function handleDeleteChannel(channelId) {
    const channel = state.channels.find((item) => item.id === channelId);
    if (!channel) {
        return;
    }
    if (!confirmAction(`Delete channel ${channel.title}? Stream sessions and chat logs will be removed.`)) {
        return;
    }
    await apiRequest(`/api/channels/${channelId}`, { method: "DELETE" });
    showToast("Channel deleted");
    await loadChannels({ hydrate: true });
    await loadProfiles();
}

async function handleDeleteUpload(uploadId, channelId) {
    if (!confirmAction("Delete this upload? This cannot be undone.")) {
        return;
    }
    try {
        await apiRequest(`/api/uploads/${uploadId}`, { method: "DELETE" });
        showToast("Upload removed");
        await loadUploadsForChannel(channelId);
    } catch (error) {
        showToast(error.message, "error");
    }
}

async function rotateStreamKey(channelId) {
    const channel = state.channels.find((item) => item.id === channelId);
    if (!channel) {
        showToast("Channel not found", "error");
        return;
    }
    const confirmed = confirmAction(`Rotate the stream key for ${channel.title}? Existing encoders will need the new key.`);
    if (!confirmed) {
        return;
    }
    try {
        const updated = await apiRequest(`/api/channels/${channelId}/stream/rotate`, { method: "POST" });
        showToast("Stream key rotated");
        if (updated) {
            const index = state.channels.findIndex((item) => item.id === channelId);
            if (index !== -1) {
                state.channels[index] = { ...state.channels[index], ...updated };
            }
        }
        renderChannels();
        renderStreamControls();
        renderDashboard();
    } catch (error) {
        showToast(error.message, "error");
    }
}

async function loadSessionsForChannel(channelId) {
    const sessions = await apiRequest(`/api/channels/${channelId}/sessions`);
    state.sessions[channelId] = sessions;
    return sessions;
}

async function loadUploadsForChannel(channelId) {
    const uploads = await apiRequest(`/api/uploads?channelId=${encodeURIComponent(channelId)}`);
    state.uploads.set(channelId, uploads);
    return uploads;
}

function computeSessionDuration(session) {
    const started = new Date(session.startedAt).getTime();
    const ended = session.endedAt ? new Date(session.endedAt).getTime() : Date.now();
    return Math.max(0, ended - started);
}

function renderSessions() {
    const container = document.getElementById("sessions-list");
    clearElement(container);
    const sessions = Object.values(state.sessions).flat();
    if (!sessions.length) {
        container.appendChild(
            createElement("div", {
                className: "empty",
                textContent: "No stream sessions yet.",
            }),
        );
        return;
    }
    const sorted = sessions.sort((a, b) => new Date(b.startedAt) - new Date(a.startedAt));
    for (const session of sorted) {
        const channel = state.channels.find((item) => item.id === session.channelId);
        const duration = formatDuration(computeSessionDuration(session));
        const card = createElement("article", { className: "card" });
        const header = createElement("div", { className: "card__header" });
        header.append(
            createElement("h3", { textContent: channel ? channel.title : session.channelId }),
            createElement("span", {
                className: "card__meta",
                textContent: `Started ${formatDate(session.startedAt)}`,
            }),
        );
        card.appendChild(header);
        card.appendChild(
            createElement("div", {
                className: "card__meta",
                textContent: `Ended: ${session.endedAt ? formatDate(session.endedAt) : "Live"}`,
            }),
        );
        card.appendChild(
            createElement("div", { className: "card__meta", textContent: `Duration: ${duration}` }),
        );
        card.appendChild(
            createElement("div", {
                className: "card__meta",
                textContent: `Peak concurrent viewers: ${session.peakConcurrent}`,
            }),
        );
        card.appendChild(
            createElement("div", {
                className: "card__meta",
                textContent: `Renditions: ${session.renditions.length ? session.renditions.join(", ") : "Source"}`,
            }),
        );
        container.appendChild(card);
    }
}

function renderUploads() {
    const container = document.getElementById("uploads-list");
    const empty = document.getElementById("uploads-empty");
    if (!container || !empty) {
        return;
    }
    clearElement(container);
    let total = 0;
    for (const channel of state.channels) {
        const uploads = state.uploads.get(channel.id) ?? [];
        if (!uploads.length) {
            continue;
        }
        total += uploads.length;
        const section = createElement("section", { className: "card" });
        const header = createElement("div", { className: "card__header" });
        header.append(
            createElement("h3", { textContent: channel.title }),
            createElement("span", {
                className: "card__meta",
                textContent: `${uploads.length} upload${uploads.length === 1 ? "" : "s"}`,
            }),
        );
        section.appendChild(header);

        const table = createElement("div", { className: "table" });
        for (const upload of uploads) {
            const row = createElement("div", { className: "table__row" });
            row.append(
                createElement("div", { className: "table__cell", textContent: upload.title || upload.filename }),
                createElement("div", {
                    className: "table__cell",
                    textContent: upload.status.replace(/_/g, " "),
                }),
                createElement("div", {
                    className: "table__cell",
                    textContent: `${upload.progress}%`,
                }),
                createElement("div", {
                    className: "table__cell",
                    textContent: formatDate(upload.createdAt),
                }),
            );
            const actions = createElement("div", { className: "table__cell" });
            const deleteBtn = createElement("button", {
                className: "danger",
                textContent: "Delete",
                dataset: { uploadId: upload.id, channelId: channel.id },
            });
            deleteBtn.addEventListener("click", () => handleDeleteUpload(upload.id, channel.id));
            actions.appendChild(deleteBtn);
            row.appendChild(actions);
            table.appendChild(row);
        }
        section.appendChild(table);
        container.appendChild(section);
    }
    empty.hidden = total > 0;
}

async function loadChatHistory(channelId, limit = 50) {
    const query = limit ? `?limit=${limit}` : "";
    const messages = await apiRequest(`/api/channels/${channelId}/chat${query}`);
    state.chat[channelId] = messages;
    return messages;
}

function renderChat() {
    const container = document.getElementById("chat-controls");
    clearElement(container);
    if (!state.channels.length) {
        container.appendChild(
            createElement("div", {
                className: "empty",
                textContent: "Add a channel to unlock chat controls.",
            }),
        );
        return;
    }

    for (const channel of state.channels) {
        const messages = state.chat[channel.id] || [];
        const card = createElement("article", { className: "card" });

        const header = createElement("div", { className: "card__header" });
        header.append(
            createElement("h3", { textContent: channel.title }),
            createElement("div", {
                className: "card__meta",
                textContent: `${messages.length} message${messages.length === 1 ? "" : "s"}`,
            }),
        );
        card.appendChild(header);

        const toolbar = createElement("div", { className: "chat-toolbar" });
        toolbar.appendChild(
            createElement("button", {
                className: "secondary",
                textContent: "Refresh",
                dataset: { action: "refresh-chat", channel: channel.id },
            }),
        );
        card.appendChild(toolbar);

        const log = createElement("div", { className: "chat-log" });
        if (messages.length) {
            for (const message of messages) {
                const messageContainer = createElement("div", { className: "chat-message" });
                const messageHeader = createElement("div", { className: "chat-header" });
                messageHeader.append(
                    createElement("strong", { textContent: message.userId }),
                    createElement("span", {
                        className: "card__meta",
                        textContent: formatRelativeTime(message.createdAt),
                    }),
                );
                messageContainer.appendChild(messageHeader);

                messageContainer.appendChild(
                    createElement("div", { textContent: message.content }),
                );

                const messageActions = createElement("div", { className: "chat-actions" });
                messageActions.appendChild(
                    createElement("button", {
                        className: "danger",
                        textContent: "Remove",
                        dataset: {
                            action: "delete-message",
                            channel: channel.id,
                            message: message.id,
                        },
                    }),
                );
                messageContainer.appendChild(messageActions);

                log.appendChild(messageContainer);
            }
        } else {
            log.appendChild(
                createElement("div", {
                    className: "card__meta",
                    textContent: "No chat messages yet.",
                }),
            );
        }
        card.appendChild(log);

        const form = createElement("form", { className: "chat-form", dataset: { channel: channel.id } });
        form.setAttribute("data-channel", channel.id);
        form.setAttribute("novalidate", "");

        const userLabel = document.createElement("label");
        userLabel.append("User");
        const userSelect = document.createElement("select");
        userSelect.name = "userId";
        userSelect.required = true;
        const placeholder = document.createElement("option");
        placeholder.value = "";
        placeholder.disabled = true;
        placeholder.textContent = "Select user";
        if (!state.users.length) {
            placeholder.selected = true;
        }
        userSelect.appendChild(placeholder);
        for (const user of state.users) {
            const option = createElement("option", {
                textContent: user.displayName,
                attributes: { value: user.id },
            });
            userSelect.appendChild(option);
        }
        userLabel.appendChild(userSelect);
        form.appendChild(userLabel);

        const messageLabel = document.createElement("label");
        messageLabel.append("Message");
        const messageInput = document.createElement("input");
        messageInput.type = "text";
        messageInput.name = "content";
        messageInput.required = true;
        messageInput.placeholder = "Say hello";
        messageLabel.appendChild(messageInput);
        form.appendChild(messageLabel);

        form.appendChild(
            createElement("button", { className: "primary", textContent: "Send message", attributes: { type: "submit" } }),
        );

        card.appendChild(form);

        const moderation = createElement("div", { className: "chat-moderation" });
        const moderationLabel = document.createElement("label");
        moderationLabel.append("Moderate user");
        const moderationSelect = document.createElement("select");
        moderationSelect.name = "moderationTarget";
        const moderationPlaceholder = document.createElement("option");
        moderationPlaceholder.value = "";
        moderationPlaceholder.disabled = true;
        moderationPlaceholder.selected = true;
        moderationPlaceholder.textContent = "Select user";
        moderationSelect.appendChild(moderationPlaceholder);
        for (const user of state.users) {
            const option = createElement("option", {
                textContent: user.displayName,
                attributes: { value: user.id },
            });
            moderationSelect.appendChild(option);
        }
        moderationLabel.appendChild(moderationSelect);
        moderation.appendChild(moderationLabel);

        const durationLabel = document.createElement("label");
        durationLabel.append("Timeout (seconds)");
        const durationInput = document.createElement("input");
        durationInput.type = "number";
        durationInput.name = "timeoutSeconds";
        durationInput.min = "5";
        durationInput.value = "60";
        durationLabel.appendChild(durationInput);
        moderation.appendChild(durationLabel);

        const moderationActions = createElement("div", { className: "chat-moderation-actions" });
        const timeoutBtn = createElement("button", { className: "secondary", textContent: "Timeout" });
        timeoutBtn.type = "button";
        const clearTimeoutBtn = createElement("button", { className: "secondary", textContent: "Clear timeout" });
        clearTimeoutBtn.type = "button";
        const banBtn = createElement("button", { className: "danger", textContent: "Ban" });
        banBtn.type = "button";
        const unbanBtn = createElement("button", { className: "secondary", textContent: "Unban" });
        unbanBtn.type = "button";
        moderationActions.append(timeoutBtn, clearTimeoutBtn, banBtn, unbanBtn);
        moderation.appendChild(moderationActions);
        card.appendChild(moderation);

        container.appendChild(card);
    }

    container.querySelectorAll(".chat-form").forEach((form) => {
        form.addEventListener("submit", async (event) => {
            event.preventDefault();
            const channelId = form.dataset.channel;
            const userId = form.elements.userId.value;
            const content = form.elements.content.value.trim();
            if (!userId || !content) {
                return;
            }
            try {
                await sendChatMessage(channelId, userId, content);
                form.reset();
            } catch (error) {
                showToast(error.message, "error");
            }
        });
    });

    container.querySelectorAll(".chat-moderation").forEach((section) => {
        const card = section.closest("article");
        const channelId = card?.querySelector(".chat-form")?.dataset.channel;
        if (!channelId) {
            return;
        }
        const select = section.querySelector("select[name=moderationTarget]");
        const durationInput = section.querySelector("input[name=timeoutSeconds]");
        const buttons = section.querySelectorAll("button");

        const resolveTarget = () => select?.value;

        const handleModeration = async (action) => {
            const targetId = resolveTarget();
            if (!targetId) {
                showToast("Select a user to moderate", "error");
                return;
            }
            try {
                if (action === "timeout") {
                    const seconds = parseInt(durationInput?.value || "60", 10);
                    const durationMs = Number.isFinite(seconds) ? Math.max(seconds, 5) * 1000 : 60000;
                    await sendModerationAction(channelId, "timeout", targetId, durationMs);
                } else if (action === "remove_timeout") {
                    await sendModerationAction(channelId, "remove_timeout", targetId, 0);
                } else {
                    await sendModerationAction(channelId, action, targetId, 0);
                }
            } catch (error) {
                showToast(error.message, "error");
            }
        };

        buttons.forEach((button) => {
            button.addEventListener("click", () => {
                switch (button.textContent) {
                    case "Timeout":
                        handleModeration("timeout");
                        break;
                    case "Clear timeout":
                        handleModeration("remove_timeout");
                        break;
                    case "Ban":
                        handleModeration("ban");
                        break;
                    case "Unban":
                        handleModeration("unban");
                        break;
                    default:
                        break;
                }
            });
        });
    });

    container.querySelectorAll("[data-action=refresh-chat]").forEach((btn) => {
        btn.addEventListener("click", async () => {
            await loadChatHistory(btn.dataset.channel);
            renderChat();
        });
    });

    container.querySelectorAll("[data-action=delete-message]").forEach((btn) => {
        btn.addEventListener("click", async () => {
            if (!confirmAction("Remove this message?")) {
                return;
            }
            await apiRequest(`/api/channels/${btn.dataset.channel}/chat/${btn.dataset.message}`, {
                method: "DELETE",
            });
            showToast("Message removed");
            await loadChatHistory(btn.dataset.channel);
            renderChat();
        });
    });
}

async function loadProfiles() {
    const profiles = await apiRequest("/api/profiles");
    state.profiles = profiles;
    state.profileIndex = new Map(profiles.map((profile) => [profile.userId, profile]));
    if (state.selectedProfileId && !state.profileIndex.has(state.selectedProfileId)) {
        state.selectedProfileId = null;
    }
    renderProfiles();
    renderProfileDetail(state.selectedProfileId);
    renderDashboard();
}

function renderProfiles() {
    const list = document.getElementById("profiles-list");
    clearElement(list);
    if (!state.profiles.length) {
        list.appendChild(
            createElement("div", {
                className: "empty",
                textContent: "Profiles will appear once you create them.",
            }),
        );
        return;
    }
    const sorted = [...state.profiles].sort((a, b) => a.displayName.localeCompare(b.displayName));
    for (const profile of sorted) {
        const liveCount = profile.liveChannels.length;
        const friends = profile.topFriends.length
            ? profile.topFriends.map((friend) => friend.displayName).join(", ")
            : "No top friends yet";
        const card = createElement("article", { className: "card" });

        const header = createElement("div", { className: "card__header" });
        header.append(
            createElement("h3", { textContent: profile.displayName }),
            createElement("span", {
                className: "card__meta",
                textContent: `${profile.channels.length} channel${
                    profile.channels.length === 1 ? "" : "s"
                }`,
            }),
        );
        card.appendChild(header);

        card.appendChild(
            createElement("p", {
                textContent: profile.bio || "No bio yet.",
            }),
        );
        card.appendChild(
            createElement("div", {
                className: "card__meta",
                textContent: `Live now: ${liveCount}`,
            }),
        );
        card.appendChild(
            createElement("div", {
                className: "card__meta",
                textContent: `Top friends: ${friends}`,
            }),
        );

        const actions = createElement("div", { className: "card__actions" });
        actions.append(
            createElement("button", {
                className: "secondary",
                textContent: "View",
                dataset: { action: "view-profile", user: profile.userId },
            }),
            createElement("button", {
                className: "primary",
                textContent: "Edit",
                dataset: { action: "edit-profile", user: profile.userId },
            }),
        );
        card.appendChild(actions);

        list.appendChild(card);
    }

    list.querySelectorAll("[data-action=view-profile]").forEach((btn) => {
        btn.addEventListener("click", () => {
            state.selectedProfileId = btn.dataset.user;
            renderProfileDetail(state.selectedProfileId);
        });
    });
    list.querySelectorAll("[data-action=edit-profile]").forEach((btn) => {
        btn.addEventListener("click", () => openProfileEditor(btn.dataset.user));
    });
}

function renderProfileDetail(userId) {
    if (!profileDetail) {
        return;
    }
    clearElement(profileDetail);
    if (!userId) {
        const header = createElement("div", { className: "card__header" });
        header.append(
            createElement("h3", { textContent: "Profile details" }),
            createElement("span", {
                className: "card__meta",
                textContent: "Select a creator to inspect or edit.",
            }),
        );
        profileDetail.append(
            header,
            createElement("p", { className: "card__meta", textContent: "No profile selected." }),
        );
        return;
    }
    const profile = state.profileIndex.get(userId);
    if (!profile) {
        const header = createElement("div", { className: "card__header" });
        header.append(createElement("h3", { textContent: "Profile details" }));
        profileDetail.append(
            header,
            createElement("p", { className: "card__meta", textContent: "Profile not found." }),
        );
        return;
    }

    const header = createElement("div", { className: "card__header" });
    header.append(
        createElement("h3", { textContent: profile.displayName }),
        createElement("button", {
            className: "secondary",
            textContent: "Edit",
            dataset: { action: "edit-profile", user: profile.userId },
        }),
    );
    profileDetail.appendChild(header);

    profileDetail.appendChild(
        createElement("p", { textContent: profile.bio || "No bio yet." }),
    );

    const friendsSection = createElement("div", { className: "profile-section" });
    friendsSection.append(
        createElement("h4", { textContent: "Top friends" }),
        createElement("p", {
            className: "card__meta",
            textContent: profile.topFriends.length
                ? profile.topFriends.map((friend) => friend.displayName).join(", ")
                : "None",
        }),
    );
    profileDetail.appendChild(friendsSection);

    const channelsSection = createElement("div", { className: "profile-section" });
    channelsSection.append(createElement("h4", { textContent: "Channels" }));
    const channelList = document.createElement("ul");
    if (profile.channels.length) {
        for (const channel of profile.channels) {
            const item = document.createElement("li");
            item.append(
                channel.title,
                " — ",
                createElement("span", {
                    className: "card__meta",
                    textContent: channel.category || "General",
                }),
            );
            channelList.appendChild(item);
        }
    } else {
        channelList.appendChild(
            createElement("li", { className: "card__meta", textContent: "No channels yet." }),
        );
    }
    channelsSection.appendChild(channelList);
    profileDetail.appendChild(channelsSection);

    const donationSection = createElement("div", { className: "profile-section" });
    donationSection.append(createElement("h4", { textContent: "Donation addresses" }));
    const donationList = document.createElement("ul");
    if (profile.donationAddresses.length) {
        for (const addr of profile.donationAddresses) {
            const item = document.createElement("li");
            item.appendChild(createElement("span", { className: "pill", textContent: addr.currency }));
            item.append(" ", addr.address);
            if (addr.note) {
                item.append(` — ${addr.note}`);
            }
            donationList.appendChild(item);
        }
    } else {
        donationList.appendChild(
            createElement("li", {
                className: "card__meta",
                textContent: "No donation links configured.",
            }),
        );
    }
    donationSection.appendChild(donationList);
    profileDetail.appendChild(donationSection);

    profileDetail.querySelectorAll("[data-action=edit-profile]").forEach((btn) => {
        btn.addEventListener("click", () => openProfileEditor(btn.dataset.user));
    });
}

async function openProfileEditor(userId) {
    const profile = await apiRequest(`/api/profiles/${userId}`);
    const user = state.users.find((item) => item.id === userId);
    openModal(`Edit profile${user ? ` — ${user.displayName}` : ""}`, "profile-form", {
        confirmLabel: "Save profile",
        onOpen: () => {
            modal.querySelector('[name="bio"]').value = profile.bio || "";
            modal.querySelector('[name="avatarUrl"]').value = profile.avatarUrl || "";
            modal.querySelector('[name="bannerUrl"]').value = profile.bannerUrl || "";
            modal.querySelector('[name="donationAddresses"]').value = donationLinesFromProfile(profile);

            const featuredSelect = modal.querySelector('[name="featuredChannelId"]');
            clearElement(featuredSelect);
            featuredSelect.appendChild(
                createElement("option", { textContent: "None", attributes: { value: "" } }),
            );
            for (const channel of profile.channels) {
                const option = createElement("option", {
                    textContent: channel.title,
                    attributes: { value: channel.id },
                });
                if (profile.featuredChannelId === channel.id) {
                    option.selected = true;
                }
                featuredSelect.appendChild(option);
            }

            const friendsSelect = modal.querySelector('[name="topFriends"]');
            clearElement(friendsSelect);
            for (const candidate of state.users.filter((candidate) => candidate.id !== userId)) {
                const option = createElement("option", {
                    textContent: candidate.displayName,
                    attributes: { value: candidate.id },
                });
                if (profile.topFriends.some((friend) => friend.userId === candidate.id)) {
                    option.selected = true;
                }
                friendsSelect.appendChild(option);
            }
        },
        onSubmit: async (values, form) => {
            const topFriends = collectSelectedValues(form.querySelector('[name="topFriends"]'));
            if (topFriends.length > 8) {
                throw new Error("Top friends cannot exceed eight entries");
            }
            const payload = {
                bio: values.bio.trim(),
                avatarUrl: values.avatarUrl.trim(),
                bannerUrl: values.bannerUrl.trim(),
                featuredChannelId: values.featuredChannelId,
                topFriends,
                donationAddresses: values.donationAddresses.trim() ? parseDonationLines(values.donationAddresses) : [],
            };
            await apiRequest(`/api/profiles/${userId}`, {
                method: "PUT",
                body: JSON.stringify(payload),
            });
            showToast("Profile saved");
            await loadProfiles();
            state.selectedProfileId = userId;
            renderProfileDetail(userId);
        },
    });
}

function mapStatusOwner(name = "") {
    const normalized = name.toLowerCase();
    if (normalized.includes("srs") || normalized.includes("rtmp") || normalized.includes("ingest")) {
        return "SRS";
    }
    if (normalized.includes("ome") || normalized.includes("origin")) {
        return "OME";
    }
    if (normalized.includes("transcod")) {
        return "Transcoder";
    }
    if (normalized.includes("config") || normalized.includes("env")) {
        return "Configuration";
    }
    return "Platform";
}

function mapStatusImpact(status = "") {
    const normalized = status.toLowerCase();
    if (normalized === "down") {
        return "Hard failure; intervention required.";
    }
    if (normalized === "degraded") {
        return "Partial impact; continue with caution.";
    }
    if (normalized === "disabled") {
        return "Component intentionally disabled.";
    }
    return "Normal operation.";
}

function buildPipelineSummary(report) {
    const checks = Array.isArray(report?.checks) ? report.checks : [];
    const pickStatus = (predicate) => {
        const matches = checks.filter((check) => predicate(check.name || ""));
        if (!matches.length) {
            return "disabled";
        }
        if (matches.some((check) => check.status === "down")) {
            return "down";
        }
        if (matches.some((check) => check.status === "degraded")) {
            return "degraded";
        }
        if (matches.some((check) => check.status === "disabled")) {
            return "disabled";
        }
        return "ready";
    };
    const ingestStatus = pickStatus((name) => /ingest|rtmp|srs/i.test(name));
    const transcoderStatus = pickStatus((name) => /transcod/i.test(name));
    const playbackStatus = pickStatus((name) => /playback|hls|viewer|origin|ome/i.test(name));
    return [
        {
            title: "Ingest",
            status: ingestStatus,
            meaning: ingestStatus === "ready" ? "Ingest healthy; new streams can connect." : "Ingest disruption; new streams may fail.",
            owner: "SRS",
        },
        {
            title: "Transcoder",
            status: transcoderStatus,
            meaning: transcoderStatus === "ready" ? "Transcoder healthy; renditions can be produced." : "Transcoder degraded; renditions may be limited.",
            owner: "Transcoder",
        },
        {
            title: "Playback readiness",
            status: playbackStatus,
            meaning: playbackStatus === "ready" ? "Playback healthy for viewers." : "Playback risk detected; verify viewer playback.",
            owner: "OME",
        },
    ];
}

function renderStatusCheck(check) {
    const item = createElement("article", { className: "status-item" });
    const header = createElement("div", { className: "status-item__header" });
    header.append(
        createElement("h4", { textContent: formatStatusName(check.name) }),
        createStatusBadge(check.status),
    );
    item.appendChild(header);

    item.appendChild(
        createElement("div", {
            className: "card__meta",
            textContent: `${check.category === "ingest" ? "Ingest" : "Core dependency"} · Owned by ${mapStatusOwner(check.name)}`,
        }),
    );

    if (check.detail) {
        item.appendChild(createElement("p", { className: "status-detail", textContent: check.detail }));
    }

    item.appendChild(
        createElement("p", {
            className: "status-remediation",
            textContent: `Next action: ${check.remediation || "Inspect logs to continue triage."}`,
        }),
    );
    item.appendChild(
        createElement("p", {
            className: "status-meta",
            textContent: `Impact: ${mapStatusImpact(check.status)}`,
        }),
    );

    item.appendChild(
        createElement("p", {
            className: "status-meta",
            textContent: check.checkedAt ? `Checked ${formatRelativeTime(check.checkedAt)}` : "Pending check",
        }),
    );

    return item;
}

function renderLogHints(logHints) {
    const container = createElement("div", { className: "log-hints" });
    container.appendChild(createElement("h4", { textContent: "Logs" }));
    if (!logHints?.length) {
        container.appendChild(createElement("p", { className: "card__meta", textContent: "No log references available." }));
        return container;
    }

    const list = createElement("div", { className: "log-hints__list" });
    for (const hint of logHints) {
        const row = createElement("div", { className: "log-hints__item" });
        row.appendChild(createElement("span", { className: "log-hints__label", textContent: hint.label }));
        row.appendChild(createElement("code", { className: "log-hints__command", textContent: hint.command }));
        const copy = createElement("button", { className: "secondary", textContent: "Copy" });
        copy.addEventListener("click", async () => {
            try {
                await navigator.clipboard.writeText(hint.command);
                showToast("Log command copied");
            } catch (error) {
                showToast(`Copy failed: ${error.message}`, "error");
            }
        });
        row.appendChild(copy);
        list.appendChild(row);
    }
    container.appendChild(list);
    return container;
}

function renderStatusBoard() {
    if (!statusBoard) {
        return;
    }
    clearElement(statusBoard);
    const report = state.statusReport;
    const card = createElement("article", { className: "card status-card" });
    const header = createElement("div", { className: "card__header status-card__header" });
    const title = createElement("div");
    title.appendChild(createElement("p", { className: "eyebrow", textContent: "Health" }));
    title.appendChild(createElement("h3", { textContent: "System status" }));
    const subtitle = report?.checkedAt
        ? `Updated ${formatRelativeTime(report.checkedAt)}`
        : "Waiting for the first status check";
    title.appendChild(createElement("p", { className: "card__meta", textContent: subtitle }));
    header.appendChild(title);

    header.appendChild(createStatusBadge(report?.status || "degraded"));

    const actions = createElement("div", { className: "status-card__actions" });
    const refresh = createElement("button", { className: "secondary", textContent: "Refresh status" });
    refresh.addEventListener("click", () => loadStatus());
    actions.appendChild(refresh);
    header.appendChild(actions);
    card.appendChild(header);

    if (report?.error) {
        card.appendChild(createElement("p", { className: "status-error", textContent: report.error }));
    }

    const grid = createElement("div", { className: "status-grid" });
    if (!report || !report.checks?.length) {
        grid.appendChild(
            createElement("p", { className: "card__meta", textContent: "No checks have run yet." }),
        );
    } else {
        for (const check of report.checks) {
            grid.appendChild(renderStatusCheck(check));
        }
    }
    card.appendChild(grid);

    const failures = report?.recentFailures ?? [];
    const failuresSection = createElement("section", { className: "status-failures" });
    failuresSection.appendChild(createElement("h4", { textContent: "Recent failures" }));
    if (!failures.length) {
        failuresSection.appendChild(
            createElement("p", { className: "card__meta", textContent: "All components are ready." }),
        );
    } else {
        const list = document.createElement("ul");
        list.className = "status-failure-list";
        for (const failure of failures) {
            const item = document.createElement("li");
            item.append(
                createStatusBadge(failure.status),
                createElement("span", {
                    textContent: `${formatStatusName(failure.name)}: ${failure.detail || failure.remediation}`,
                }),
            );
            list.appendChild(item);
        }
        failuresSection.appendChild(list);
    }
    card.appendChild(failuresSection);

    card.appendChild(renderLogHints(report?.logHints));

    statusBoard.appendChild(card);
}

function renderOverviewIncidents() {
    if (!overviewIncidents) {
        return;
    }
    clearElement(overviewIncidents);
    const report = state.statusReport;
    const card = createElement("article", { className: "card" });
    card.appendChild(createElement("h3", { textContent: "Active failures" }));

    const failures = report?.recentFailures ?? [];
    if (!failures.length) {
        card.appendChild(createElement("p", { className: "card__meta", textContent: "No active failures detected." }));
    } else {
        const list = createElement("div", { className: "card-column" });
        for (const failure of failures) {
            const row = createElement("article", { className: "status-item" });
            row.append(
                createElement("h4", { textContent: formatStatusName(failure.name) }),
                createElement("p", { className: "status-detail", textContent: `What is broken: ${failure.detail || "Check component logs."}` }),
                createElement("p", { className: "status-meta", textContent: `Owned by: ${mapStatusOwner(failure.name)}` }),
                createElement("p", { className: "status-remediation", textContent: `Operator action: ${failure.remediation || "Inspect logs to continue triage."}` }),
                createElement("p", { className: "status-meta", textContent: "Verify recovery: Refresh status and confirm component returns to ready." }),
            );
            list.appendChild(row);
        }
        card.appendChild(list);
    }
    overviewIncidents.appendChild(card);
}

function renderOverviewStreams() {
    if (!overviewStreams) {
        return;
    }
    clearElement(overviewStreams);
    if (!state.channels.length) {
        overviewStreams.appendChild(createElement("div", { className: "empty", textContent: "No channels available." }));
        return;
    }
    for (const channel of state.channels) {
        const latestSession = (state.sessions[channel.id] || []).slice().sort((a, b) => new Date(b.startedAt) - new Date(a.startedAt))[0];
        const card = createElement("article", { className: "card" });
        card.append(
            createElement("h3", { textContent: channel.title }),
            createElement("p", { className: "card__meta", textContent: `Current state: ${channel.liveState || "idle"}` }),
            createElement("p", { className: "card__meta", textContent: `Started/Updated: ${latestSession ? formatRelativeTime(latestSession.startedAt) : "No sessions yet"}` }),
            createElement("p", { className: "card__meta", textContent: `Health indicator: ${state.statusReport?.status || "unknown"}` }),
        );
        overviewStreams.appendChild(card);
    }
}

function renderStreams() {
    if (!streamsList) {
        return;
    }
    clearElement(streamsList);
    if (!state.channels.length) {
        streamsList.appendChild(createElement("div", { className: "empty", textContent: "No channels to display." }));
        return;
    }
    for (const channel of state.channels) {
        const latestSession = (state.sessions[channel.id] || []).slice().sort((a, b) => new Date(b.startedAt) - new Date(a.startedAt))[0];
        const stateLabel = channel.liveState || "idle";
        const card = createElement("article", { className: "card" });
        card.append(
            createElement("h3", { textContent: channel.title }),
            createElement("p", { className: "card__meta", textContent: `Current lifecycle state: ${stateLabel}` }),
            createElement("p", { className: "card__meta", textContent: `Last state change: ${latestSession ? formatRelativeTime(latestSession.startedAt) : "Not yet available"}` }),
            createElement("p", { className: "card__meta", textContent: "Last state change reason: TODO: verify in code" }),
        );
        const actions = createElement("div", { className: "card__actions" });
        const open = createElement("button", { className: "secondary", textContent: "Open Go Live controls" });
        open.addEventListener("click", () => switchView("stream"));
        actions.appendChild(open);
        card.appendChild(actions);
        streamsList.appendChild(card);
    }
}

function renderSystemHealth() {
    if (!systemHealthBoard) {
        return;
    }
    clearElement(systemHealthBoard);
    const report = state.statusReport;
    const card = createElement("article", { className: "card status-card" });
    card.appendChild(createElement("h3", { textContent: "Component matrix" }));
    const checks = report?.checks ?? [];
    if (!checks.length) {
        card.appendChild(createElement("p", { className: "card__meta", textContent: "No status checks available." }));
    } else {
        const list = createElement("div", { className: "status-grid" });
        for (const check of checks) {
            list.appendChild(renderStatusCheck(check));
        }
        card.appendChild(list);
    }
    card.appendChild(renderLogHints(report?.logHints));
    systemHealthBoard.appendChild(card);
}

function renderDashboard() {
    renderStatusBoard();
    renderOverviewIncidents();
    renderOverviewStreams();
    const sessions = Object.values(state.sessions).flat();
    const totalDuration = sessions.reduce((sum, session) => sum + computeSessionDuration(session), 0);
    const totalPeak = sessions.reduce((sum, session) => sum + session.peakConcurrent, 0);
    const chatCount = Object.values(state.chat).reduce((sum, messages) => sum + (messages?.length || 0), 0);
    const lastSession = sessions.sort((a, b) => new Date(b.startedAt) - new Date(a.startedAt))[0];

    const pipeline = buildPipelineSummary(state.statusReport);
    const cards = [
        ...pipeline.map((item) => ({
            title: item.title,
            value: item.status,
            detail: `${item.meaning} Owner: ${item.owner}`,
        })),
        {
            title: "Users",
            value: state.users.length,
            detail: "Accounts with control center access",
        },
        {
            title: "Channels",
            value: state.channels.length,
            detail: "Spaces ready to go live",
        },
        {
            title: "Live channels",
            value: state.channels.filter((channel) => channel.liveState === "live").length,
            detail: "Currently broadcasting",
        },
        {
            title: "Streaming hours",
            value: (totalDuration / 3_600_000).toFixed(1),
            detail: "Accumulated session runtime",
        },
        {
            title: "Peak concurrents",
            value: totalPeak,
            detail: "Sum across all sessions",
        },
        {
            title: "Chat messages",
            value: chatCount,
            detail: "Moderated from the control center",
        },
        {
            title: "Profiles",
            value: state.profiles.length,
            detail: "Creators with public landing pages",
        },
        {
            title: "Last stream",
            value: lastSession ? formatRelativeTime(lastSession.startedAt) : "—",
            detail: lastSession ? `Channel ${lastSession.channelId}` : "No sessions yet",
        },
    ];

    clearElement(overviewCards);
    for (const cardData of cards) {
        const card = createElement("article", { className: "card" });

        const header = createElement("div", { className: "card__header" });
        const title = createElement("h3", { textContent: cardData.title });
        header.appendChild(title);
        card.appendChild(header);

        const value = createElement("div", {
            className: "card__value",
            textContent: String(cardData.value),
        });
        card.appendChild(value);

        const detail = createElement("div", {
            className: "card__meta",
            textContent: String(cardData.detail),
        });
        card.appendChild(detail);

        overviewCards.appendChild(card);
    }
}

function renderStreamControls() {
    const container = document.getElementById("stream-controls");
    clearElement(container);
    if (!state.channels.length) {
        const emptyState = createElement("div", {
            className: "empty",
            textContent: "Create a channel first to control your live stream.",
        });
        container.appendChild(emptyState);
        return;
    }
    for (const channel of state.channels) {
        const card = createElement("article", { className: "card" });

        const header = createElement("div", { className: "card__header" });
        const title = createElement("h3", { textContent: channel.title });
        const key = createElement("span", {
            className: "card__meta",
            textContent: channel.streamKey || "Stream key unavailable",
        });
        header.append(title, key);
        card.appendChild(header);

        const status = createElement("div", { className: "card__meta" });
        const statusLabel = createElement("strong", {
            className: channel.liveState === "live" ? "status-live" : "status-offline",
            textContent: channel.liveState,
        });
        status.append("State: ", statusLabel);
        card.appendChild(status);

        const form = createElement("form", {
            className: "stream-form",
            dataset: { channel: channel.id },
        });

        const renditionsLabel = createElement("label");
        renditionsLabel.append("Renditions (comma separated)");
        const renditionsInput = createElement("input", {
            attributes: {
                type: "text",
                name: "renditions",
                placeholder: "1080p60,720p30",
            },
        });
        renditionsLabel.appendChild(renditionsInput);
        form.appendChild(renditionsLabel);

        const peakLabel = createElement("label");
        peakLabel.append("Peak concurrent viewers (on stop)");
        const peakInput = createElement("input", {
            attributes: {
                type: "number",
                name: "peakConcurrent",
                min: "0",
                value: "0",
            },
        });
        peakLabel.appendChild(peakInput);
        form.appendChild(peakLabel);

        const actions = createElement("div", { className: "card__actions" });
        const startButton = createElement("button", {
            className: "primary",
            textContent: "Start stream",
            attributes: { type: "submit" },
            dataset: { action: "start" },
        });
        const stopButton = createElement("button", {
            className: "secondary",
            textContent: "Stop stream",
            attributes: { type: "button" },
            dataset: { action: "stop" },
        });
        const rotateButton = createElement("button", {
            className: "secondary",
            textContent: "Rotate key",
            attributes: { type: "button" },
            dataset: { action: "rotate" },
        });
        actions.append(startButton, stopButton, rotateButton);
        form.appendChild(actions);

        card.appendChild(form);
        container.appendChild(card);
    }

    container.querySelectorAll(".stream-form").forEach((form) => {
        const channelId = form.dataset.channel;
        const stopBtn = form.querySelector('[data-action="stop"]');
        const rotateBtn = form.querySelector('[data-action="rotate"]');
        form.addEventListener("submit", async (event) => {
            event.preventDefault();
            const renditions = form.elements.renditions.value
                .split(",")
                .map((item) => item.trim())
                .filter(Boolean);
            try {
                await apiRequest(`/api/channels/${channelId}/stream/start`, {
                    method: "POST",
                    body: JSON.stringify({ renditions }),
                });
                showToast("Stream started");
                await loadChannels({ hydrate: true });
            } catch (error) {
                showToast(error.message, "error");
            }
        });
        stopBtn.addEventListener("click", async () => {
            const peakConcurrent = Number(form.elements.peakConcurrent.value) || 0;
            try {
                await apiRequest(`/api/channels/${channelId}/stream/stop`, {
                    method: "POST",
                    body: JSON.stringify({ peakConcurrent }),
                });
                showToast("Stream stopped");
                await loadChannels({ hydrate: true });
            } catch (error) {
                showToast(error.message, "error");
            }
        });
        rotateBtn.addEventListener("click", () => {
            void rotateStreamKey(channelId);
        });
    });
}

function renderSetupWizard() {
    const container = document.getElementById("setup-wizard");
    if (!container) {
        return;
    }
    clearElement(container);
    const template = document.getElementById("setup-wizard-template");
    if (!template) {
        return;
    }
    if (!isCurrentUserAdmin()) {
        const notice = createElement("p", {
            className: "setup-wizard__status",
            textContent: "The setup wizard is restricted to administrators.",
        });
        notice.dataset.state = "error";
        container.appendChild(notice);
        return;
    }
    container.appendChild(template.content.cloneNode(true));
    const form = container.querySelector("#setup-wizard-form");
    const status = container.querySelector("#setup-wizard-status");
    const submit = container.querySelector("#setup-wizard-submit");
    const setStatus = (state, message) => {
        if (!status) {
            return;
        }
        status.dataset.state = state;
        status.textContent = message || "";
    };
    setStatus("idle", "Provide production-ready values. Saving will restart the service.");
    form?.addEventListener("submit", async (event) => {
        event.preventDefault();
        if (!form) {
            return;
        }
        const formData = new FormData(form);
        const apiPort = Number(formData.get("apiPort"));
        const payload = {
            adminEmail: formData.get("adminEmail")?.toString().trim() ?? "",
            adminPassword: formData.get("adminPassword")?.toString().trim() ?? "",
            viewerUrl: formData.get("viewerUrl")?.toString().trim() ?? "",
            publicApiUrl: formData.get("publicApiUrl")?.toString().trim() ?? "",
            viewerOrigin: formData.get("viewerOrigin")?.toString().trim() ?? "",
            apiPort: Number.isFinite(apiPort) ? apiPort : 0,
            tlsCertPath: formData.get("tlsCertPath")?.toString().trim() ?? "",
            tlsKeyPath: formData.get("tlsKeyPath")?.toString().trim() ?? "",
            postgresPassword: formData.get("postgresPassword")?.toString().trim() ?? "",
            redisPassword: formData.get("redisPassword")?.toString().trim() ?? "",
            metricsToken: formData.get("metricsToken")?.toString().trim() ?? "",
            srsToken: formData.get("srsToken")?.toString().trim() ?? "",
            omeToken: formData.get("omeToken")?.toString().trim() ?? "",
            transcoderToken: formData.get("transcoderToken")?.toString().trim() ?? "",
        };

        for (const key of ["adminPassword", "publicApiUrl", "viewerOrigin", "tlsCertPath", "tlsKeyPath", "metricsToken"]) {
            if (!payload[key]) {
                delete payload[key];
            }
        }

        setStatus("pending", "Saving configuration and scheduling a restart...");
        if (submit) {
            submit.disabled = true;
        }
        try {
            await apiRequest("/api/setup", { method: "POST", body: JSON.stringify(payload) });
            setStatus("ready", "Configuration saved. The service will restart shortly.");
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            setStatus("error", message);
            if (submit) {
                submit.disabled = false;
            }
        }
    });
}

function computeInstallerScript(data) {
    const mode = data.mode || "production";
    const addr = data.addr || (mode === "production" ? ":80" : ":8080");
    const logDir = data.enableLogs ? `${data.dataDir}/logs` : "";
    // Keep this URL in sync with docs/installing-on-ubuntu.md and deploy/install/ubuntu.sh.
    const scriptURL = "https://raw.githubusercontent.com/BitRiver-Live/BitRiver-Live/main/deploy/install/ubuntu.sh";
    const hostnameHint = data.hostname
        ? `# Reverse proxy hint: point ${data.hostname} to this service and expose TLS traffic on 443.`
        : `# Configure your reverse proxy or tailnet to expose the service. ${mode === "production" ? "Port 80 is used by default." : "Development mode keeps the control center on :8080."}`;
    const flags = [
        "--install-dir \"$INSTALL_DIR\"",
        "--data-dir \"$DATA_DIR\"",
        "--service-user \"$SERVICE_USER\"",
        "--mode \"$MODE\"",
        "--addr \"$ADDR\"",
    ];
    if (logDir) {
        flags.push("--enable-logs");
        flags.push("--log-dir \"$LOG_DIR\"");
    }
    if (data.tlsCert) {
        flags.push("--tls-cert \"$TLS_CERT\"");
    }
    if (data.tlsKey) {
        flags.push("--tls-key \"$TLS_KEY\"");
    }
    if (data.rateGlobalRps) {
        flags.push("--rate-global-rps \"$RATE_GLOBAL_RPS\"");
    }
    if (data.rateLoginLimit) {
        flags.push("--rate-login-limit \"$RATE_LOGIN_LIMIT\"");
    }
    if (data.rateLoginWindow) {
        flags.push("--rate-login-window \"$RATE_LOGIN_WINDOW\"");
    }
    if (data.redisAddr) {
        flags.push("--redis-addr \"$REDIS_ADDR\"");
    }
    if (data.redisPassword) {
        flags.push("--redis-password \"$REDIS_PASSWORD\"");
    }
    if (data.hostname) {
        flags.push("--hostname \"$HOSTNAME\"");
    }
    const flagBlock = flags
        .map((flag, index) => {
            const suffix = index === flags.length - 1 ? '' : ' \\';
            return `  ${flag}${suffix}`;
        })
        .join("\n");
    return `#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${data.installDir}"
DATA_DIR="${data.dataDir}"
SERVICE_USER="${data.serviceUser}"
MODE="${mode}"
ADDR="${addr}"
LOG_DIR="${logDir}"
TLS_CERT="${data.tlsCert || ""}"
TLS_KEY="${data.tlsKey || ""}"
RATE_GLOBAL_RPS="${data.rateGlobalRps || ""}"
RATE_LOGIN_LIMIT="${data.rateLoginLimit || ""}"
RATE_LOGIN_WINDOW="${data.rateLoginWindow || ""}"
REDIS_ADDR="${data.redisAddr || ""}"
REDIS_PASSWORD="${data.redisPassword || ""}"
HOSTNAME="${data.hostname || ""}"
SCRIPT_URL="${scriptURL}"
SCRIPT_PATH="$(mktemp)"

trap 'rm -f "$SCRIPT_PATH"' EXIT

curl -fsSL "$SCRIPT_URL" -o "$SCRIPT_PATH"
chmod +x "$SCRIPT_PATH"
"$SCRIPT_PATH" \
${flagBlock}

${hostnameHint}
`;
}

function setupInstaller() {
    const container = document.getElementById("installer");
    container.innerHTML = "";
    const template = document.getElementById("installer-template");
    container.appendChild(template.content.cloneNode(true));
    const form = container.querySelector("#installer-form");
    const output = container.querySelector("#installer-output");
    const copyButton = container.querySelector("#copy-installer");
    const status = container.querySelector("#installer-status");
    const setStatus = (state, message) => {
        if (!status) {
            return;
        }
        if (state) {
            status.dataset.state = state;
        }
        if (typeof message === "string") {
            status.textContent = message;
        }
    };
    setStatus("idle");
    if (copyButton) {
        copyButton.disabled = true;
    }
    const summaryElements = {
        installDir: container.querySelector('[data-summary="installDir"]'),
        dataDir: container.querySelector('[data-summary="dataDir"]'),
        serviceUser: container.querySelector('[data-summary="serviceUser"]'),
        mode: container.querySelector('[data-summary="mode"]'),
        addr: container.querySelector('[data-summary="addr"]'),
        hostname: container.querySelector('[data-summary="hostname"]'),
        enableLogs: container.querySelector('[data-summary="enableLogs"]'),
        logDir: container.querySelector('[data-summary="logDir"]'),
    };
    const updateSummary = () => {
        if (!form) {
            return;
        }
        const formData = new FormData(form);
        const data = Object.fromEntries(formData.entries());
        const enableLogs = Boolean(form.elements.enableLogs?.checked);
        const mode = data.mode || "production";
        const addr = data.addr || (mode === "production" ? ":80" : ":8080");
        const dataDir = data.dataDir || "/var/lib/bitriver-live";
        const normalizedDataDir = dataDir.endsWith("/") ? dataDir.slice(0, -1) : dataDir;
        const logDir = enableLogs ? `${normalizedDataDir}/logs` : "Not provisioned";
        if (summaryElements.installDir) {
            summaryElements.installDir.textContent = data.installDir || "/opt/bitriver-live";
        }
        if (summaryElements.dataDir) {
            summaryElements.dataDir.textContent = dataDir;
        }
        if (summaryElements.serviceUser) {
            summaryElements.serviceUser.textContent = data.serviceUser || "bitriver";
        }
        if (summaryElements.mode) {
            summaryElements.mode.textContent =
                mode === "production" ? "Production (port 80)" : "Development (:8080)";
        }
        if (summaryElements.addr) {
            summaryElements.addr.textContent = addr;
        }
        if (summaryElements.hostname) {
            summaryElements.hostname.textContent = data.hostname || "Not provided";
        }
        if (summaryElements.enableLogs) {
            summaryElements.enableLogs.textContent = enableLogs ? "Provisioned" : "Skipped";
        }
        if (summaryElements.logDir) {
            summaryElements.logDir.textContent = enableLogs ? logDir : "Not provisioned";
        }
    };
    const modeField = form.elements.mode;
    const addrField = form.elements.addr;
    if (modeField && addrField) {
        let manualOverride = false;
        const syncAddress = () => {
            if (manualOverride) {
                return;
            }
            addrField.value = modeField.value === "production" ? ":80" : ":8080";
        };
        modeField.addEventListener("change", syncAddress);
        addrField.addEventListener("input", () => {
            manualOverride = true;
        });
        syncAddress();
    }
    const handleDirty = () => {
        updateSummary();
        if (!status) {
            return;
        }
        const currentState = status.dataset.state;
        if (currentState === "ready" || currentState === "copied") {
            setStatus("dirty", "Form updated — generate a fresh script so everything stays in sync.");
            if (copyButton) {
                copyButton.disabled = true;
            }
        }
    };
    form.addEventListener("input", handleDirty);
    form.addEventListener("change", handleDirty);
    updateSummary();
    form.addEventListener("submit", (event) => {
        event.preventDefault();
        const formData = new FormData(form);
        const data = Object.fromEntries(formData.entries());
        data.enableLogs = form.elements.enableLogs.checked;
        data.mode = data.mode || "production";
        data.addr = data.addr || (data.mode === "production" ? ":80" : ":8080");
        const script = computeInstallerScript(data);
        output.value = script;
        output.focus();
        output.select();
        output.scrollTop = 0;
        setStatus("ready", "Script ready. Copy it and run on your Ubuntu server.");
        if (copyButton) {
            copyButton.disabled = false;
            copyButton.focus();
        }
        showToast("Installer script ready. Copy it and run on your server.");
    });
    if (copyButton) {
        copyButton.addEventListener("click", async () => {
            if (!output?.value) {
                return;
            }
            try {
                await navigator.clipboard.writeText(output.value);
                showToast("Installer script copied! Paste it into your server terminal.");
                setStatus("copied", "Script copied. Paste it into your server terminal and press Enter.");
            } catch (error) {
                showToast("Copy failed. Press Ctrl+C / ⌘+C to copy manually.", "error");
                setStatus("ready", "Script ready. Copy it manually if the button does not work.");
            }
        });
    }
}

async function loadModeration(force = false) {
    if (moderationLoaded && !force) {
        renderModeration();
        return;
    }
    try {
        const queueParams = new URLSearchParams({ limit: "50", actionsLimit: "20" });
        const automodParams = new URLSearchParams({ limit: "50" });
        const shouldAppend = moderationLoaded && force;
        if (shouldAppend && state.moderation.queueMeta?.nextCursor) {
            queueParams.set("cursor", state.moderation.queueMeta.nextCursor);
        }
        if (shouldAppend && state.moderation.actionsMeta?.nextCursor) {
            queueParams.set("actionsCursor", state.moderation.actionsMeta.nextCursor);
        }
        if (shouldAppend && state.moderation.automodMeta?.nextCursor) {
            automodParams.set("cursor", state.moderation.automodMeta.nextCursor);
        }
        const [queuePayload, automodPayload, appealsPayload] = await Promise.all([
            apiRequest(`/api/moderation/queue?${queueParams.toString()}`),
            apiRequest(`/api/moderation/automod?${automodParams.toString()}`),
            apiRequest(`/api/channels/${state.channels?.[0]?.id || ""}/chat/moderation/appeals?status=all`).catch(() => []),
        ]);
        const queue = Array.isArray(queuePayload?.queue) ? queuePayload.queue : [];
        const actions = Array.isArray(queuePayload?.actions) ? queuePayload.actions : [];
        const automod = Array.isArray(automodPayload?.actions) ? automodPayload.actions : [];
        state.moderation.queue = shouldAppend && queueParams.has("cursor") ? state.moderation.queue.concat(queue) : queue;
        state.moderation.actions =
            shouldAppend && queueParams.has("actionsCursor") ? state.moderation.actions.concat(actions) : actions;
        state.moderation.automod =
            shouldAppend && automodParams.has("cursor") ? state.moderation.automod.concat(automod) : automod;
        state.moderation.appeals = Array.isArray(appealsPayload) ? appealsPayload : [];
        state.moderation.queueMeta = queuePayload?.queueMeta ?? null;
        state.moderation.actionsMeta = queuePayload?.actionsMeta ?? null;
        state.moderation.automodMeta = automodPayload?.meta ?? null;
        const filters = {};
        if (Array.isArray(state.channels) && state.channels.length) {
            const channelIds = new Set(state.channels.map((channel) => channel.id));
            for (const channelId of moderationFilterCache.keys()) {
                if (!channelIds.has(channelId)) {
                    moderationFilterCache.delete(channelId);
                    moderationFilterInvalidations.delete(channelId);
                }
            }
            await Promise.all(
                state.channels.map(async (channel) => {
                    try {
                        if (moderationFilterCache.has(channel.id) && !moderationFilterInvalidations.has(channel.id)) {
                            filters[channel.id] = moderationFilterCache.get(channel.id);
                            return;
                        }
                        const response = await apiRequest(`/api/channels/${channel.id}/chat/moderation/filters`);
                        const entries = Array.isArray(response) ? response : [];
                        moderationFilterCache.set(channel.id, entries);
                        moderationFilterInvalidations.delete(channel.id);
                        filters[channel.id] = entries;
                    } catch (error) {
                        console.warn("Failed to load filters", error);
                        filters[channel.id] = [];
                    }
                }),
            );
        }
        state.moderation.filters = filters;
        moderationLoaded = true;
        renderModeration();
    } catch (error) {
        showToast(error.message, "error");
    }
}

async function resolveModerationFlag(flagId, resolution) {
    if (!flagId) {
        return;
    }
    await apiRequest(`/api/moderation/queue/${flagId}`, {
        method: "POST",
        body: JSON.stringify({ resolution }),
    });
    showToast(resolution === "ban" ? "Viewer banned" : "Flag dismissed");
    moderationLoaded = false;
    state.moderation.queueMeta = null;
    state.moderation.actionsMeta = null;
    await loadModeration(true);
}

function renderModeration() {
    const queueContainer = document.getElementById("moderation-queue");
    const historyContainer = document.getElementById("moderation-history");
    const filtersContainer = document.getElementById("moderation-filters");
    const automodContainer = document.getElementById("moderation-automod");
    const appealsContainer = document.getElementById("moderation-appeals");
    if (!queueContainer || !historyContainer || !filtersContainer || !automodContainer) {
        return;
    }
    clearElement(queueContainer);
    clearElement(historyContainer);
    clearElement(filtersContainer);
    clearElement(automodContainer);
    clearElement(appealsContainer);

    const queue = state.moderation.queue;
    if (!queue.length) {
        queueContainer.appendChild(
            createElement("div", {
                className: "empty",
                textContent: "No flagged chat messages. Your community is in good shape.",
            }),
        );
    } else {
        for (const flag of queue) {
            const card = createElement("article", { className: "card" });
            const header = createElement("div", { className: "card__header" });
            header.append(
                createElement("h3", { textContent: flag.channelTitle || flag.channelId }),
                createElement("span", {
                    className: "card__meta",
                    textContent: `Flagged ${formatRelativeTime(flag.flaggedAt || flag.createdAt)}`,
                }),
            );
            card.appendChild(header);

            card.appendChild(
                createElement("p", {
                    className: "card__meta",
                    textContent: `Reporter: ${flag.reporter?.displayName || "Auto mod"}`,
                }),
            );

            if (flag.message) {
                card.appendChild(
                    createElement("blockquote", {
                        className: "moderation-quote",
                        textContent: flag.message,
                    }),
                );
            }

            card.appendChild(
                createElement("div", {
                    className: "card__meta",
                    textContent: `Reason: ${flag.reason || "Manual review"}`,
                }),
            );

            const actions = createElement("div", { className: "card__actions" });
            actions.append(
                createElement("button", {
                    className: "secondary",
                    textContent: "Dismiss",
                    dataset: { action: "resolve-flag", id: flag.id, resolution: "dismiss" },
                }),
                createElement("button", {
                    className: "danger",
                    textContent: "Ban & purge",
                    dataset: { action: "resolve-flag", id: flag.id, resolution: "ban" },
                }),
            );
            card.appendChild(actions);

            queueContainer.appendChild(card);
        }

        queueContainer.querySelectorAll("[data-action=resolve-flag]").forEach((button) => {
            button.addEventListener("click", async () => {
                const { id, resolution } = button.dataset;
                try {
                    await resolveModerationFlag(id, resolution);
                } catch (error) {
                    showToast(error.message, "error");
                }
            });
        });
    }

    const actions = state.moderation.actions;
    if (!actions.length) {
        historyContainer.appendChild(
            createElement("div", {
                className: "empty",
                textContent: "No recent moderation actions.",
            }),
        );
    } else {
        for (const entry of actions) {
            const card = createElement("article", { className: "card" });
            card.appendChild(
                createElement("div", {
                    className: "card__header",
                    textContent: `${entry.moderator?.displayName || "System"} → ${entry.action?.replace(/_/g, " ")}`,
                }),
            );
            card.appendChild(
                createElement("div", {
                    className: "card__meta",
                    textContent: `Target: ${entry.targetId || "unknown"}`,
                }),
            );
            card.appendChild(
                createElement("div", {
                    className: "card__meta",
                    textContent: formatDate(entry.createdAt),
                }),
            );
            historyContainer.appendChild(card);
        }
    }


    const appeals = state.moderation.appeals || [];
    if (!appeals.length) {
        appealsContainer.appendChild(createElement("div", { className: "empty", textContent: "No appeals yet." }));
    } else {
        for (const appeal of appeals) {
            const card = createElement("article", { className: "card" });
            card.append(
                createElement("h3", { textContent: `Appeal ${appeal.id}` }),
                createElement("div", { className: "card__meta", textContent: `Status: ${appeal.status}` }),
                createElement("div", { className: "card__meta", textContent: `Reason: ${appeal.reason || "n/a"}` }),
            );
            const actions = createElement("div", { className: "card__actions" });
            actions.append(
                createElement("button", { className: "secondary", textContent: "Resolve", dataset: { action: "resolve-appeal", id: appeal.id } }),
                createElement("button", { className: "danger", textContent: "Deny", dataset: { action: "deny-appeal", id: appeal.id } }),
                createElement("button", { className: "secondary", textContent: "Reopen", dataset: { action: "reopen-appeal", id: appeal.id } }),
            );
            card.appendChild(actions);
            appealsContainer.appendChild(card);
        }
    }

    const automodActions = state.moderation.automod || [];
    if (!automodActions.length) {
        automodContainer.appendChild(
            createElement("div", {
                className: "empty",
                textContent: "No automod actions yet.",
            }),
        );
    } else {
        for (const entry of automodActions) {
            const card = createElement("article", { className: "card" });
            const header = createElement("div", { className: "card__header" });
            header.append(
                createElement("h3", { textContent: entry.channelTitle || entry.channelId }),
                createElement("span", {
                    className: "card__meta",
                    textContent: formatRelativeTime(entry.createdAt),
                }),
            );
            card.appendChild(header);
            card.appendChild(
                createElement("div", {
                    className: "card__meta",
                    textContent: `User: ${entry.user?.displayName || entry.userId || "unknown"}`,
                }),
            );
            if (entry.filterPattern) {
                card.appendChild(
                    createElement("div", {
                        className: "card__meta",
                        textContent: `Filter: ${entry.filterKind || "unknown"} · ${entry.filterPattern}`,
                    }),
                );
            }
            if (entry.message) {
                card.appendChild(
                    createElement("blockquote", {
                        className: "moderation-quote",
                        textContent: entry.message,
                    }),
                );
            }
            card.appendChild(
                createElement("div", {
                    className: "card__meta",
                    textContent: `Action: ${entry.action || "blocked"}`,
                }),
            );
            automodContainer.appendChild(card);
        }
    }

    const channels = Array.isArray(state.channels) ? state.channels : [];
    if (!channels.length) {
        filtersContainer.appendChild(
            createElement("div", {
                className: "empty",
                textContent: "Create a channel to configure automod filters.",
            }),
        );
    } else {
        for (const channel of channels) {
            const card = createElement("article", { className: "card" });
            const header = createElement("div", { className: "card__header" });
            header.append(
                createElement("h3", { textContent: channel.title }),
                createElement("span", { className: "card__meta", textContent: channel.id }),
            );
            card.appendChild(header);

            const filters = state.moderation.filters?.[channel.id] || [];
            if (!filters.length) {
                card.appendChild(
                    createElement("div", {
                        className: "card__meta",
                        textContent: "No filters configured yet.",
                    }),
                );
            } else {
                for (const filter of filters) {
                    const row = createElement("div", { className: "card__meta" });
                    row.textContent = `${filter.kind.toUpperCase()} · ${filter.pattern}`;
                    const badge = createElement("span", {
                        className: "badge",
                        textContent: filter.enabled ? "Enabled" : "Disabled",
                    });
                    row.appendChild(badge);
                    card.appendChild(row);

                    const actions = createElement("div", { className: "card__actions" });
                    actions.append(
                        createElement("button", {
                            className: "secondary",
                            textContent: filter.enabled ? "Disable" : "Enable",
                            dataset: {
                                action: "toggle-filter",
                                channel: channel.id,
                                id: filter.id,
                                enabled: String(!filter.enabled),
                            },
                        }),
                        createElement("button", {
                            className: "danger",
                            textContent: "Delete",
                            dataset: { action: "delete-filter", channel: channel.id, id: filter.id },
                        }),
                    );
                    card.appendChild(actions);
                }
            }

            const form = createElement("form", { dataset: { channel: channel.id } });
            form.className = "filter-form";
            const kindLabel = document.createElement("label");
            kindLabel.append("Filter type");
            const kindSelect = document.createElement("select");
            kindSelect.name = "kind";
            kindSelect.append(
                createElement("option", { textContent: "Word", attributes: { value: "word" } }),
                createElement("option", { textContent: "Regex", attributes: { value: "regex" } }),
            );
            kindLabel.appendChild(kindSelect);
            form.appendChild(kindLabel);

            const patternLabel = document.createElement("label");
            patternLabel.append("Pattern");
            const patternInput = document.createElement("input");
            patternInput.name = "pattern";
            patternInput.placeholder = "spam phrase or regex";
            patternLabel.appendChild(patternInput);
            form.appendChild(patternLabel);

            const enabledLabel = document.createElement("label");
            enabledLabel.append("Enabled");
            const enabledCheckbox = document.createElement("input");
            enabledCheckbox.type = "checkbox";
            enabledCheckbox.name = "enabled";
            enabledCheckbox.checked = true;
            enabledLabel.appendChild(enabledCheckbox);
            form.appendChild(enabledLabel);

            form.appendChild(
                createElement("button", {
                    className: "primary",
                    textContent: "Add filter",
                    attributes: { type: "submit" },
                }),
            );
            card.appendChild(form);

            filtersContainer.appendChild(card);
        }
    }

    filtersContainer.querySelectorAll("form.filter-form").forEach((form) => {
        form.addEventListener("submit", async (event) => {
            event.preventDefault();
            const channelId = form.dataset.channel;
            const kind = form.elements.kind.value;
            const pattern = form.elements.pattern.value.trim();
            const enabled = form.elements.enabled.checked;
            if (!channelId || !pattern) {
                showToast("Provide a filter pattern", "error");
                return;
            }
            try {
                await createChatFilter(channelId, { kind, pattern, enabled });
                form.reset();
                form.elements.enabled.checked = true;
            } catch (error) {
                showToast(error.message, "error");
            }
        });
    });

    filtersContainer.querySelectorAll("[data-action=toggle-filter]").forEach((button) => {
        button.addEventListener("click", async () => {
            const { channel, id, enabled } = button.dataset;
            try {
                await updateChatFilter(channel, id, { enabled: enabled === "true" });
            } catch (error) {
                showToast(error.message, "error");
            }
        });
    });

    filtersContainer.querySelectorAll("[data-action=delete-filter]").forEach((button) => {
        button.addEventListener("click", async () => {
            const { channel, id } = button.dataset;
            if (!confirmAction("Delete this filter?")) {
                return;
            }
            try {
                await deleteChatFilter(channel, id);
            } catch (error) {
                showToast(error.message, "error");
            }
        });
    });
}

async function loadAnalytics(force = false) {
    if (analyticsLoaded && !force) {
        renderAnalytics();
        return;
    }
    try {
        const payload = await apiRequest("/api/analytics/overview");
        state.analytics.summary = payload?.summary || null;
        state.analytics.perChannel = Array.isArray(payload?.perChannel) ? payload.perChannel : [];
        analyticsLoaded = true;
        renderAnalytics();
    } catch (error) {
        showToast(error.message, "error");
    }
}

function renderAnalytics() {
    const overview = document.getElementById("analytics-overview");
    const breakdown = document.getElementById("analytics-breakdown");
    if (!overview || !breakdown) {
        return;
    }
    clearElement(overview);
    clearElement(breakdown);

    const summary = state.analytics.summary;
    if (summary) {
        const metrics = [
            {
                label: "Live viewers",
                value: formatNumber(summary.liveViewers ?? 0),
                detail: "Across all active channels",
            },
            {
                label: "Streams live",
                value: formatNumber(summary.streamsLive ?? 0),
                detail: "Currently broadcasting",
            },
            {
                label: "Daily watch time",
                value: `${formatNumber(Math.round(summary.watchTimeMinutes ?? 0))} min`,
                detail: "Rolling 24 hours",
            },
            {
                label: "Chat messages",
                value: formatNumber(summary.chatMessages ?? 0),
                detail: "Today",
            },
        ];
        for (const metric of metrics) {
            const card = createElement("article", { className: "card" });
            card.appendChild(createElement("h3", { textContent: metric.label }));
            card.appendChild(createElement("div", { className: "metric-value", textContent: metric.value }));
            card.appendChild(createElement("div", { className: "card__meta", textContent: metric.detail }));
            overview.appendChild(card);
        }
    } else {
        overview.appendChild(
            createElement("div", {
                className: "empty",
                textContent: "Analytics will appear after your first stream.",
            }),
        );
    }

    const rows = state.analytics.perChannel || [];
    if (!rows.length) {
        breakdown.appendChild(
            createElement("div", {
                className: "empty",
                textContent: "No channel analytics available yet.",
            }),
        );
        return;
    }

    const table = document.createElement("table");
    table.className = "analytics-table";
    const head = document.createElement("thead");
    head.innerHTML = `
        <tr>
            <th scope="col">Channel</th>
            <th scope="col">Live viewers</th>
            <th scope="col">Followers</th>
            <th scope="col">Avg. watch</th>
            <th scope="col">Chat msgs</th>
        </tr>
    `;
    table.appendChild(head);

    const body = document.createElement("tbody");
    for (const entry of rows) {
        const row = document.createElement("tr");
        const avgWatch = entry.avgWatchMinutes ?? 0;
        row.innerHTML = `
            <th scope="row">${escapeHTML(entry.title || entry.channelId)}</th>
            <td>${formatNumber(entry.liveViewers ?? 0)}</td>
            <td>${formatNumber(entry.followers ?? 0)}</td>
            <td>${formatNumber(Math.round(avgWatch))} min</td>
            <td>${formatNumber(entry.chatMessages ?? 0)}</td>
        `;
        body.appendChild(row);
    }
    table.appendChild(body);
    breakdown.appendChild(table);
}

async function refreshAll() {
    await Promise.all([loadStatus(), loadUsers(), loadProfiles(), loadAnalytics(true), loadMFAStatus()]);
    await loadChannels({ hydrate: true });
    await loadModeration(true);
}

function attachActions() {
    document.getElementById("create-user-button").addEventListener("click", handleCreateUser);
    document.getElementById("create-channel-button").addEventListener("click", handleCreateChannel);
    const createUploadButton = document.getElementById("create-upload-button");
    if (createUploadButton) {
        createUploadButton.addEventListener("click", handleCreateUpload);
    }
    document.getElementById("refresh-users").addEventListener("click", () => loadUsers());
    document.getElementById("refresh-channels").addEventListener("click", () => loadChannels({ hydrate: true }));
    const refreshUploadsButton = document.getElementById("refresh-uploads");
    if (refreshUploadsButton) {
        refreshUploadsButton.addEventListener("click", () => loadAllUploads());
    }
    document.getElementById("refresh-data").addEventListener("click", () => refreshAll());
    const moderationButton = document.getElementById("refresh-moderation");
    if (moderationButton) {
        moderationButton.addEventListener("click", () => loadModeration(true));
    }
    const analyticsButton = document.getElementById("refresh-analytics");
    if (analyticsButton) {
        analyticsButton.addEventListener("click", () => loadAnalytics(true));
    }
    document.getElementById("download-snapshot").addEventListener("click", exportSnapshot);
    if (signOutButton) {
        signOutButton.addEventListener("click", handleSignOut);
    }
}

async function loadLegal() {
    if (!isCurrentUserAdmin()) {
        return;
    }
    const [dmca, dsr] = await Promise.all([
        apiRequest("/api/legal/dmca"),
        apiRequest("/api/legal/data-subject"),
    ]);
    state.legal.dmca = Array.isArray(dmca) ? dmca : [];
    state.legal.dataSubject = Array.isArray(dsr) ? dsr : [];
    renderLegal();
}

function renderLegal() {
    const dmcaContainer = document.getElementById("legal-dmca-list");
    const dsrContainer = document.getElementById("legal-dsr-list");
    if (!dmcaContainer || !dsrContainer) {
        return;
    }
    clearElement(dmcaContainer);
    clearElement(dsrContainer);
    if (!isCurrentUserAdmin()) {
        dmcaContainer.appendChild(createElement("p", { className: "empty", textContent: "Administrator access required." }));
        dsrContainer.appendChild(createElement("p", { className: "empty", textContent: "Administrator access required." }));
        return;
    }
    for (const item of state.legal.dmca) {
        const card = createElement("article", { className: "card" });
        card.append(createElement("h4", { textContent: item.contentUrl }), createElement("p", { className: "card__meta", textContent: `Status: ${item.status}` }));
        const select = document.createElement("select");
        ["open", "actioned", "restored", "rejected"].forEach((status) => {
            const option = document.createElement("option");
            option.value = status;
            option.textContent = status;
            option.selected = status === item.status;
            select.appendChild(option);
        });
        const btn = createElement("button", { className: "secondary", textContent: "Update" });
        btn.addEventListener("click", async () => {
            await apiRequest(`/api/legal/dmca/${item.id}`, { method: "PATCH", body: JSON.stringify({ status: select.value, notes: "updated from control centre" }) });
            await loadLegal();
        });
        card.append(select, btn);
        dmcaContainer.appendChild(card);
    }
    for (const item of state.legal.dataSubject) {
        const card = createElement("article", { className: "card" });
        card.append(createElement("h4", { textContent: `${item.subjectEmail} (${item.requestType})` }), createElement("p", { className: "card__meta", textContent: `Status: ${item.status}` }));
        const select = document.createElement("select");
        ["open", "actioned", "rejected"].forEach((status) => {
            const option = document.createElement("option");
            option.value = status;
            option.textContent = status;
            option.selected = status === item.status;
            select.appendChild(option);
        });
        const btn = createElement("button", { className: "secondary", textContent: "Update" });
        btn.addEventListener("click", async () => {
            await apiRequest(`/api/legal/data-subject/${item.id}`, { method: "PATCH", body: JSON.stringify({ status: select.value, notes: "updated from control centre" }) });
            await loadLegal();
        });
        card.append(select, btn);
        dsrContainer.appendChild(card);
    }
}

async function initialize() {
    const session = await requireSession();
    state.currentUser = session.user;
    initChatClient();
    renderAccountStatus();
    attachActions();
    await loadMFAStatus();
    renderSetupWizard();
    setupInstaller();
    await refreshAll();
}

if (!globalThis.__BR_SKIP_INIT__) {
    initialize().catch((error) => {
        console.error(error);
        if (error instanceof UnauthorizedError) {
            return;
        }
        showToast(`Failed to initialize: ${error.message}`, "error");
    });
}

export function __setModerationStateForTest(partial) {
    state.moderation = { ...state.moderation, ...partial };
}

export function __setChannelsForTest(channels) {
    state.channels = Array.isArray(channels) ? channels : [];
}

export function __setCurrentUserForTest(user) {
    state.currentUser = user;
}

export { renderModeration };
