# Frontend Manual QA

The control center relies on client-side rendering for most of its views. Use the
following checklist to verify that untrusted content is safely escaped while
legitimate strings are still rendered correctly.

1. Start the API server locally:

   ```bash
   go run -tags postgres ./cmd/server --mode development --addr :8080 --data /tmp/bitriver-live.json
   ```

2. Sign in to the control center and create a user with the display name
   `River <img src=x onerror=alert(1)>`.

   *Expected:* the Users list shows the literal characters
   `River <img src=x onerror=alert(1)>` without executing any script or
   loading an image. The Roles pills and action buttons should remain functional.

3. Create or edit a profile for that user with the bio set to
   `<img src=x onerror=alert(1)>` and a donation note of the same value.

   *Expected:* the Profiles list and the Profile details panel render the exact
   text for the bio and donation note with no alert dialog or other script
   execution.

4. Replace the malicious values with a normal string (for example,
   `River Rapid Adventures`) and confirm that the updated content appears in the
   Users and Profiles views.

5. Create a channel titled `<img src=x onerror=alert(1)>` and navigate to the
   **Go Live** view.

   *Expected:* the channel card shows the literal characters of the title in
   the header with no image load or alert dialog, and the Start/Stop buttons
   remain usable.

These steps demonstrate that unsafe markup is rendered inert while normal
content flows through unchanged.

## Viewer chat access

1. Open a channel page in the viewer experience while signed out.

   *Expected:* the chat panel loads without errors, shows "No messages yet"
   when there is no history, and the composer is disabled with the
   "Sign in to participate in chat" placeholder. If the network tab shows a
   `401` response for the chat API, the panel should still render the empty
   state and stop polling until you sign in.

2. Sign in with a viewer account and return to the same channel page.

   *Expected:* existing chat history (if any) renders without needing to
   refresh, and the composer placeholder changes to "Share your thoughts"
   to indicate that posting is available.
