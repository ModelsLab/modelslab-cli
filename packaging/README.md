# Packaging

The CLI is a single Go binary. Homebrew, Scoop, deb and rpm are produced by
goreleaser directly; npm and PyPI are not registries goreleaser publishes to, so
they are built here from the binaries goreleaser has already produced.

Nothing in this directory compiles anything. Both builders take a directory of
extracted release binaries and repackage them, so every channel ships the same
bytes for a given tag.

```
artifacts/
  darwin_amd64/modelslab
  darwin_arm64/modelslab
  linux_amd64/modelslab
  linux_arm64/modelslab
  windows_amd64/modelslab.exe
  windows_arm64/modelslab.exe
```

## npm — `packaging/npm/build.mjs`

```bash
node packaging/npm/build.mjs v1.2.3 artifacts dist/npm
```

Produces seven packages: one entry package (`modelslab-cli`) whose only content
is a launcher shim, plus one package per platform holding just the binary and
the `os`/`cpu` fields npm filters on. The entry package declares the six as
`optionalDependencies`, so npm installs exactly the one that matches.

The alternative — one package with a postinstall that downloads a binary — was
rejected deliberately. It needs network at install time and produces a silently
broken install under `npm ci --ignore-scripts`, which many CI and agent sandboxes
set. Six small packages buy an install that cannot half-work.

**Publish platform packages before the entry package.** The entry package pins
exact versions of all six; publishing it first leaves a window where every
install fails.

## PyPI — `packaging/pypi/build.py`

```bash
python3 packaging/pypi/build.py v1.2.3 artifacts dist/pypi
```

Produces one wheel per platform, each containing the binary and a console script
that `execv`s it. Wheels are written directly rather than through a build
backend: there is nothing to compile, so a backend would only add a dependency,
and writing them here keeps the platform tag explicit instead of inferred from
whatever host ran the build.

Two things that are easy to get wrong and are covered by CI:

- **Version normalisation.** Tags are semver, wheel filenames are PEP 440. A
  `v1.2.3-rc1` tag naively becomes `modelslab_cli-1.2.3-rc1-py3-none-*.whl`,
  which pip reads as version `1.2.3` with build tag `rc1` — and build tags must
  start with a digit, so the file is invalid. `normalise_version()` converts it
  to `1.2.3rc1`.
- **The executable bit.** pip decides whether to mark an unpacked file executable
  with `stat.S_ISREG(mode) and mode & 0o111`, so the zip entry's mode has to
  carry the file-type bits, not just permissions. A bare `0o755` fails `S_ISREG`,
  the binary lands `0o644`, and the first run dies with `EPERM`. `twine check`
  passes either way; only installing and running catches it.

## Releasing

`.github/workflows/release.yml` runs both builders on a tag and publishes if the
corresponding token is configured. Missing tokens skip that registry rather than
failing the release.

| Secret | Registry |
| --- | --- |
| `NPM_TOKEN` | npm (automation token with publish rights) |
| `PYPI_TOKEN` | PyPI (project or account API token, used as `__token__`) |

`.github/workflows/ci.yml` builds both on every PR and installs and *runs* the
result, because both of the failure modes above pass every static check.
