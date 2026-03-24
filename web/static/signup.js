const signupForm = document.getElementById("signup-form");
const signupCard = document.getElementById("signup-card");
const signupClosedNote = document.getElementById("signup-closed-note");
const loginForm = document.getElementById("login-form");
const feedback = document.getElementById("auth-feedback");
const mfaStep = document.getElementById("mfa-step");
const mfaForm = document.getElementById("mfa-form");
const mfaEnrollment = document.getElementById("mfa-enrollment");
const mfaRecoveryCodes = document.getElementById("mfa-recovery-codes");
const destinationCopy = document.getElementById("auth-destination-copy");
const destinationPath = document.getElementById("auth-destination-path");
const returnLinks = document.querySelectorAll("[data-auth-return-link]");
const signupJumpLink = document.getElementById("auth-signup-jump-link");
const DEFAULT_DESTINATION = "/viewer";
const REDIRECT_DELAY_MS = 600;
let pendingMFAToken = null;
const allowSelfSignup = document.body?.dataset.allowSelfSignup === "true";

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
const viewerDestination = resolveViewerDestination(destination);

function resolveViewerDestination(candidate) {
    if (!isSafeOnsitePath(candidate)) {
        return DEFAULT_DESTINATION;
    }
    const url = new URL(candidate, window.location.origin);
    const path = `${url.pathname}${url.search}${url.hash}` || DEFAULT_DESTINATION;
    if (url.pathname === "/" || url.pathname === "/viewer") {
        return DEFAULT_DESTINATION;
    }
    if (url.pathname.startsWith("/signup") || url.pathname.startsWith("/admin") || url.pathname.startsWith("/api/")) {
        return DEFAULT_DESTINATION;
    }
    return path;
}

function describeDestination(route) {
    const normalized = route.toLowerCase();
    if (normalized.includes("/browse")) {
        return "You will land back in browse with your current search or filter context ready to go.";
    }
    if (normalized.includes("/channel/")) {
        return "You will jump back into the stream page you opened from the viewer.";
    }
    if (route === DEFAULT_DESTINATION) {
        return "You will head back to live discovery, featured streams, and the full channel directory.";
    }
    return "You will return to the viewer route that sent you here as soon as auth completes.";
}

function formatDestinationPath(route) {
    if (route.length <= 56) {
        return route;
    }
    return `${route.slice(0, 53)}...`;
}

function applyDestinationContext() {
    returnLinks.forEach((link) => {
        if (!(link instanceof HTMLAnchorElement)) {
            return;
        }
        link.href = viewerDestination;
    });

    if (destinationPath) {
        destinationPath.textContent = formatDestinationPath(viewerDestination);
    }

    if (destinationCopy) {
        destinationCopy.textContent = describeDestination(viewerDestination);
    }
}

function applyAuthConfig() {
    if (signupCard) {
        signupCard.hidden = !allowSelfSignup;
    }
    if (signupClosedNote) {
        signupClosedNote.hidden = allowSelfSignup;
    }
    if (signupJumpLink) {
        signupJumpLink.hidden = !allowSelfSignup;
    }
}

function focusAuthTarget() {
    const targetId = window.location.hash.replace(/^#/, "");
    if (!targetId) {
        return;
    }

    if (targetId === "signup-card" && !allowSelfSignup) {
        showFeedback("Public self-signup is disabled on this server. Sign in with an existing account or ask an administrator for access. If you run this server, use the bootstrap admin account from the deployment .env file at /admin.", "error");
        const emailInput = loginForm?.querySelector('input[name="email"]');
        if (emailInput instanceof HTMLElement) {
            emailInput.focus();
        }
        return;
    }

    const target = document.getElementById(targetId);
    if (!target) {
        return;
    }

    target.scrollIntoView({ block: "start" });
    const focusTarget = target.querySelector("input, button, a");
    if (focusTarget instanceof HTMLElement) {
        focusTarget.focus();
    }
}

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

function showMFA(stepVisible) {
    if (!mfaStep) {
        return;
    }
    mfaStep.hidden = !stepVisible;
}

function renderEnrollment(enrollment) {
    if (!mfaEnrollment) {
        return;
    }
    if (!enrollment) {
        mfaEnrollment.hidden = true;
        return;
    }
    mfaEnrollment.hidden = false;
    const secretInput = mfaEnrollment.querySelector('input[name="secret"]');
    const urlInput = mfaEnrollment.querySelector('input[name="otpauthUrl"]');
    if (secretInput) {
        secretInput.value = enrollment.secret || "";
    }
    if (urlInput) {
        urlInput.value = enrollment.otpauthUrl || "";
    }
    if (mfaRecoveryCodes) {
        mfaRecoveryCodes.innerHTML = "";
        if (Array.isArray(enrollment.recoveryCodes)) {
            enrollment.recoveryCodes.forEach((code) => {
                const li = document.createElement("li");
                li.textContent = code;
                mfaRecoveryCodes.appendChild(li);
            });
        }
    }
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

async function handleMFAEnrollment(token) {
    try {
        const enrollment = await requestAuth("/api/auth/mfa/enroll", { token });
        renderEnrollment(enrollment);
    } catch (error) {
        showFeedback(error.message, "error");
    }
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
            showFeedback("Account created! Redirecting you now.");
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
            const response = await requestAuth("/api/auth/login", data);
            if (response.mfaRequired) {
                pendingMFAToken = response.mfaToken;
                renderEnrollment(response.enrollment);
                showMFA(true);
                showFeedback("MFA required. Enter the verification code to continue.");
                return;
            }
            showFeedback("Signed in! Redirecting you now.");
            window.setTimeout(() => {
                window.location.assign(destination);
            }, REDIRECT_DELAY_MS);
        } catch (error) {
            showFeedback(error.message, "error");
        }
    });
}

if (mfaForm) {
    mfaForm.addEventListener("submit", async (event) => {
        event.preventDefault();
        clearFeedback();
        const form = event.currentTarget;
        const data = Object.fromEntries(new FormData(form).entries());
        if (pendingMFAToken) {
            data.token = pendingMFAToken;
        }
        try {
            await requestAuth("/api/auth/mfa/verify", data);
            showFeedback("Verified! Redirecting you now.");
            window.setTimeout(() => {
                window.location.assign(destination);
            }, REDIRECT_DELAY_MS);
        } catch (error) {
            showFeedback(error.message, "error");
        }
    });
}

const params = new URLSearchParams(window.location.search);
const mfaToken = params.get("mfaToken");
applyDestinationContext();
applyAuthConfig();
focusAuthTarget();
if (mfaToken) {
    pendingMFAToken = mfaToken;
    showMFA(true);
    if (params.get("mfa") === "enroll") {
        handleMFAEnrollment(mfaToken);
    }
}
