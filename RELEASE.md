# Release runbook

The steps to ship a new orcha version.


## 0. Prerequisites (one-time)

- A clean working tree (`git status` shows nothing).
- Go 1.21+ and Python 3.9+ on PATH.
- `python3 -m pip install --user build twine` for building and uploading wheels.
- A PyPI account with a project-scoped API token. Save it to `~/.pypirc`:

  ```ini
  [pypi]
    username = __token__
    password = pypi-<your-token-here>
  ```

- Repository write access to `github.com/ryfoo/orcha`.

## 1. Bump the version

Edit the single source of truth:

```bash
echo "$VERSION" > VERSION
```

Both the Go binary (via `-ldflags`) and the Python wheel (via a sed pass on
`__init__.py`) read from this file at build time.

## 2. Build the release artifacts

```bash
make release
```

This produces, in `dist/`:

- `orcha-linux-amd64`, `orcha-linux-arm64`
- `orcha-darwin-amd64`, `orcha-darwin-arm64`
- `orcha-windows-amd64.exe`
- `manifest.json` (URL + sha256 for each binary)
- `orcha-$VERSION-py3-none-any.whl`
- `orcha-$VERSION.tar.gz`

Sanity-check the manifest:

```bash
cat dist/manifest.json
```

## 3. Run tests

```bash
make test
( cd python && python3 -m pytest tests/ )
```

Both must pass before tagging.

## 4. Commit + tag

```bash
git add VERSION python/orcha/__init__.py
git commit -m "Release v$VERSION"
git tag -a "v$VERSION" -m "Orcha v$VERSION"
git push origin main
git push origin "v$VERSION"
```

## 5. Create the GitHub Release

```bash
gh release create "v$VERSION" \
  --title "v$VERSION" \
  --notes "See CHANGELOG / commit log for details." \
  dist/orcha-linux-amd64 \
  dist/orcha-linux-arm64 \
  dist/orcha-darwin-amd64 \
  dist/orcha-darwin-arm64 \
  dist/orcha-windows-amd64.exe \
  dist/manifest.json
```

After this completes, the URLs in `manifest.json` resolve to real downloads
and the Python downloader will find the binary for the user's platform.

## 6. Publish to PyPI

```bash
python3 -m twine check  dist/orcha_dev-$VERSION-py3-none-any.whl dist/orcha_dev-$VERSION.tar.gz
python3 -m twine upload dist/orcha_dev-$VERSION-py3-none-any.whl dist/orcha_dev-$VERSION.tar.gz
```

> Note: PyPI normalizes the distribution name `orcha-dev` to `orcha_dev` in
> wheel and sdist filenames (PEP 503). The import path stays `import orcha`.

`twine check` validates the wheel's metadata (long_description, classifiers,
etc.) before you push; if it fails, fix the issue and rebuild — do not upload
a broken wheel.

## 7. Verify end-to-end

In a fresh shell, ideally a different machine or a clean venv:

```bash
python3 -m venv /tmp/orcha-test && source /tmp/orcha-test/bin/activate
pip install orcha-dev
orcha version          # should print $VERSION
orcha run --help       # should show the `run` subcommand
```

The first `orcha run` against an actual pipeline will trigger the binary
download to `~/.orcha/bin/`. If that succeeds and the sha256 matches, the
release is good.

## Recovering from a bad release

- **Wrong artifact uploaded to GitHub:** delete the release, run
  `git tag -d v$VERSION && git push origin :refs/tags/v$VERSION`, fix, retry.
  Re-tagging is fine before anyone has consumed the artifacts.
- **Bad wheel on PyPI:** PyPI does not allow re-uploading a yanked filename.
  Yank the version (`pip install pypi-cli` or via the web UI), bump the
  patch version, and ship `$VERSION+1`. There is no other safe path.
