#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const https = require("node:https");
const { spawnSync, execFileSync } = require("node:child_process");

const PACKAGE_JSON = require("../package.json");

function platformInfo() {
  const p = process.platform;
  const a = process.arch;
  if (p === "darwin" && a === "x64") return { os: "darwin", arch: "amd64", ext: "tar.gz", exe: "" };
  if (p === "darwin" && a === "arm64") return { os: "darwin", arch: "arm64", ext: "tar.gz", exe: "" };
  if (p === "linux" && a === "x64") return { os: "linux", arch: "amd64", ext: "tar.gz", exe: "" };
  if (p === "linux" && a === "arm64") return { os: "linux", arch: "arm64", ext: "tar.gz", exe: "" };
  if (p === "win32" && a === "x64") return { os: "windows", arch: "amd64", ext: "zip", exe: ".exe" };
  throw new Error(`Unsupported platform/arch: ${p}/${a}`);
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const doRequest = (currentUrl) => {
      https
        .get(currentUrl, (res) => {
          if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
            res.resume();
            doRequest(res.headers.location);
            return;
          }
          if (res.statusCode !== 200) {
            reject(new Error(`Download failed (${res.statusCode}) for ${currentUrl}`));
            res.resume();
            return;
          }
          const out = fs.createWriteStream(dest);
          res.pipe(out);
          out.on("finish", () => out.close(resolve));
          out.on("error", reject);
        })
        .on("error", reject);
    };
    doRequest(url);
  });
}

function findFile(root, targetName) {
  const entries = fs.readdirSync(root, { withFileTypes: true });
  for (const entry of entries) {
    const full = path.join(root, entry.name);
    if (entry.isFile() && entry.name === targetName) {
      return full;
    }
    if (entry.isDirectory()) {
      const nested = findFile(full, targetName);
      if (nested) return nested;
    }
  }
  return "";
}

function extractArchive(archivePath, outDir, ext) {
  if (ext === "tar.gz") {
    execFileSync("tar", ["-xzf", archivePath, "-C", outDir], { stdio: "inherit" });
    return;
  }
  if (ext === "zip") {
    if (process.platform === "win32") {
      execFileSync(
        "powershell.exe",
        ["-NoProfile", "-NonInteractive", "-Command", `Expand-Archive -Path "${archivePath}" -DestinationPath "${outDir}" -Force`],
        { stdio: "inherit" }
      );
      return;
    }
    execFileSync("unzip", ["-o", archivePath, "-d", outDir], { stdio: "inherit" });
    return;
  }
  throw new Error(`Unsupported archive format: ${ext}`);
}

async function ensureBinary(binaryPath, binaryName, assetFile, versionTag, ext) {
  if (fs.existsSync(binaryPath)) {
    return;
  }

  fs.mkdirSync(path.dirname(binaryPath), { recursive: true });
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "yt-vod-manager-npm-"));
  const archivePath = path.join(tempDir, assetFile);
  const releaseUrl = `https://github.com/marcohefti/yt-vod-manager/releases/download/${versionTag}/${assetFile}`;

  try {
    await download(releaseUrl, archivePath);
    extractArchive(archivePath, tempDir, ext);

    const extractedBinary = findFile(tempDir, binaryName);
    if (!extractedBinary) {
      throw new Error(`Could not find extracted binary ${binaryName} in archive ${assetFile}`);
    }

    fs.copyFileSync(extractedBinary, binaryPath);
    if (process.platform !== "win32") {
      fs.chmodSync(binaryPath, 0o755);
    }
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
}

async function main() {
  const target = platformInfo();
  const versionTag = `v${PACKAGE_JSON.version}`;
  const binaryName = `yt-vod-manager${target.exe}`;
  const assetFile = `yt-vod-manager_${versionTag}_${target.os}_${target.arch}.${target.ext}`;
  const installDir = path.join(__dirname, "..", "vendor", `${target.os}-${target.arch}`);
  const binaryPath = path.join(installDir, binaryName);

  await ensureBinary(binaryPath, binaryName, assetFile, versionTag, target.ext);

  const result = spawnSync(binaryPath, process.argv.slice(2), { stdio: "inherit" });
  if (result.error) {
    throw result.error;
  }
  process.exit(result.status ?? 0);
}

main().catch((err) => {
  console.error(`yt-vod-manager npm launcher error: ${err.message}`);
  process.exit(1);
});
