# Legal review gate

Status: pending counsel

Do not publish a public FSL engine or console tag until counsel signs this file by replacing the status line with `Status: signed` and adding the signer, date, and scope. CI refuses a Fair Source release while the status is `pending counsel`.

This file is the review packet. It is not a legal opinion.

## What counsel is asked to confirm

1. The `LICENSE` text is the unmodified Functional Source License, Version 1.1, ALv2 Future License (SPDX `FSL-1.1-ALv2`), which Apito identifies as FSL-1.1-Apache-2.0.
2. Future engine and console releases use that license. Tags already published as Apache 2.0, including engine `v1.8.25` and earlier, remain Apache 2.0. Nothing in the release process rewrites those tags.
3. Each release manifest records `availability_date` and `apache_change_date` exactly two years later, and those dates are immutable after publish.
4. `interfaces/`, `schemas/`, and documented protocols in the engine repo are Apache 2.0. `open_driver`, CLI, SDKs, templates, and examples are Apache 2.0 in their own repositories.
5. `CONTRIBUTING.md` states the inbound license. No CLA is added unless counsel asks for a narrow agreement that allows copying an FSL contribution into private Cloud code.
6. `TRADEMARK.md` is the public trademark policy. Begin word and logo registration in the jurisdictions Apito actually sells in.
7. Public copy says “Fair Source — converts to Apache 2.0 after two years.” It does not say the current engine or console is OSI open source or Apache-licensed.
8. Tracked secrets (`BRANKA_KEY`, admin reset tokens, Paddle, PostHog) are not part of the public tree. Public trees ship `.env.example` only.

## Sign-off

| Field | Value |
| --- | --- |
| Status | pending counsel |
| Signer | |
| Date | |
| Scope | engine + console FSL cutover |
