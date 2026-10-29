# Chartrove

**Your health history. In your hands.**

Chartrove is an independently maintained, open-source personal and family health record application, forked from [Fasten Onprem](https://github.com/fastenhealth/fasten-onprem). It brings medical records together in a desktop application or a self-hosted installation.

Chartrove is a separate project from Fasten Health and Fasten Connect. The original source history and attribution are retained; Fasten's hosted services, commercial support, client registrations, and policies do not become Chartrove services through this fork.

## Project status

This is a development preview. The macOS desktop host is being revived around the existing Go backend and Angular frontend. Local builds are ad hoc signed and not notarized. No production-ready Chartrove desktop release or container image is being advertised here.

- Import FHIR bundles and organize records, timelines, conditions, labs, and practitioners.
- Keep desktop records in a local database, or run the backend on your own server.
- Use separate accounts for different people's records.
- Test the restored Epic connection with published **synthetic sandbox data only**. Full UI validation is still in progress; production provider connections are not ready.

Fasten's retired production integration infrastructure is not restored by renaming this project. Some inherited features still reference external services. The legacy CCDA converter, for example, sends the selected document to a Fasten Health service; it is separate from Chartrove and remains unvalidated for this fork. Prefer local FHIR bundle imports during development.

## Desktop development

Use macOS with Xcode Command Line Tools, Go 1.26 or newer, and Mise Node 26.13.0. The frontend uses the committed Yarn lockfile; the desktop module pins Wails v3.0.0. The Angular migration needed for supported Node 26 compatibility is in progress; see [the toolchain status](CONTRIBUTING.md#toolchain).

```sh
cd frontend
npm exec --yes --package=corepack@0.35.0 -- corepack yarn install --frozen-lockfile --ignore-optional --registry https://registry.npmjs.org --network-concurrency 4
cd ..
sh desktop/build.sh
```

The build creates `dist/Chartrove.zip` containing `Chartrove.app`. It does not install the app or open an existing database. See the [desktop guide](desktop/README.md) for architecture, signing, test commands, and native smoke validation.

To run with a fresh, disposable test database after extracting the ZIP:

```sh
test_data=$(mktemp -d "${TMPDIR:-/tmp}/chartrove-test.XXXXXX")
Chartrove.app/Contents/MacOS/chartrove --unencrypted --data-dir "$test_data"
```

Use synthetic data in that directory. Normal startup enables database encryption; save the recovery key when setting it up.

## Existing data

The branding update does not migrate, rename, or open existing health records. The desktop host retains its existing `FastenCommunity` data-directory name and bundle identifier for compatibility. Internal database names, encryption behavior, provider identifiers, and the Wails bridge contract also remain unchanged. The purchased Fasten application's storage is separate and is not a test input.

Do not edit or regenerate `healthit.yaml` as part of this rebrand.

## Documentation

- [Contributing and development](CONTRIBUTING.md)
- [Desktop build and testing](desktop/README.md)
- [Provider connection validation](desktop/PROVIDER_VALIDATION.md)
- [Replacing Lighthouse for production](desktop/LIGHTHOUSE_REPLACEMENT.md)
- [Branding and compatibility boundaries](docs/BRANDING.md)
- [Current desktop branding validation](desktop/BRANDING_VALIDATION.md)

Historical Fasten documentation is available in the [upstream docs repository](https://github.com/fastenhealth/docs). It describes upstream behavior and may not apply to Chartrove.

## Support and contributions

Use [Chartrove issues](https://github.com/karamhouse/chartrove/issues) and [pull requests](https://github.com/karamhouse/chartrove/pulls). Public reports must omit health information, credentials, and identifying details.

The upstream open backlog was copied with original-author credit and discussion links. Migrated PRs are drafts against historical base commits; they require review and retargeting or rebasing before integration into `main`. Copied review transcripts are not new approvals. GitHub Actions remain disabled while the inherited workflows are assessed.

## Fasten attribution and license

Chartrove derives from [Fasten Onprem](https://github.com/fastenhealth/fasten-onprem), originally developed by **Jason Kulatunga (AnalogJ)** and **Alex Szilagyi (alexszilagyi)**, with contributions from the Fasten community. Their work, copyright notices, and original Git history are preserved.

See [LICENSE.md](LICENSE.md) for the existing license and notices. This branding change does not replace the upstream license or imply endorsement by Fasten Health.
