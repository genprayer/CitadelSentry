# Publishing, licensing, and repository hygiene

## Choose the publication model first

There are three distinct choices:

1. **Private source + public binaries/docs.** Best when preventing reuse is important. Users receive signed or checksummed releases without receiving implementation source.
2. **Public source + all-rights-reserved notice.** Good for transparency and review, but it is source-available, not open source. The notice does not stop someone from copying code technically; it states that copying, redistribution, and derivatives are not permitted without written permission.
3. **Public source + open-source license.** Only choose this when reuse by other projects is intentional. Select the license deliberately and ensure every dependency and asset is compatible.

The repository currently uses `LICENSE` with an all-rights-reserved notice and `UNLICENSED` package metadata. Keep that consistent until the copyright holder decides otherwise. Copyright protection is normally automatic and does not require registration or a fee under the Berne framework ([WIPO](https://www.wipo.int/en/web/copyright/protection)). Registration, trademark work, contract drafting, takedowns, and legal enforcement can still cost money and depend on jurisdiction.

For a German national trade mark, the DPMA currently lists a basic fee of EUR 290 for electronic filing covering up to three classes, plus EUR 100 for each additional class ([DPMA fee table](https://www.dpma.de/english/services/fees/trademarks/index.html)). That fee is for trade mark registration, not for copyright in the code.

GitHub also notes that a public repository without an open-source license remains under default copyright, but GitHub users may still view and fork a public repository under the platform terms ([GitHub licensing documentation](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/licensing-a-repository)). If limiting access and copying matters, keep the source private and publish binaries plus documentation only. Existing clones cannot be recalled by making a repository private later.

Obtain qualified legal advice for a public EULA, privacy notice, liability language, export rules, and enforcement strategy. This document is engineering guidance, not legal advice.

## Safe public contents

Publish only what users need:

- release binaries and matching SHA-256 checksums;
- installation, testing, safety, and support documentation;
- a changelog and a clear version identifier;
- assets that Citadel owns or is licensed to redistribute;
- source code only if the chosen source-publication model permits it.

Never publish:

- `.env`, API keys, OAuth/client secrets, auth caches, local `config.json`, SSH keys, signing certificates, or private server addresses;
- `node_modules`, Nuxt output, Wails build output, local logs, screenshots containing tokens, or temporary test databases;
- Squad/Steam proprietary binaries, extracted game assets, or fonts/images without redistribution rights;
- internal infrastructure manifests or admin endpoints that are not part of the end-user contract.

The current asset inventory and unresolved provenance gate are recorded in [`ASSETS.md`](../ASSETS.md). Do not publish the source or binary as a final public release until each bundled image/logo has an owner or redistribution license recorded and required third-party notices have been generated.

## GitHub release checklist

1. Review the exact diff and scan the tree for secrets and private paths.
2. Run the complete gates in [`TESTING.md`](TESTING.md).
3. Build Linux and Windows artifacts from a clean, reproducible checkout.
4. Generate checksums and, when available, sign the release artifacts.
5. Upload artifacts to a release page; do not commit generated binaries to the source tree unless that is an explicit repository policy.
6. Publish release notes with supported OS versions, known limitations, rollback instructions, and the private security contact.
7. Keep at least the previous known-good artifact available for rollback.
