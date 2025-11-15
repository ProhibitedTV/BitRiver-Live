const signupForm = document.getElementById("signup-form");
const loginForm = document.getElementById("login-form");
const feedback = document.getElementById("auth-feedback");
const DEFAULT_DESTINATION = "/viewer";
const REDIRECT_DELAY_MS = 600;

function isSafeOnsitePath(candidate) {
    if (!candidate || typeof candidate !== "string") {
        return false;
    }
    try {
        const url = new URL(candidate, window.location.origin);
        return url.origin === window.location.origin;
    } catch (error) {
        console.warn("Invalid next parameter", error);
        return false;
    }
}

function resolveDestination() {
    const params = new URLSearchParams(window.location.search);
    const next = params.get("next");
    if (next && isSafeOnsitePath(next)) {
        const url = new URL(next, window.location.origin);
        return `${url.pathname}${url.search}${url.hash}` || DEFAULT_DESTINATION;
    }
    return DEFAULT_DESTINATION;
}

const destination = resolveDestination();

function showFeedback(message, variant = "info") {
    if (!feedback) {
        return;
    }
    feedback.textContent = message;
    feedback.hidden = false;
    feedback.classList.toggle("error", variant === "error");
}

function clearFeedback() {
    if (!feedback) {
        return;
    }
    feedback.hidden = true;
    feedback.textContent = "";
    feedback.classList.remove("error");
}

async function requestAuth(path, payload) {
    const response = await fetch(path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify(payload),
    });
    if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || response.statusText);
    }
    return response.json();
}

if (signupForm) {
    signupForm.addEventListener("submit", async (event) => {
        event.preventDefault();
        clearFeedback();
        const form = event.currentTarget;
        const data = Object.fromEntries(new FormData(form).entries());
        try {
            await requestAuth("/api/auth/signup", data);
            form.reset();
            showFeedback("Account created! Redirecting you to the control center.");
            window.setTimeout(() => {
                window.location.assign(destination);
            }, REDIRECT_DELAY_MS);
        } catch (error) {
            showFeedback(error.message, "error");
        }
    });
}

if (loginForm) {
    loginForm.addEventListener("submit", async (event) => {
        event.preventDefault();
        clearFeedback();
        const form = event.currentTarget;
        const data = Object.fromEntries(new FormData(form).entries());
        try {
            await requestAuth("/api/auth/login", data);
            showFeedback("Signed in! Redirecting you now.");
            window.setTimeout(() => {
                window.location.assign(destination);
            }, REDIRECT_DELAY_MS);
        } catch (error) {
            showFeedback(error.message, "error");
        }
    });
}
