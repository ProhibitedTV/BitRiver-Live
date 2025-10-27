const state = {
    users: [],
    channels: [],
    sessions: {},
    chat: {},
    profiles: [],
    profileIndex: new Map(),
    selectedProfileId: null,
};

const modal = document.getElementById("modal");
const modalTitle = document.getElementById("modal-title");
const modalBody = document.getElementById("modal-body");
const overviewCards = document.getElementById("overview-cards");
const profileDetail = document.getElementById("profile-detail");

function switchView(id) {
    for (const panel of document.querySelectorAll(".panel")) {
        panel.classList.toggle("active", panel.id === id);
    }
}

document.querySelectorAll(".hero__nav button").forEach((btn) => {
    btn.addEventListener("click", () => switchView(btn.dataset.view));
});

async function apiRequest(path, options = {}) {
    const response = await fetch(path, {
        headers: { "Content-Type": "application/json" },
        ...options,
    });
    if (!response.ok) {
        const payload = await response.json().catch(() => ({}));
        throw new Error(payload.error || response.statusText);
    }
    if (response.status === 204) {
        return null;
    }
    return response.json();
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
    list.innerHTML = "";
    if (!state.users.length) {
        empty.style.display = "block";
        return;
    }
    empty.style.display = "none";
    for (const user of state.users) {
        const card = document.createElement("article");
        card.className = "card";
        const roles = user.roles.length
            ? user.roles.map((role) => `<span class="pill">${role}</span>`).join(" ")
            : '<span class="card__meta">viewer</span>';
        card.innerHTML = `
            <div class="card__header">
                <h3>${user.displayName}</h3>
                <span class="card__meta">Joined ${formatRelativeTime(user.createdAt)}</span>
            </div>
            <div class="card__meta">${user.email}</div>
            <div class="pill-group">${roles}</div>
            <div class="card__actions">
                <button class="secondary" data-action="edit-user" data-user="${user.id}">Edit</button>
                <button class="secondary" data-action="profile-user" data-user="${user.id}">Profile</button>
                <button class="danger" data-action="delete-user" data-user="${user.id}">Remove</button>
            </div>
        `;
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
}

function renderChannels() {
    const list = document.getElementById("channels-list");
    const empty = document.getElementById("channels-empty");
    list.innerHTML = "";
    if (!state.channels.length) {
        empty.style.display = "block";
        return;
    }
    empty.style.display = "none";
    for (const channel of state.channels) {
        const owner = state.users.find((user) => user.id === channel.ownerId);
        const tags = channel.tags.length
            ? channel.tags.map((tag) => `<span class="pill">${tag}</span>`).join(" ")
            : '<span class="card__meta">No tags</span>';
        const updated = formatRelativeTime(channel.updatedAt);
        const liveClass = channel.liveState === "live" ? "status-live" : "status-offline";
        const card = document.createElement("article");
        card.className = "card";
        card.innerHTML = `
            <div class="card__header">
                <h3>${channel.title}</h3>
                <span class="card__meta">${channel.category || "General"}</span>
            </div>
            <div class="card__meta">Owner: ${owner ? owner.displayName : channel.ownerId}</div>
            <div class="pill-group">${tags}</div>
            <div class="channel-meta">
                <span class="card__meta">Updated ${updated}</span>
                <span class="card__meta">State: <span class="${liveClass}">${channel.liveState}</span></span>
            </div>
            <details>
                <summary>Stream key & ingest tips</summary>
                <div class="stream-key">
                    <code>${channel.streamKey}</code>
                    <button class="secondary" data-action="copy-stream-key" data-key="${channel.streamKey}">Copy</button>
                </div>
                <p class="card__meta">Use <code>rtmp://YOUR_INGEST_SERVER/live</code> with the key above.</p>
            </details>
            <div class="card__actions">
                <button class="secondary" data-action="edit-channel" data-channel="${channel.id}">Edit</button>
                <button class="danger" data-action="delete-channel" data-channel="${channel.id}">Delete</button>
            </div>
        `;
        list.appendChild(card);
    }

    list.querySelectorAll("[data-action=copy-stream-key]").forEach((btn) => {
        btn.addEventListener("click", async () => {
            try {
                await navigator.clipboard.writeText(btn.dataset.key);
                showToast("Stream key copied");
            } catch (error) {
                showToast("Clipboard not available", "error");
            }
        });
    });
    list.querySelectorAll("[data-action=edit-channel]").forEach((btn) => {
        btn.addEventListener("click", () => handleEditChannel(btn.dataset.channel));
    });
    list.querySelectorAll("[data-action=delete-channel]").forEach((btn) => {
        btn.addEventListener("click", () => handleDeleteChannel(btn.dataset.channel));
    });
}

function populateOwnerSelect(select, selected) {
    select.innerHTML = state.users
        .map((user) => {
            const isSelected = selected === user.id ? "selected" : "";
            return `<option value="${user.id}" ${isSelected}>${user.displayName}</option>`;
        })
        .join("");
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
    container.innerHTML = "";
    const sessions = Object.values(state.sessions).flat();
    if (!sessions.length) {
        container.innerHTML = '<div class="empty">No stream sessions yet.</div>';
        return;
    }
    const sorted = sessions.sort((a, b) => new Date(b.startedAt) - new Date(a.startedAt));
    for (const session of sorted) {
        const channel = state.channels.find((item) => item.id === session.channelId);
        const duration = formatDuration(computeSessionDuration(session));
        const card = document.createElement("article");
        card.className = "card";
        card.innerHTML = `
            <div class="card__header">
                <h3>${channel ? channel.title : session.channelId}</h3>
                <span class="card__meta">Started ${formatDate(session.startedAt)}</span>
            </div>
            <div class="card__meta">Ended: ${session.endedAt ? formatDate(session.endedAt) : "Live"}</div>
            <div class="card__meta">Duration: ${duration}</div>
            <div class="card__meta">Peak concurrent viewers: ${session.peakConcurrent}</div>
            <div class="card__meta">Renditions: ${session.renditions.length ? session.renditions.join(", ") : "Source"}</div>
        `;
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
    container.innerHTML = "";
    if (!state.channels.length) {
        container.innerHTML = '<div class="empty">Add a channel to unlock chat controls.</div>';
        return;
    }

    for (const channel of state.channels) {
        const messages = state.chat[channel.id] || [];
        const card = document.createElement("article");
        card.className = "card";
        const messageMarkup = messages
            .map(
                (message) => `
                    <div class="chat-message">
                        <div class="chat-header">
                            <strong>${message.userId}</strong>
                            <span class="card__meta">${formatRelativeTime(message.createdAt)}</span>
                        </div>
                        <div>${message.content}</div>
                        <div class="chat-actions">
                            <button class="danger" data-action="delete-message" data-channel="${channel.id}" data-message="${message.id}">Remove</button>
                        </div>
                    </div>
                `,
            )
            .join("");
        const userOptions = state.users
            .map((user) => `<option value="${user.id}">${user.displayName}</option>`)
            .join("");
        card.innerHTML = `
            <div class="card__header">
                <h3>${channel.title}</h3>
                <div class="card__meta">${messages.length} message${messages.length === 1 ? "" : "s"}</div>
            </div>
            <div class="chat-toolbar">
                <button class="secondary" data-action="refresh-chat" data-channel="${channel.id}">Refresh</button>
            </div>
            <div class="chat-log">${messageMarkup || '<div class="card__meta">No chat messages yet.</div>'}</div>
            <form class="chat-form" data-channel="${channel.id}">
                <label>
                    User
                    <select name="userId" required>
                        <option value="" disabled ${state.users.length ? "" : "selected"}>Select user</option>
                        ${userOptions}
                    </select>
                </label>
                <label>
                    Message
                    <input type="text" name="content" required placeholder="Say hello" />
                </label>
                <button type="submit" class="primary">Send message</button>
            </form>
        `;
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
                await apiRequest(`/api/channels/${channelId}/chat`, {
                    method: "POST",
                    body: JSON.stringify({ userId, content }),
                });
                form.reset();
                await loadChatHistory(channelId);
                renderChat();
            } catch (error) {
                showToast(error.message, "error");
            }
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
    list.innerHTML = "";
    if (!state.profiles.length) {
        list.innerHTML = '<div class="empty">Profiles will appear once you create them.</div>';
        return;
    }
    const sorted = [...state.profiles].sort((a, b) => a.displayName.localeCompare(b.displayName));
    for (const profile of sorted) {
        const liveCount = profile.liveChannels.length;
        const friends = profile.topFriends.length
            ? profile.topFriends.map((friend) => friend.displayName).join(", ")
            : "No top friends yet";
        const card = document.createElement("article");
        card.className = "card";
        card.innerHTML = `
            <div class="card__header">
                <h3>${profile.displayName}</h3>
                <span class="card__meta">${profile.channels.length} channel${profile.channels.length === 1 ? "" : "s"}</span>
            </div>
            <p>${profile.bio || "No bio yet."}</p>
            <div class="card__meta">Live now: ${liveCount}</div>
            <div class="card__meta">Top friends: ${friends}</div>
            <div class="card__actions">
                <button class="secondary" data-action="view-profile" data-user="${profile.userId}">View</button>
                <button class="primary" data-action="edit-profile" data-user="${profile.userId}">Edit</button>
            </div>
        `;
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
    if (!userId) {
        profileDetail.innerHTML = `
            <div class="card__header">
                <h3>Profile details</h3>
                <span class="card__meta">Select a creator to inspect or edit.</span>
            </div>
            <p class="card__meta">No profile selected.</p>
        `;
        return;
    }
    const profile = state.profileIndex.get(userId);
    if (!profile) {
        profileDetail.innerHTML = `
            <div class="card__header">
                <h3>Profile details</h3>
            </div>
            <p class="card__meta">Profile not found.</p>
        `;
        return;
    }
    const donations = profile.donationAddresses.length
        ? profile.donationAddresses
              .map((addr) => `<li><span class="pill">${addr.currency}</span> ${addr.address}${addr.note ? ` — ${addr.note}` : ""}</li>`)
              .join("")
        : "<li class=\"card__meta\">No donation links configured.</li>";
    const channels = profile.channels
        .map((channel) => `<li>${channel.title} — <span class="card__meta">${channel.category || "General"}</span></li>`)
        .join("");
    profileDetail.innerHTML = `
        <div class="card__header">
            <h3>${profile.displayName}</h3>
            <button class="secondary" data-action="edit-profile" data-user="${profile.userId}">Edit</button>
        </div>
        <p>${profile.bio || "No bio yet."}</p>
        <div class="profile-section">
            <h4>Top friends</h4>
            <p class="card__meta">${profile.topFriends.length ? profile.topFriends.map((friend) => friend.displayName).join(", ") : "None"}</p>
        </div>
        <div class="profile-section">
            <h4>Channels</h4>
            <ul>${channels || '<li class="card__meta">No channels yet.</li>'}</ul>
        </div>
        <div class="profile-section">
            <h4>Donation addresses</h4>
            <ul>${donations}</ul>
        </div>
    `;
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
            const options = ['<option value="">None</option>'];
            for (const channel of profile.channels) {
                const selected = profile.featuredChannelId === channel.id ? "selected" : "";
                options.push(`<option value="${channel.id}" ${selected}>${channel.title}</option>`);
            }
            featuredSelect.innerHTML = options.join("");

            const friendsSelect = modal.querySelector('[name="topFriends"]');
            friendsSelect.innerHTML = state.users
                .filter((candidate) => candidate.id !== userId)
                .map((candidate) => {
                    const selected = profile.topFriends.some((friend) => friend.userId === candidate.id)
                        ? "selected"
                        : "";
                    return `<option value="${candidate.id}" ${selected}>${candidate.displayName}</option>`;
                })
                .join("");
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

    overviewCards.innerHTML = "";
    for (const cardData of cards) {
        const card = document.createElement("article");
        card.className = "card";
        card.innerHTML = `
            <div class="card__header">
                <h3>${cardData.title}</h3>
            </div>
            <div class="card__value">${cardData.value}</div>
            <div class="card__meta">${cardData.detail}</div>
        `;
        overviewCards.appendChild(card);
    }
}

function renderStreamControls() {
    const container = document.getElementById("stream-controls");
    container.innerHTML = "";
    if (!state.channels.length) {
        container.innerHTML = '<div class="empty">Create a channel first to control your live stream.</div>';
        return;
    }
    for (const channel of state.channels) {
        const form = document.createElement("article");
        form.className = "card";
        form.innerHTML = `
            <div class="card__header">
                <h3>${channel.title}</h3>
                <span class="card__meta">${channel.streamKey}</span>
            </div>
            <div class="card__meta">State: <strong class="${channel.liveState === "live" ? "status-live" : "status-offline"}">${channel.liveState}</strong></div>
            <form class="stream-form" data-channel="${channel.id}">
                <label>
                    Renditions (comma separated)
                    <input type="text" name="renditions" placeholder="1080p60,720p30" />
                </label>
                <label>
                    Peak concurrent viewers (on stop)
                    <input type="number" name="peakConcurrent" min="0" value="0" />
                </label>
                <div class="card__actions">
                    <button type="submit" data-action="start" class="primary">Start stream</button>
                    <button type="button" data-action="stop" class="secondary">Stop stream</button>
                </div>
            </form>
        `;
        container.appendChild(form);
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
        ? `# Reverse proxy hint: point ${data.hostname} to this service for HTTPS.`
        : `# Configure your reverse proxy or tailnet to expose the service. ${mode === "production" ? "Port 80 is used by default." : "Development mode keeps the control center on :8080."}`;
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
BITRIVER_LIVE_ADDR=${addr}
BITRIVER_LIVE_MODE=${mode}
BITRIVER_LIVE_DATA=$DATA_FILE
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
echo "Service is running on $ADDR (${mode} mode)"
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
}

async function initialize() {
    attachActions();
    setupInstaller();
    await refreshAll();
}

initialize().catch((error) => {
    console.error(error);
    showToast(`Failed to initialize: ${error.message}`, "error");
});
