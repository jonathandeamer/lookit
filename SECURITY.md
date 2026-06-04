# Security policy

## Supported versions

lookit is pre-1.0; only the latest release (and `main`) receives security fixes.

## Reporting a vulnerability

Please report security issues **privately** — not as a public issue or pull request.

- Preferred: GitHub [private vulnerability reporting](https://github.com/jonathandeamer/lookit/security/advisories/new) (the "Report a vulnerability" button on the repository's Security tab).
- Fallback: email `jonathan@tilde.team`.

This is a hobby project maintained by one person, so there is no formal SLA, but you can expect an acknowledgement within a few days. Credible reports will be investigated and fixed, and I am happy to credit you in the published advisory.

## Scope

lookit's threat model is the **untrusted finger response**: it renders bytes from arbitrary remote servers (TCP/79) to your terminal. The most relevant issues are things like:

- escape or control sequences reaching the terminal despite the `sanitize` ingress filter (terminal repainting, OSC-52 clipboard injection, and similar);
- a crafted response that wedges or exhausts the parser;
- a server-supplied target or link escaping the port-79 pin to reach another service.

Vulnerabilities in lookit's own handling of that untrusted network input are in scope. Issues that require already having code execution on the user's machine, or that only affect a forked or modified build, are generally out of scope.
