import fs from 'node:fs';
import http from 'node:http';
import net from 'node:net';
import path from 'node:path';
import { spawn } from 'node:child_process';

const rootDir = process.cwd();
const frontendDir = path.join(rootDir, 'frontend');
const backendDir = path.join(rootDir, 'backend');
const isWindows = process.platform === 'win32';
const npmCliPath = path.join(path.dirname(process.execPath), 'node_modules', 'npm', 'bin', 'npm-cli.js');
const npmCommand = isWindows && fs.existsSync(npmCliPath) ? process.execPath : 'npm';
const npmBaseArgs = isWindows && fs.existsSync(npmCliPath) ? [npmCliPath] : [];
const uvCommand = isWindows ? 'uv.exe' : 'uv';
const requestedFrontendUrl = (() => {
    try {
        return new URL(process.env.IMAGEFLOW_FRONTEND_URL || 'http://127.0.0.1:5173');
    } catch {
        return new URL('http://127.0.0.1:5173');
    }
})();
const frontendHost = requestedFrontendUrl.hostname || '127.0.0.1';
const frontendProtocol = requestedFrontendUrl.protocol || 'http:';
const frontendPortStart = Number(requestedFrontendUrl.port || '5173');

let frontendProcess = null;
let backendProcess = null;
let backendRestartTimer = null;
let backendRestartPromise = Promise.resolve();
let stopping = false;
const backendFileMtimestamps = new Map();
let frontendPort = frontendPortStart;
let frontendUrl = `${frontendProtocol}//${frontendHost}:${frontendPort}`;

function spawnProcess(command, args, options = {}) {
    return spawn(command, args, {
        cwd: rootDir,
        stdio: 'inherit',
        shell: false,
        ...options,
        env: {
            ...process.env,
            ...options.env,
        },
    });
}

function waitForUrl(url, timeoutMs = 30000) {
    const deadline = Date.now() + timeoutMs;

    return new Promise((resolve, reject) => {
        const probe = () => {
            const request = http.get(url, (response) => {
                response.resume();
                resolve();
            });

            request.on('error', () => {
                if (Date.now() >= deadline) {
                    reject(new Error(`Timed out waiting for frontend dev server: ${url}`));
                    return;
                }
                setTimeout(probe, 300);
            });

            request.setTimeout(1000, () => {
                request.destroy();
            });
        };

        probe();
    });
}

function canListenOnPort(port, host) {
    return new Promise((resolve) => {
        const server = net.createServer();

        server.once('error', () => {
            resolve(false);
        });

        server.once('listening', () => {
            server.close(() => resolve(true));
        });

        server.listen(port, host);
    });
}

async function findAvailablePort(startPort, host, attempts = 20) {
    for (let offset = 0; offset < attempts; offset += 1) {
        const port = startPort + offset;
        if (await canListenOnPort(port, host)) {
            return port;
        }
    }
    throw new Error(`No available port found from ${host}:${startPort}`);
}

function startFrontend() {
    frontendProcess = spawnProcess(
        npmCommand,
        [...npmBaseArgs, 'run', 'dev', '--', '--host', frontendHost, '--port', String(frontendPort), '--strictPort'],
        { cwd: frontendDir },
    );

    frontendProcess.on('exit', (code, signal) => {
        frontendProcess = null;
        if (!stopping) {
            stopAll(1);
        }
    });
}

function startBackend() {
    backendProcess = spawnProcess(
        uvCommand,
        ['run', 'python', '-m', 'backend.main'],
        {
            env: {
                IMAGEFLOW_FRONTEND_URL: frontendUrl,
            },
        },
    );

    backendProcess.on('exit', (code, signal) => {
        backendProcess = null;
    });
}

function isProcessRunning(child) {
    return Boolean(child && child.exitCode === null && child.signalCode === null);
}

function waitForProcessExit(child) {
    return new Promise((resolve) => {
        if (!child || !isProcessRunning(child)) {
            resolve();
            return;
        }

        child.once('exit', () => resolve());
    });
}

async function stopProcess(child, options = {}) {
    if (!child || !isProcessRunning(child)) {
        return;
    }

    const { forceAfterMs = 3000 } = options;
    const exitPromise = waitForProcessExit(child);

    try {
        child.kill();
    } catch {
        // Fall back to forced termination below.
    }

    const forceTimer = setTimeout(() => {
        if (!isProcessRunning(child)) {
            return;
        }
        if (isWindows && child.pid) {
            spawn('taskkill', ['/pid', String(child.pid), '/t', '/f'], {
                stdio: 'ignore',
                shell: false,
            });
            return;
        }
        try {
            child.kill('SIGKILL');
        } catch {
            // Ignore repeated termination failures during shutdown.
        }
    }, forceAfterMs);

    await exitPromise;
    clearTimeout(forceTimer);
}

function shouldRestartForBackendFile(relativePath) {
    const normalized = String(relativePath || '').replaceAll('\\', '/');
    if (!normalized.endsWith('.py')) {
        return false;
    }
    if (
        normalized.includes('__pycache__/') ||
        normalized.startsWith('tests/') ||
        normalized.startsWith('engines/')
    ) {
        return false;
    }
    return true;
}

function snapshotBackendFiles(dirPath) {
    if (!fs.existsSync(dirPath)) {
        return;
    }

    for (const entry of fs.readdirSync(dirPath, { withFileTypes: true })) {
        const absolutePath = path.join(dirPath, entry.name);
        if (entry.isDirectory()) {
            if (entry.name === '__pycache__') {
                continue;
            }
            snapshotBackendFiles(absolutePath);
            continue;
        }

        const relativePath = path.relative(backendDir, absolutePath);
        if (!entry.isFile() || !shouldRestartForBackendFile(relativePath)) {
            continue;
        }

        const stat = fs.statSync(absolutePath);
        backendFileMtimestamps.set(absolutePath, stat.mtimeMs);
    }
}

function restartBackend(reason) {
    if (stopping) {
        return;
    }

    if (backendRestartTimer) {
        clearTimeout(backendRestartTimer);
    }

    backendRestartTimer = setTimeout(() => {
        backendRestartTimer = null;
        backendRestartPromise = backendRestartPromise.then(async () => {
            await stopProcess(backendProcess, { forceAfterMs: 2000 });
            if (!stopping) {
                startBackend();
            }
        });
    }, 250);
}

function watchBackend() {
    if (!fs.existsSync(backendDir)) {
        return;
    }

    backendFileMtimestamps.clear();
    snapshotBackendFiles(backendDir);

    fs.watch(backendDir, { recursive: true }, (_eventType, fileName) => {
        if (!fileName) {
            return;
        }

        const normalized = String(fileName).replaceAll('\\', '/');
        if (!shouldRestartForBackendFile(normalized)) {
            return;
        }

        const absolutePath = path.join(backendDir, normalized);
        if (!fs.existsSync(absolutePath)) {
            return;
        }

        let stat;
        try {
            stat = fs.statSync(absolutePath);
        } catch {
            return;
        }

        if (!stat.isFile()) {
            return;
        }

        const previousMtime = backendFileMtimestamps.get(absolutePath);
        if (previousMtime === stat.mtimeMs) {
            return;
        }
        backendFileMtimestamps.set(absolutePath, stat.mtimeMs);

        restartBackend(normalized);
    });
}

function stopAll(exitCode = 0) {
    if (stopping) {
        return;
    }

    stopping = true;
    if (backendRestartTimer) {
        clearTimeout(backendRestartTimer);
    }
    backendRestartPromise = backendRestartPromise.finally(async () => {
        await Promise.all([
            stopProcess(backendProcess, { forceAfterMs: 1500 }),
            stopProcess(frontendProcess, { forceAfterMs: 1500 }),
        ]);
        process.exit(exitCode);
    });
}

process.on('SIGINT', () => stopAll(0));
process.on('SIGTERM', () => stopAll(0));
process.on('uncaughtException', (error) => {
    console.error(error);
    stopAll(1);
});

frontendPort = await findAvailablePort(frontendPortStart, frontendHost);
frontendUrl = `${frontendProtocol}//${frontendHost}:${frontendPort}`;
startFrontend();
await waitForUrl(frontendUrl);
watchBackend();
startBackend();
