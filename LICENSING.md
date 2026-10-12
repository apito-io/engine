# Licensing

Copyright 2024–2026 Apito LLC.

## This tree, going forward

The Apito Engine server is **Fair Source** under the Functional Source License, Version 1.1, ALv2 Future License.

- SPDX identifier: `FSL-1.1-ALv2`
- Also identified as: `FSL-1.1-Apache-2.0`
- Future license: Apache License, Version 2.0
- Change: the second anniversary of the date that version is made available

Do not describe a current FSL version as open source or as Apache-licensed. It is Fair Source. It converts to Apache 2.0 on its published change date.

## Per-release dates

Each published engine tag gets an immutable pair:

| Field | Rule |
| --- | --- |
| `availability_date` | UTC calendar date the tag is made available |
| `apache_change_date` | `availability_date` plus exactly two years |

Example only (not a release): a version made available on 2026-10-11 converts on 2028-10-11. The release manifest written by CI is the record. Do not edit a published date.

Unreleased commits in git have no availability date until a tag is published.

## Already published Apache versions

Tags published while this repository’s `LICENSE` was the Apache License 2.0 stay Apache 2.0 forever. That includes `v1.8.25` and every earlier engine tag. The text that shipped on `v1.8.25` is in `licenses/APACHE-2.0-PUBLISHED.txt`. Changing the default branch does not relicense those tags.

## Apache 2.0 inside this repository

These paths are Apache License 2.0 now (`licenses/APACHE-2.0.txt`), including in FSL-tagged engine releases:

- `interfaces/` — driver and plugin interfaces
- `schemas/` — schema and blueprint formats
- Documented wire protocols shipped beside those interfaces

`open_driver`, the CLI, SDKs, templates, and examples are separate repositories and are Apache 2.0 immediately, not FSL.

## What Fair Source allows

Commercial internal use, self-hosting, and professional services for someone who is themselves a licensee are permitted. Offering the engine as a competing hosted substitute is not, until that version’s Apache change date.

Self-hosting has no project, tenant, user, or performance license key and does not phone home.

## Contributor inbound license

See `CONTRIBUTING.md`. There is no CLA in this repository. Counsel decides whether a narrow contributor agreement is required before any FSL patch is copied into private Apito Cloud code.
