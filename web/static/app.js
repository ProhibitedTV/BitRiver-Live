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
};

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

const modal = document.getElementById("modal");
const modalTitle = document.getElementById("modal-title");
const modalBody = document.getElementById("modal-body");
const overviewCards = document.getElementById("overview-cards");
const profileDetail = document.getElementById("profile-detail");
const accountActions = document.getElementById("account-actions");
const accountName = document.getElementById("current-user-name");
const signOutButton = document.getElementById("sign-out-button");

function switchView(id) {
    for (const panel of document.querySelectorAll(".panel")) {
        panel.classList.toggle("active", panel.id === id);
    }
}

document.querySelectorAll(".hero__nav button").forEach((btn) => {
    btn.addEventListener("click", () => switchView(btn.dataset.view));
});

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

function redirectToAuth() {
    window.location.replace("/signup");
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
    renderChannels();
    renderStreamControls();
    renderDashboard();
    renderSessions();
    renderChat();
    initChatClient();
    syncChatSubscriptions();
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

async function loadSessionsForChannel(channelId) {
    const sessions = await apiRequest(`/api/channels/${channelId}/sessions`);
    state.sessions[channelId] = sessions;
    return sessions;
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

function renderDashboard() {
    const sessions = Object.values(state.sessions).flat();
    const totalDuration = sessions.reduce((sum, session) => sum + computeSessionDuration(session), 0);
    const totalPeak = sessions.reduce((sum, session) => sum + session.peakConcurrent, 0);
    const chatCount = Object.values(state.chat).reduce((sum, messages) => sum + (messages?.length || 0), 0);
    const lastSession = sessions.sort((a, b) => new Date(b.startedAt) - new Date(a.startedAt))[0];

    const cards = [
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
        actions.append(startButton, stopButton);
        form.appendChild(actions);

        card.appendChild(form);
        container.appendChild(card);
    }

    container.querySelectorAll(".stream-form").forEach((form) => {
        const channelId = form.dataset.channel;
        const stopBtn = form.querySelector('[data-action="stop"]');
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
    });
}

function computeInstallerScript(data) {
    const mode = data.mode || "production";
    const addr = data.addr || (mode === "production" ? ":80" : ":8080");
    const logDir = data.enableLogs ? `${data.dataDir}/logs` : "";
    const hostnameHint = data.hostname
        ? `# Reverse proxy hint: point ${data.hostname} to this service and expose TLS traffic on 443.`
        : `# Configure your reverse proxy or tailnet to expose the service. ${mode === "production" ? "Port 80 is used by default." : "Development mode keeps the control center on :8080."}`;
    const envLines = [
        `BITRIVER_LIVE_ADDR=${addr}`,
        `BITRIVER_LIVE_MODE=${mode}`,
        `BITRIVER_LIVE_DATA=$DATA_FILE`,
    ];
    if (data.tlsCert) {
        envLines.push(`BITRIVER_LIVE_TLS_CERT=${data.tlsCert}`);
    }
    if (data.tlsKey) {
        envLines.push(`BITRIVER_LIVE_TLS_KEY=${data.tlsKey}`);
    }
    if (data.rateGlobalRps) {
        envLines.push(`BITRIVER_LIVE_RATE_GLOBAL_RPS=${data.rateGlobalRps}`);
    }
    if (data.rateLoginLimit) {
        envLines.push(`BITRIVER_LIVE_RATE_LOGIN_LIMIT=${data.rateLoginLimit}`);
    }
    if (data.rateLoginWindow) {
        envLines.push(`BITRIVER_LIVE_RATE_LOGIN_WINDOW=${data.rateLoginWindow}`);
    }
    if (data.redisAddr) {
        envLines.push(`BITRIVER_LIVE_RATE_REDIS_ADDR=${data.redisAddr}`);
    }
    if (data.redisPassword) {
        envLines.push(`BITRIVER_LIVE_RATE_REDIS_PASSWORD=${data.redisPassword}`);
    }
    return `#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${data.installDir}"
DATA_DIR="${data.dataDir}"
SERVICE_USER="${data.serviceUser}"
MODE="${mode}"
ADDR="${addr}"
DATA_FILE="$DATA_DIR/store.json"
${logDir ? `LOG_DIR="${logDir}"` : ""}

if ! id -u "$SERVICE_USER" >/dev/null 2>&1; then
    sudo useradd --system --create-home --shell /usr/sbin/nologin "$SERVICE_USER"
fi

sudo install -d -o "$SERVICE_USER" -g "$SERVICE_USER" "$INSTALL_DIR" "$DATA_DIR"
${logDir ? 'sudo install -d -o "$SERVICE_USER" -g "$SERVICE_USER" "$LOG_DIR"' : ""}

if ! command -v go >/dev/null 2>&1; then
    echo "Go 1.21+ is required to build BitRiver Live" >&2
    exit 1
fi

GOFLAGS="-trimpath" go build -o bitriver-live ./cmd/server
sudo install -m 0755 bitriver-live "$INSTALL_DIR/bitriver-live"
rm -f bitriver-live

cat <<'ENV' | sudo tee "$INSTALL_DIR/.env" >/dev/null
${envLines.join("\n")}
ENV

sudo chown -R "$SERVICE_USER":"$SERVICE_USER" "$INSTALL_DIR" "$DATA_DIR"

cat <<'SERVICE' | sudo tee /etc/systemd/system/bitriver-live.service >/dev/null
[Unit]
Description=BitRiver Live Streaming Control Center
After=network.target

[Service]
Type=simple
User=$SERVICE_USER
EnvironmentFile=$INSTALL_DIR/.env
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/bitriver-live
Restart=on-failure
${logDir ? 'StandardOutput=append:$LOG_DIR/server.log\nStandardError=append:$LOG_DIR/server.log' : ''}

[Install]
WantedBy=multi-user.target
SERVICE

sudo systemctl daemon-reload
sudo systemctl enable --now bitriver-live.service

${hostnameHint}
echo "Service is running on $ADDR (${mode} mode). TLS settings and metrics are configured via $INSTALL_DIR/.env"
`;
}

function setupInstaller() {
    const container = document.getElementById("installer");
    container.innerHTML = "";
    const template = document.getElementById("installer-template");
    container.appendChild(template.content.cloneNode(true));
    const form = container.querySelector("#installer-form");
    const output = container.querySelector("#installer-output");
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
        showToast("Installer script generated. Copy and run on your home server.");
    });
}

async function refreshAll() {
    await Promise.all([
        loadUsers(),
        loadChannels({ hydrate: true }),
        loadProfiles(),
    ]);
}

function attachActions() {
    document.getElementById("create-user-button").addEventListener("click", handleCreateUser);
    document.getElementById("create-channel-button").addEventListener("click", handleCreateChannel);
    document.getElementById("refresh-users").addEventListener("click", () => loadUsers());
    document.getElementById("refresh-channels").addEventListener("click", () => loadChannels({ hydrate: true }));
    document.getElementById("refresh-data").addEventListener("click", () => refreshAll());
    document.getElementById("download-snapshot").addEventListener("click", exportSnapshot);
    if (signOutButton) {
        signOutButton.addEventListener("click", handleSignOut);
    }
}

async function initialize() {
    const session = await requireSession();
    state.currentUser = session.user;
    initChatClient();
    renderAccountStatus();
    attachActions();
    setupInstaller();
    await refreshAll();
}

initialize().catch((error) => {
    console.error(error);
    if (error instanceof UnauthorizedError) {
        return;
    }
    showToast(`Failed to initialize: ${error.message}`, "error");
});
