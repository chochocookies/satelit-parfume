import type { CapacitorConfig } from '@capacitor/cli';

// appId is a reverse-domain identifier that becomes permanent the
// moment this app is first published to the Play Store / App Store —
// changing it later means publishing as a brand new app listing, not
// an update. com.satelitparfume.app is a reasonable placeholder given
// this project's own name; swap it for whatever domain/bundle
// convention actually gets registered before a real store submission.
const config: CapacitorConfig = {
  appId: 'com.satelitparfume.app',
  appName: 'Satelit Parfume',
  // webDir is required by Capacitor's schema but goes unused here — see
  // server.url below. It'd only matter if this switched to bundling a
  // static export inside the app itself instead.
  webDir: 'www',
  server: {
    // Points the native WebView straight at the deployed web app rather
    // than bundling a static export inside the native shell. Chosen
    // over static export because this app (apps/web) is a genuinely
    // dynamic, server-rendered Next.js app — next.config.ts already
    // declares output: "standalone" (a Node server, not a static site),
    // and switching that to output: "export" would need auditing every
    // route for server-only APIs (cookies(), headers(), server-side
    // data fetching) this codebase already uses in places, with no way
    // to verify the result actually builds from this sandbox (next
    // build itself needs fonts.googleapis.com, which is blocked here —
    // see the root README's own "Verification notes"). Pointing at a
    // live URL sidesteps all of that: the web app keeps deploying
    // exactly as it already does, and this shell just wraps it.
    //
    // CAPACITOR_SERVER_URL below is a placeholder for local testing
    // only — see this folder's README.md for how to point this at a
    // real deployment before building for a device or an app store.
    url: process.env.CAPACITOR_SERVER_URL || 'http://10.0.2.2:3000',
    cleartext: process.env.CAPACITOR_SERVER_URL === undefined,
  },
};

export default config;
