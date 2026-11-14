# Bundled asset inventory

Every bundled image or logo must have documented provenance before a public release. A file being present in the repository does not prove that Citadel owns it or may redistribute it. Until the copyright holder records the source and license, the status of every row below is **UNVERIFIED — DO NOT PUBLISH AS FINAL**.

| SHA-256 | Files | Required evidence |
| --- | --- | --- |
| `a875fcf9a3f214801984bf960e2098ece3ea99e4f350aeb02b9743ff0e40b618` | `build/appicon.png`, `frontend/app/assets/img/citadel_sentry_round.png`, `frontend/public/img/citadel_sentry_round.png` | Creator/source file and Citadel redistribution rights |
| `6c7ee9798418de87664c9d659789e1254effb89441367c7bf5953eb03ae7e828` | `build/windows/icon.ico` | Derivation source and redistribution rights |
| `271a7358582c92acbb36f49930ea9db3b43de6ed7b6e752d985089e220008178` | `frontend/app/assets/img/logo_citadel.png`, `frontend/public/img/logo_citadel.png` | Logo owner approval and trade mark policy |
| `9fc4f75c2c45772b7d8d752853c24f29b353ad0340fafa5362b66d7b1b0d938b` | `frontend/public/img/citadel_sentry_final.jpg` | Creator/source file and Citadel redistribution rights |
| `27c04baadd2fc0caea93a26ceed9a35a9bf966b2e4a9f67b86f5888758b622e0` | `frontend/public/img/citadel_sentry_gold.jpg` | Creator/source file and Citadel redistribution rights |
| `f99c33af6dce51ad954665bfad14a5553e3d2f99d1828536f9cbe519a0626632` | `frontend/public/img/logo-citadel.svg` | Original vector source, author, and logo redistribution terms |
| `762c17f514d35d9b875901100cf5acb8ae13f606ab45765024a446373cc10976` | `frontend/public/img/logo_citadel_512.png` | Logo owner approval and trade mark policy |

Before release, replace **UNVERIFIED** with one of:

- `Citadel-owned` plus author/date/source evidence;
- the exact third-party license and attribution;
- `removed from public artifact`.

Also generate dependency notices for the Go and npm production dependency trees. The root `LICENSE` covers Citadel's code only and does not replace third-party license obligations.
