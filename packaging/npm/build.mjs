#!/usr/bin/env node
/**
 * Builds the npm packages for a released version.
 *
 * Layout follows the esbuild/biome pattern: one entry package that declares an
 * optionalDependency on a package per platform, each containing nothing but the
 * binary and `os`/`cpu` fields. npm installs exactly the one that matches and
 * skips the rest.
 *
 * The obvious alternative — one package with a postinstall script that downloads
 * the right binary — was rejected on purpose. It needs network at install time
 * and silently produces a broken install under `npm ci --ignore-scripts`, which
 * plenty of CI and agent sandboxes set. Six small packages are the cost of an
 * install that cannot half-work.
 *
 * Usage: node packaging/npm/build.mjs <version> <artifacts-dir> [out-dir]
 *   <artifacts-dir> holds the goreleaser tarballs already extracted into
 *   <artifacts-dir>/<goos>_<goarch>/modelslab[.exe]
 */
import { mkdirSync, writeFileSync, copyFileSync, existsSync, chmodSync } from 'node:fs';
import { join, resolve } from 'node:path';

const NAME = 'modelslab-cli';
const BIN = 'modelslab';
const REPO = 'https://github.com/ModelsLab/modelslab-cli';
const DESCRIPTION =
    'ModelsLab CLI — AI generation and account management from the terminal';

/** goreleaser target -> npm os/cpu. Windows is `win32` to npm, whatever Go calls it. */
const TARGETS = [
    { go: 'darwin_amd64', os: 'darwin', cpu: 'x64' },
    { go: 'darwin_arm64', os: 'darwin', cpu: 'arm64' },
    { go: 'linux_amd64', os: 'linux', cpu: 'x64' },
    { go: 'linux_arm64', os: 'linux', cpu: 'arm64' },
    { go: 'windows_amd64', os: 'win32', cpu: 'x64' },
    { go: 'windows_arm64', os: 'win32', cpu: 'arm64' },
];

const [, , rawVersion, artifactsDir, outDirArg] = process.argv;

if (!rawVersion || !artifactsDir) {
    console.error('usage: build.mjs <version> <artifacts-dir> [out-dir]');
    process.exit(1);
}

// Tags are v-prefixed; npm versions are not.
const version = rawVersion.replace(/^v/, '');
const outDir = resolve(outDirArg ?? 'dist/npm');

if (!/^\d+\.\d+\.\d+(-[\w.]+)?$/.test(version)) {
    console.error(`refusing to build: "${version}" is not a semver version`);
    process.exit(1);
}

const common = {
    version,
    description: DESCRIPTION,
    homepage: 'https://modelslab.sh',
    repository: { type: 'git', url: `git+${REPO}.git` },
    bugs: { url: `${REPO}/issues` },
    license: 'MIT',
    author: 'ModelsLab <support@modelslab.com>',
};

const platformPackages = [];

for (const target of TARGETS) {
    const exe = target.os === 'win32' ? `${BIN}.exe` : BIN;
    const source = join(artifactsDir, target.go, exe);

    if (!existsSync(source)) {
        console.error(`missing binary for ${target.go}: ${source}`);
        process.exit(1);
    }

    const pkgName = `${NAME}-${target.os}-${target.cpu}`;
    const pkgDir = join(outDir, pkgName);
    mkdirSync(join(pkgDir, 'bin'), { recursive: true });

    const dest = join(pkgDir, 'bin', exe);
    copyFileSync(source, dest);
    // npm preserves the mode in the tarball; without this the shim cannot exec it.
    chmodSync(dest, 0o755);

    writeFileSync(
        join(pkgDir, 'package.json'),
        JSON.stringify(
            {
                name: pkgName,
                ...common,
                description: `${DESCRIPTION} (${target.os} ${target.cpu} binary)`,
                os: [target.os],
                cpu: [target.cpu],
                // Only the binary. No lifecycle scripts, nothing to execute at install.
                files: ['bin'],
                preferUnplugged: true,
            },
            null,
            2
        ) + '\n'
    );

    platformPackages.push(pkgName);
    console.log(`built ${pkgName}`);
}

// --- entry package -----------------------------------------------------------
const entryDir = join(outDir, NAME);
mkdirSync(join(entryDir, 'bin'), { recursive: true });

writeFileSync(
    join(entryDir, 'package.json'),
    JSON.stringify(
        {
            name: NAME,
            ...common,
            keywords: [
                'modelslab',
                'cli',
                'ai',
                'image-generation',
                'video-generation',
                'text-to-speech',
                'llm',
                'agent',
            ],
            bin: { [BIN]: 'bin/modelslab.cjs' },
            files: ['bin', 'README.md'],
            // Optional so an unsupported platform fails at run time with a
            // readable message instead of failing the whole install.
            optionalDependencies: Object.fromEntries(
                platformPackages.map((name) => [name, version])
            ),
            engines: { node: '>=16' },
        },
        null,
        2
    ) + '\n'
);

copyFileSync(
    resolve(import.meta.dirname, 'shim.cjs'),
    join(entryDir, 'bin', 'modelslab.cjs')
);
copyFileSync(
    resolve(import.meta.dirname, 'README.md'),
    join(entryDir, 'README.md')
);

console.log(`built ${NAME} (entry) with ${platformPackages.length} optional deps`);
console.log(`output: ${outDir}`);
