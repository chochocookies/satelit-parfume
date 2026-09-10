# Satelit Parfume — Mobile (Capacitor)

Phase 14's shell: wraps the existing web app (`apps/web`) as a native
Android/iOS app via [Capacitor](https://capacitorjs.com), rather than
building a second, separate mobile app from scratch.

## What's actually set up here

- `package.json` — `@capacitor/core`, `@capacitor/android`, `@capacitor/ios`,
  `@capacitor/cli`, all pinned to the same 8.5.1 line (Capacitor requires
  its core/cli/platform packages to stay in sync). Installed and
  verified with `npx cap doctor` — dependencies resolve cleanly.
- `capacitor.config.ts` — generated with `npx cap init`, then hand-edited
  to add the `server` block (see "Why server mode" below). Read the
  comments in that file before changing anything in it.

## What's deliberately NOT done here, and why

`npx cap add android` and `npx cap add ios` — the commands that actually
generate the native Android Studio / Xcode projects — were not run.
This isn't a scope choice; it's a hard environment limit: generating
those projects needs the Android SDK/Gradle and Xcode respectively,
neither of which exist in the sandbox this was built in, and that
sandbox's network access is restricted to a small allowlist that
doesn't reach Google's or Apple's own package repositories either. Any
attempt would either fail outright or produce a scaffold that looks
complete but silently isn't.

**Run these yourself, from a machine with the right tooling:**

```bash
cd apps/mobile
npm install                # if you haven't already
npx cap add android        # needs Android Studio + its SDK on PATH
npx cap add ios            # needs Xcode, macOS only
npx cap sync               # re-run this after any change to
                            # capacitor.config.ts or the native folders
```

`cap add` creates `android/` and `ios/` folders here containing real,
buildable native projects — open them directly in Android Studio /
Xcode from that point on (`npm run open:android` / `npm run
open:ios` once they exist).

## Why server mode, not a bundled static build

`capacitor.config.ts`'s `server.url` points the native WebView straight
at the deployed web app, instead of Capacitor's other common mode:
bundling a static export of the site inside the app itself. That second
mode needs `apps/web` to build with Next.js's `output: "export"` —
apps/web currently uses `output: "standalone"` (a real Node server, see
its own `next.config.ts`), and switching that over would mean auditing
every route this codebase has for server-only behavior (cookies,
headers, server-side data fetching) with no way to actually verify the
result builds, in this sandbox at least (`next build` itself needs
`fonts.googleapis.com`, which the same sandbox can't reach — see the
root README's "Verification notes"). Pointing at a live URL sidesteps
all of that risk entirely: the web app keeps deploying exactly as it
already does, and this is just a thin native shell around it.

**The real implication:** this native app needs `apps/web` actually
deployed somewhere reachable — a phone on a cellular network can't
reach your laptop's `localhost`. For local testing:

- **Android emulator**: the config's default,
  `http://10.0.2.2:3000`, is the emulator's own standing alias for
  your host machine's `localhost` — run `npm run dev` in `apps/web`
  normally and the emulator build will reach it.
- **iOS simulator**: unlike Android's emulator, the iOS simulator
  shares your Mac's network stack directly — `http://localhost:3000`
  works as-is. Set `CAPACITOR_SERVER_URL=http://localhost:3000` before
  `cap sync` when building for iOS specifically.
- **A real device, or anything beyond your own local testing**: set
  `CAPACITOR_SERVER_URL` to your actual deployed domain (Phase 15 —
  production hardening — is what gets that domain live with a real
  cert) before running `cap sync`, and drop the plain-HTTP default
  entirely at that point.

## App icon / splash screen

Phase 13's generated icon set (`apps/web/public/icon-512.png`, the "SP"
monogram in the brand's own colors) is a reasonable starting point for
the native app icon too, but Capacitor needs its own set of
platform-specific sizes — use
[`@capacitor/assets`](https://github.com/ionic-team/capacitor-assets)
once `android/`/`ios/` exist to generate those automatically from a
single 1024×1024 source image, rather than hand-exporting every size
Android and iOS each expect.

## App store submission

Both stores need more than a working build: a registered developer
account (Google Play: one-time fee; Apple: annual fee), privacy policy
URL, store listing copy and screenshots, and — for Android specifically
— a signing keystore you generate and keep permanently (losing it means
losing the ability to publish updates to an already-listed app). None
of that is blocked on anything in this repo; it's account/business
setup that happens outside of it, whenever you're ready to submit.
