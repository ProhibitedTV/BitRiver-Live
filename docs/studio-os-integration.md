# Publish from BitRiver Studio OS

BitRiver-Live accepts **authenticated RTMP ingest** from OBS or an upstream
publisher. The first integration with
[BitRiver Studio OS](https://github.com/ProhibitedTV/BitRiver-Studio-OS)
exchanges only public connection metadata, never creator secrets.

The Creator Live page now offers **Download Studio OS profile** for a channel
you own, when an RTMP ingest server is available. The downloaded JSON:

- uses schema `bitriver.studio.ingest/v1`;
- records `provider: "bitriver-live"`, a channel ID, RTMP server URL and public
  channel viewer URL;
- **does not** contain a stream key, cookie, login token or account password.

Only a canonical `rtmp://` or `rtmps://` server ending in `/live` is
exported. If your channel does not advertise a supported endpoint, the
download is disabled or fails closed. Do not use an API/control-plane address
as an RTMP server.

## On the Studio OS computer

1. Transfer the downloaded JSON to that machine.
2. Run the Studio OS `import-bitriver-live-profile.py` tool in `--dry-run`
   mode to inspect the destination.
3. On the local computer, have the unprivileged `bitriver` service user
   run the tool without `--dry-run` and enter the stream key at the hidden
   prompt. The key file lives outside the versioned configuration.
4. Add the **disabled** destination fragment to the Studio OS draft
   `mediaRouter.publishDestinations`. Validate and explicitly apply it.
   Enable the destination only when you intend to broadcast.
5. Confirm OBS publishing to local MediaMTX, forwarding to the *chosen*
   BitRiver-Live instance's SRS, and live viewer playback. Local service
   liveness does not establish end-to-end success.

See the complete [Studio OS ecosystem integration guide](https://github.com/ProhibitedTV/BitRiver-Studio-OS/blob/main/docs/ECOSYSTEM_INTEGRATION.md)
for the manual draft settings step, limits and security considerations.

**Current scope:** The export does not log Studio OS into your Live account,
push keys between servers, launch a stream, or register an instance. It is a
manual, user-controlled interoperability bridge. Multi-user FFmpeg process
argument secrecy remains a release blocker on the Studio OS side. Do not
claim secure unattended multi-platform streaming until that work is complete.
