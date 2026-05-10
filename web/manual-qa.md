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

## Viewer profile management

1. Sign in as a viewer and open `/profile`.

   *Expected:* the page shows your display name with any saved avatar,
   banner, and bio values prefilled in the form fields. Empty fields should
   show helper text so you know what to add.

2. Update the Avatar URL, Banner URL, and Bio fields (try a harmless string
   such as `River <img src=x onerror=alert(1)>`) and save the profile.

   *Expected:* the preview card updates immediately without executing any
   scripts or loading unexpected images, and a confirmation message confirms
   the save. Refreshing the page should persist the new values.

## Viewer channel playback and replays

1. Open a channel page while the backend API is temporarily unreachable (for
   example, by stopping the API server).

   *Expected:* the page shows a recovery surface explaining the failure with a
   **Try again** button and a link back to **Browse**. Clicking **Try again**
   should retry and succeed once the API is reachable again.

2. Switch to the **Videos** tab on any channel with no VODs.

   *Expected:* a short "Loading past broadcasts…" placeholder appears first,
   then the "No VODs yet" empty state once the fetch resolves. If the VOD API
   returns an error, the gallery shows a retry button that clears the error and
   reloads the list.

## Control center installer wizard

1. Sign in as an administrator, open **Administration** -> **Settings**, and
   scroll to **Install BitRiver Live**.

   *Expected:* the installer opens on **Welcome** with the 7-step progress bar
   visible, Quick Install language emphasized, and no advanced service fields
   visible yet.

2. Click **Run system check**.

   *Expected:* the wizard advances to **System Check**, shows pass/warning/fail
   rows with plain-language actions, includes a **Refresh checks** button, and
   keeps raw host details hidden until **Show technical details** is expanded.

3. Confirm the host-readiness messaging uses live server-side results when the
   endpoint is available.

   *Expected:* checks mention the real host shape (for example supported
   Ubuntu/systemd state, installer tools, paths, port readiness, TLS files, or
   external service reachability) instead of only browser-side warnings. If the
   endpoint is temporarily unavailable, the wizard should show the fallback
   guidance message and still allow **Continue** and **Refresh checks**.

4. Continue to **Install Mode** and **Core Settings**.

   *Expected:* **Quick Install** is selected by default and marked
   recommended. Switching to **Advanced Install** reveals the database, TLS,
   Redis, and lower-level service fields; switching back to **Quick Install**
   hides them again.

5. Fill in required fields with at least one deliberate validation error first
   (for example an invalid email or weak password), then correct it.

   *Expected:* inline helper text stays visible for every field, the validation
   error appears next to the affected input, and all entered values remain in
   place after the error and after fixing it.

6. Continue to **Review** and expand **Show technical details**.

   *Expected:* the review step summarizes the chosen install path, sign-in
   details, storage/services, and any advanced technical choices before the
   handoff starts.

7. Click **Start handoff** and then continue to **Success**.

   *Expected:* the **Installing** step uses the progress UI rather than raw log
   output, the generated command stays available behind technical details, and
   the **Success** step shows the app URL, admin URL, config path, data path,
   and next steps. Using **Back** and **Start over** should keep the flow
   predictable instead of dropping you into a blank state.
