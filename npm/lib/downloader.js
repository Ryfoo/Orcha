'use strict';

// Resolve and (lazily) download the orcha Go binary for the current platform.
//
// The binary lives at ~/.orcha/bin/orcha-<os>-<arch>[.exe]. On first call we:
//
//   1. Fetch a manifest.json from the GitHub release that matches this npm
//      package's version.
//   2. Look up the entry for our platform — it gives us a binary URL and the
//      expected sha256.
//   3. Download the binary, verify the hash, mark it executable, cache it.
//
// Subsequent runs find the cached file and skip the network entirely. The
// behavior matches the Python SDK's downloader.py byte-for-byte so a single
// GitHub release serves both ecosystems.
//
// Override hooks (env):
//   ORCHA_BINARY_PATH    — absolute path to a pre-built binary (no download
//                          or hash check is run).
//   ORCHA_BINARY_VERSION — pin a specific release tag instead of using the
//                          npm package version.

const crypto = require('node:crypto');
const fs = require('node:fs');
const fsp = require('node:fs/promises');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');
const { URL } = require('node:url');

const { OrchaError } = require('./errors');
const pkg = require('../package.json');

const MANIFEST_URL_TEMPLATE = 'https://github.com/ryfoo/orcha/releases/download/v{version}/manifest.json';

function detectPlatform() {
  const osMap = { linux: 'linux', darwin: 'darwin', win32: 'windows' };
  const archMap = { x64: 'amd64', arm64: 'arm64' };

  const sys = os.platform();
  const arch = os.arch();
  if (!(sys in osMap)) throw new OrchaError(`unsupported OS: ${sys}`);
  if (!(arch in archMap)) throw new OrchaError(`unsupported architecture: ${arch}`);
  return { osName: osMap[sys], arch: archMap[arch] };
}

function binaryFilename(osName, arch) {
  let name = `orcha-${osName}-${arch}`;
  if (osName === 'windows') name += '.exe';
  return name;
}

function installDir() {
  return path.join(os.homedir(), '.orcha', 'bin');
}

// Wrap https.get so we can transparently follow GitHub release redirects
// (objects.githubusercontent.com) without pulling in a dep.
function httpsGet(url, opts = {}, redirectsLeft = 5) {
  return new Promise((resolve, reject) => {
    const req = https.get(url, opts, (res) => {
      const status = res.statusCode || 0;
      if (status >= 300 && status < 400 && res.headers.location) {
        if (redirectsLeft <= 0) {
          res.resume();
          reject(new OrchaError(`too many redirects fetching ${url}`));
          return;
        }
        const next = new URL(res.headers.location, url).toString();
        res.resume();
        httpsGet(next, opts, redirectsLeft - 1).then(resolve, reject);
        return;
      }
      resolve(res);
    });
    req.on('error', reject);
    req.setTimeout(120_000, () => {
      req.destroy(new OrchaError(`timeout fetching ${url}`));
    });
  });
}

async function fetchManifest(version) {
  const url = MANIFEST_URL_TEMPLATE.replace('{version}', version);
  let res;
  try {
    res = await httpsGet(url);
  } catch (e) {
    throw new OrchaError(`failed to fetch manifest at ${url}: ${e.message}`);
  }
  const status = res.statusCode || 0;
  if (status === 404) {
    res.resume();
    throw new OrchaError(
      `orcha release v${version} not found at ${url}. Either the release ` +
      `hasn't been published yet, or ORCHA_BINARY_VERSION points at a tag ` +
      `that doesn't exist.`,
    );
  }
  if (status < 200 || status >= 300) {
    res.resume();
    throw new OrchaError(`failed to fetch manifest at ${url}: HTTP ${status}`);
  }
  const chunks = [];
  for await (const chunk of res) chunks.push(chunk);
  const body = Buffer.concat(chunks).toString('utf8');
  try {
    return JSON.parse(body);
  } catch (e) {
    throw new OrchaError(`manifest at ${url} was not valid JSON: ${e.message}`);
  }
}

async function downloadTo(url, dest) {
  const tmpPath = `${dest}.tmp-${process.pid}-${Date.now()}`;
  let res;
  try {
    res = await httpsGet(url);
  } catch (e) {
    throw new OrchaError(`download failed (${url}): ${e.message}`);
  }
  const status = res.statusCode || 0;
  if (status < 200 || status >= 300) {
    res.resume();
    throw new OrchaError(`download failed (${url}): HTTP ${status}`);
  }
  await new Promise((resolve, reject) => {
    const out = fs.createWriteStream(tmpPath);
    res.pipe(out);
    out.on('finish', () => out.close(resolve));
    out.on('error', reject);
    res.on('error', reject);
  });
  try {
    await fsp.rename(tmpPath, dest);
  } catch (e) {
    await fsp.unlink(tmpPath).catch(() => {});
    throw e;
  }
}

async function sha256(filePath) {
  const hash = crypto.createHash('sha256');
  const stream = fs.createReadStream(filePath);
  for await (const chunk of stream) hash.update(chunk);
  return hash.digest('hex');
}

// resolveBinary returns the cached binary path, downloading + verifying it
// the first time. Safe to call from multiple processes — concurrent downloads
// race only on the final rename, which is atomic on the platforms we target.
async function resolveBinary(version) {
  const override = process.env.ORCHA_BINARY_PATH;
  if (override) {
    if (!fs.existsSync(override)) {
      throw new OrchaError(`ORCHA_BINARY_PATH points to missing file: ${override}`);
    }
    return override;
  }

  const v = version || process.env.ORCHA_BINARY_VERSION || pkg.version;
  const { osName, arch } = detectPlatform();
  const name = binaryFilename(osName, arch);
  const target = path.join(installDir(), name);
  if (fs.existsSync(target)) return target;

  await fsp.mkdir(path.dirname(target), { recursive: true });

  const manifest = await fetchManifest(v);
  const platformKey = `${osName}-${arch}`;
  const entry = manifest?.binaries?.[platformKey];
  if (!entry) {
    const known = Object.keys(manifest?.binaries || {}).sort();
    throw new OrchaError(
      `release v${v} has no binary for ${platformKey} (available: [${known.join(', ')}])`,
    );
  }

  await downloadTo(entry.url, target);
  const actual = await sha256(target);
  if (actual !== entry.sha256) {
    await fsp.unlink(target).catch(() => {});
    throw new OrchaError(
      `sha256 mismatch for ${name}: expected ${entry.sha256}, got ${actual}`,
    );
  }
  // chmod +x for owner/group/other; no-op on Windows.
  if (osName !== 'windows') {
    await fsp.chmod(target, 0o755);
  }
  return target;
}

module.exports = {
  resolveBinary,
  // Exported for tests / advanced callers.
  detectPlatform,
  binaryFilename,
  installDir,
  sha256,
};
