const SESSION_KEY = "bitriver-live:session";

const signupForm = document.getElementById("signup-form");
const loginForm = document.getElementById("login-form");
const feedback = document.getElementById("auth-feedback");

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
        body: JSON.stringify(payload),
    });
    if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || response.statusText);
    }
    return response.json();
}

function storeSessionToken(token) {
    try {
        localStorage.setItem(SESSION_KEY, token);
    } catch (error) {
        console.warn("Unable to persist session token", error);
    }
}

if (signupForm) {
    signupForm.addEventListener("submit", async (event) => {
        event.preventDefault();
        clearFeedback();
        const form = event.currentTarget;
        const data = Object.fromEntries(new FormData(form).entries());
        try {
            const result = await requestAuth("/api/auth/signup", data);
            storeSessionToken(result.token);
            form.reset();
            showFeedback(
                "Account created! Launch the control center to claim a channel or request creator access.",
            );
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
            const result = await requestAuth("/api/auth/login", data);
            storeSessionToken(result.token);
            showFeedback("Signed in. Open the control center to manage your stream.");
        } catch (error) {
            showFeedback(error.message, "error");
        }
    });
}
