#!/usr/bin/env node
/**
 * Locates the platform binary and hands the process over to it.
 *
 * `spawnSync` with `stdio: 'inherit'` rather than an exec: Node has no execv, and
 * inheriting the streams keeps interactive prompts, pipes and TTY detection
 * behaving as if the binary were invoked directly. The child's exit code and
 * terminating signal are both propagated — a wrapper that swallowed a non-zero
 * exit would break every script that checks it.
 */
const { spawnSync } = require('node:child_process');
const { existsSync, realpathSync } = require('node:fs');
const { join, dirname, sep } = require('node:path');

const PLATFORM_PACKAGES = {
    'darwin x64': 'modelslab-cli-darwin-x64',
    'darwin arm64': 'modelslab-cli-darwin-arm64',
    'linux x64': 'modelslab-cli-linux-x64',
    'linux arm64': 'modelslab-cli-linux-arm64',
    'win32 x64': 'modelslab-cli-win32-x64',
    'win32 arm64': 'modelslab-cli-win32-arm64',
};

function resolveBinary() {
    const key = `${process.platform} ${process.arch}`;
    const pkg = PLATFORM_PACKAGES[key];

    if (!pkg) {
        throw new Error(
            `ModelsLab CLI does not ship a binary for ${key}.\n` +
                `Supported: ${Object.keys(PLATFORM_PACKAGES).join(', ')}.\n` +
                `Build from source: https://github.com/ModelsLab/modelslab-cli`
        );
    }

    const exe = process.platform === 'win32' ? 'modelslab.exe' : 'modelslab';
    const subpath = `${pkg}/bin/${exe}`;

    /*
     * Three lookups, because one is not enough in practice.
     *
     * The plain resolve covers a normal `npm i -g`. It fails whenever this
     * package is reached through a symlink — `npm install <path>`, `npm link`,
     * and every pnpm install — because module resolution then starts from the
     * link target, which has no node_modules of its own. Anchoring the search at
     * the real directory and at the caller's cwd covers those.
     */
    const anchors = [__dirname];
    try {
        anchors.push(realpathSync(__dirname));
    } catch {
        // realpath can fail on a broken link; the other anchors still apply.
    }
    anchors.push(process.cwd());

    for (const resolver of [
        () => require.resolve(subpath),
        () => require.resolve(subpath, { paths: anchors }),
    ]) {
        try {
            return resolver();
        } catch {
            // try the next strategy
        }
    }

    /*
     * Last resort: walk up from here looking for a sibling inside any
     * node_modules directory. Catches layouts where the package is present but
     * unreachable by Node's algorithm from where this file physically lives.
     */
    for (const anchor of anchors) {
        let dir = anchor;
        while (true) {
            const candidate = join(dir, 'node_modules', pkg, 'bin', exe);
            if (existsSync(candidate)) {
                return candidate;
            }
            const parent = dirname(dir);
            if (parent === dir || !parent.includes(sep)) {
                break;
            }
            dir = parent;
        }
    }

    {
        /*
         * The optional dependency is missing. Almost always one of:
         * `--no-optional`, an npm version that skipped it, or an install that
         * partially failed. Say which package so the fix is one command.
         */
        throw new Error(
            `ModelsLab CLI is installed but the binary for ${key} is not.\n` +
                `Expected the optional dependency "${pkg}".\n\n` +
                `Fix it with:  npm install ${pkg}\n` +
                `Or reinstall: npm install -g modelslab-cli\n\n` +
                `If you installed with --no-optional or --ignore-optional, that is why.`
        );
    }
}

let binary;
try {
    binary = resolveBinary();
} catch (error) {
    process.stderr.write(`${error.message}\n`);
    process.exit(1);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: 'inherit' });

if (result.error) {
    process.stderr.write(`failed to run ModelsLab CLI: ${result.error.message}\n`);
    process.exit(1);
}

// A signalled child has a null status; re-raise so the parent shell sees the
// same thing it would have seen running the binary directly.
if (result.signal) {
    process.kill(process.pid, result.signal);
}

process.exit(result.status ?? 1);
