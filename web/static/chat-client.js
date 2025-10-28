export class ChatClient {
    constructor({ url = "/api/chat/ws", onEvent, onError, onOpen } = {}) {
        this.url = this.resolveURL(url);
        this.onEvent = onEvent;
        this.onError = onError;
        this.onOpen = onOpen;
        this.socket = null;
        this.pendingJoins = new Set();
        this.joined = new Set();
        this.queue = [];
        this.reconnectDelay = 500;
        this.maxDelay = 4000;
        this.connect();
    }

    resolveURL(path) {
        if (path.startsWith("ws://") || path.startsWith("wss://")) {
            return path;
        }
        const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
        return `${protocol}//${window.location.host}${path}`;
    }

    connect() {
        if (this.socket) {
            try {
                this.socket.close();
            } catch (error) {
                console.warn("chat socket close", error);
            }
        }
        this.socket = new WebSocket(this.url);
        this.socket.addEventListener("open", () => {
            this.reconnectDelay = 500;
            if (typeof this.onOpen === "function") {
                this.onOpen();
            }
            for (const channel of this.pendingJoins) {
                this.join(channel);
            }
            this.flushQueue();
        });
        this.socket.addEventListener("message", (event) => {
            this.handleMessage(event);
        });
        this.socket.addEventListener("close", () => {
            this.scheduleReconnect();
        });
        this.socket.addEventListener("error", (event) => {
            if (typeof this.onError === "function") {
                this.onError(event);
            }
        });
    }

    scheduleReconnect() {
        setTimeout(() => {
            this.reconnectDelay = Math.min(this.maxDelay, this.reconnectDelay * 2);
            this.connect();
        }, this.reconnectDelay);
    }

    flushQueue() {
        while (this.queue.length) {
            this.socket.send(JSON.stringify(this.queue.shift()));
        }
    }

    send(payload) {
        const data = JSON.stringify(payload);
        if (this.socket?.readyState === WebSocket.OPEN) {
            this.socket.send(data);
        } else {
            this.queue.push(payload);
        }
    }

    join(channelId) {
        if (!channelId) {
            return;
        }
        this.pendingJoins.add(channelId);
        this.joined.add(channelId);
        this.send({ type: "join", channelId });
    }

    leave(channelId) {
        if (!channelId) {
            return;
        }
        this.pendingJoins.delete(channelId);
        this.joined.delete(channelId);
        this.send({ type: "leave", channelId });
    }

    message(channelId, content) {
        this.send({ type: "message", channelId, content });
    }

    timeout(channelId, targetId, durationMs) {
        this.send({ type: "timeout", channelId, targetId, durationMs });
    }

    clearTimeout(channelId, targetId) {
        this.send({ type: "remove_timeout", channelId, targetId });
    }

    ban(channelId, targetId) {
        this.send({ type: "ban", channelId, targetId });
    }

    unban(channelId, targetId) {
        this.send({ type: "unban", channelId, targetId });
    }

    handleMessage(event) {
        let payload = null;
        try {
            payload = JSON.parse(event.data);
        } catch (error) {
            console.warn("Invalid chat payload", error);
            return;
        }
        if (payload?.type === "event" && typeof this.onEvent === "function") {
            this.onEvent(payload.event);
            return;
        }
        if (payload?.type === "error" && typeof this.onError === "function") {
            this.onError(new Error(payload.error));
            return;
        }
    }
}
