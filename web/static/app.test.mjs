import assert from "node:assert/strict";
import test from "node:test";

class FakeClassList {
    constructor() {
        this.items = new Set();
    }

    toggle(name, force) {
        if (force === undefined) {
            if (this.items.has(name)) {
                this.items.delete(name);
                return false;
            }
            this.items.add(name);
            return true;
        }
        if (force) {
            this.items.add(name);
        } else {
            this.items.delete(name);
        }
        return force;
    }
}

class FakeElement {
    constructor(id = "") {
        this.id = id;
        this.children = [];
        this.dataset = {};
        this.attributes = {};
        this.classList = new FakeClassList();
        this.className = "";
        this.textContent = "";
    }

    appendChild(child) {
        this.children.push(child);
        return child;
    }

    append(...nodes) {
        nodes.forEach((node) => this.appendChild(node));
    }

    removeChild(child) {
        const index = this.children.indexOf(child);
        if (index >= 0) {
            this.children.splice(index, 1);
        }
        return child;
    }

    get firstChild() {
        return this.children[0] ?? null;
    }

    setAttribute(name, value) {
        this.attributes[name] = String(value);
    }

    removeAttribute(name) {
        delete this.attributes[name];
    }

    querySelectorAll() {
        return [];
    }

    addEventListener() {}
}

function setupDom() {
    const elements = new Map();
    const documentElement = new FakeElement("document");
    const body = new FakeElement("body");
    const getElementById = (id) => {
        if (!elements.has(id)) {
            elements.set(id, new FakeElement(id));
        }
        return elements.get(id);
    };

    globalThis.document = {
        createElement: () => new FakeElement(),
        getElementById,
        querySelectorAll: () => [],
        addEventListener() {},
        documentElement,
        body,
    };
    globalThis.window = {
        matchMedia: () => null,
        location: { pathname: "/", search: "", hash: "", replace() {} },
        confirm: () => true,
    };
    globalThis.localStorage = {
        getItem: () => null,
        setItem: () => {},
    };

    return { elements };
}

test("renderModeration handles partial data payloads", async () => {
    globalThis.__BR_SKIP_INIT__ = true;
    const { elements } = setupDom();

    const moduleUrl = new URL("./app.js", import.meta.url);
    const { renderModeration, __setModerationStateForTest, __setChannelsForTest } = await import(moduleUrl);

    __setChannelsForTest([{ id: "channel-1", title: "Channel 1" }, { id: "channel-2", title: "Channel 2" }]);
    __setModerationStateForTest({
        queue: [
            {
                id: "flag-1",
                channelId: "channel-1",
                channelTitle: "Channel 1",
                flaggedAt: new Date().toISOString(),
            },
        ],
        actions: [],
        appeals: [
            { id: "appeal-1", reason: "please review", status: "open", createdAt: new Date().toISOString() },
        ],
        automod: [
            {
                id: "auto-1",
                channelId: "channel-1",
                channelTitle: "Channel 1",
                message: "Blocked content",
                createdAt: new Date().toISOString(),
            },
        ],
        filters: {
            "channel-1": [
                {
                    id: "filter-1",
                    channelId: "channel-1",
                    kind: "keyword",
                    pattern: "spam",
                    enabled: true,
                    createdAt: new Date().toISOString(),
                    updatedAt: new Date().toISOString(),
                },
            ],
        },
    });

    renderModeration();

    assert.ok(elements.get("moderation-queue").children.length > 0);
    assert.ok(elements.get("moderation-history").children.length > 0);
    assert.ok(elements.get("moderation-appeals").children.length > 0);
    assert.ok(elements.get("moderation-automod").children.length > 0);
    assert.ok(elements.get("moderation-filters").children.length > 0);
});
