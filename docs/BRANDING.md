# Chartrove branding and compatibility

Chartrove is an independent fork of Fasten Onprem. The repository is [karamhouse/chartrove](https://github.com/karamhouse/chartrove). Fasten's original authors, source history, copyright notices, and license remain credited in the README, license file, and application footer.

## Product identity

- Product and native application name: **Chartrove**.
- macOS package: `Chartrove.app`, distributed as `Chartrove.zip`.
- Executable: `Chartrove.app/Contents/MacOS/chartrove`.
- Header, authentication pages, browser title, support links, and generated practitioner CSV branding use Chartrove.
- The purple C monogram is generated from type; original Fasten image assets are retained in history and are not presented as the Chartrove logo.
- Chartrove support belongs in this repository. The UI does not present Fasten's commercial support, mailing list, or legal policies as Chartrove services.

## Names retained for compatibility

| Identifier | Reason |
| --- | --- |
| `~/Library/Application Support/FastenCommunity` | Preserve the existing desktop host's data selection |
| `house.karam.fasten-community` | Preserve native/WebView identity |
| `fasten.db`, signing-key and lock filenames | Avoid changing database, encryption, or concurrent-instance behavior |
| Go module/import paths and `fasten:source-auth-response` | Keep backend imports and the Wails bridge contract compatible |
| Provider `platform_type: fasten`, API/environment keys | Preserve stored source and configuration semantics |
| Fasten Lighthouse/client/callback URLs | External service identities require their own replacement and registration work |

Changing a display name does not migrate records. This task must not read, edit, regenerate, or migrate `healthit.yaml` or an existing health-record database. Validate with a fresh synthetic database and explicit `--data-dir` only.

## Historical evidence

Earlier validation receipts, binary hashes, archived build names, and the original desktop revival plan describe the builds that actually ran. Do not relabel old evidence as a successful Chartrove build. Record current package and UI validation separately.

The purchased Fasten app is preserved independently; Chartrove's branding does not replace or modify it.
