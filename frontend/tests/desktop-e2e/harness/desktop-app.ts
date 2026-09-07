// desktop-app.ts is the process harness `make desktop-test-ui` drives: it
// resolves which packaged executable to run, launches it in complete
// isolation from any real xchats install on this machine (its own temp
// config/data directories, its own loopback port, file-backed credentials
// so it never needs a real OS keyring), waits for
// XCHATS_DESKTOP_E2E_HTTP=1's readiness endpoint, and tears down cleanly —
// killing only the process it started, never anything found by port or
// name.
//
// This never runs in CI and is never invoked from another Make target, an
// npm lifecycle hook, or implicitly by anything else — see
// docs/desktop.md's "Local Executable UI Testing" section and the Makefile
// target's own comment. It exists to test the real, packaged Wails
// executable end to end, something no unit test can do.
import { type ChildProcess, spawn } from 'node:child_process'
import { mkdtempSync, mkdirSync, copyFileSync, chmodSync, openSync, closeSync } from 'node:fs'
import net from 'node:net'
import os from 'node:os'
import path from 'node:path'

export interface DesktopAppOptions {
  /** Directory app.log/console output is written to for post-mortem debugging. */
  logDir: string
}

export interface DesktopApp {
  /** Base URL of the running instance's loopback listener (E2E HTTP mode). */
  baseURL: string
  /** Absolute path to the isolated data root this instance was started with. */
  dataDir: string
  /** Absolute path to the isolated config directory this instance was started with. */
  configDir: string
  /** Waits for both the backend and the Wails window to report ready. Throws on timeout. */
  waitForReady(timeoutMs?: number): Promise<void>
  /** Stops the process (SIGTERM, then SIGKILL after a grace period) and waits for exit. */
  stop(): Promise<void>
  /**
   * Stops and relaunches the SAME executable against the SAME data/config
   * directories (a fresh loopback port is chosen) — for the "does data
   * survive a restart" half of the scenario. Resolves once the new
   * instance's readiness endpoint reports ready.
   */
  restart(): Promise<void>
}

const READY_PATH = '/__xchats_e2e/ready'

/** Resolves the executable Playwright should drive: DESKTOP_E2E_BINARY, or this platform's `wails build` output. */
export function resolveBinaryPath(repoRoot: string): string {
  const explicit = process.env.DESKTOP_E2E_BINARY
  if (explicit && explicit.trim() !== '') return path.resolve(explicit)

  const buildBin = path.join(repoRoot, 'backend', 'cmd', 'xchats', 'build', 'bin')
  switch (process.platform) {
    case 'win32':
      return path.join(buildBin, 'xchats.exe')
    case 'darwin':
      // Wails packages a macOS build as an .app bundle; the actual
      // executable Node can spawn directly lives inside it.
      return path.join(buildBin, 'xchats.app', 'Contents', 'MacOS', 'xchats')
    default:
      return path.join(buildBin, 'xchats')
  }
}

/** Binds port 0, reads back the OS-assigned port, and releases it — the same probe-then-release tradeoff internal/desktop/desktop.go's freeAddr accepts. */
async function getFreePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer()
    srv.unref()
    srv.on('error', reject)
    srv.listen(0, '127.0.0.1', () => {
      const addr = srv.address()
      if (addr === null || typeof addr === 'string') {
        reject(new Error('getFreePort: unexpected listener address'))
        return
      }
      const { port } = addr
      srv.close(() => resolve(port))
    })
  })
}

async function pollReady(baseURL: string, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs
  let lastError = ''
  while (Date.now() < deadline) {
    try {
      const res = await fetch(baseURL + READY_PATH)
      if (res.ok) return
      lastError = `HTTP ${res.status}`
    } catch (err) {
      lastError = err instanceof Error ? err.message : String(err)
    }
    await new Promise((r) => setTimeout(r, 300))
  }
  throw new Error(`timed out waiting for ${baseURL}${READY_PATH} to report ready (last: ${lastError})`)
}

/**
 * Copies the resolved binary into a FRESH temp directory whose name
 * deliberately contains a space — the executable must run correctly from a
 * path a naive shell-quoting bug would mishandle. Node's child_process.spawn
 * with an argv array never goes through a shell, so this exercises the
 * app's own argv/self-location handling rather than the harness's.
 */
function stageBinaryUnderSpacedPath(binaryPath: string): string {
  const stageDir = mkdtempSync(path.join(os.tmpdir(), 'xchats e2e '))
  const staged = path.join(stageDir, path.basename(binaryPath))
  copyFileSync(binaryPath, staged)
  chmodSync(staged, 0o755)
  return staged
}

class RunningInstance {
  constructor(
    public child: ChildProcess,
    public baseURL: string
  ) {}

  async stop(): Promise<void> {
    if (this.child.exitCode !== null || this.child.signalCode !== null) return
    await new Promise<void>((resolve) => {
      const child = this.child
      const onExit = () => resolve()
      child.once('exit', onExit)
      // Negative pid (POSIX only) signals the whole process group, so a
      // headless-Linux run wrapped in xvfb-run also takes down the actual
      // xchats process it spawned, not just the wrapper shell.
      const target = process.platform === 'win32' ? child.pid : -(child.pid ?? 0)
      try {
        if (target) process.kill(target, 'SIGTERM')
      } catch {
        // ESRCH: already exited between the exitCode check above and here.
        resolve()
        return
      }
      const killTimer = setTimeout(() => {
        try {
          if (target) process.kill(target, 'SIGKILL')
        } catch {
          /* already gone */
        }
      }, 5_000)
      child.once('exit', () => clearTimeout(killTimer))
    })
  }
}

export async function launchDesktopApp(repoRoot: string, options: DesktopAppOptions): Promise<DesktopApp> {
  const binaryPath = resolveBinaryPath(repoRoot)
  const stagedBinary = stageBinaryUnderSpacedPath(binaryPath)

  const dataDir = mkdtempSync(path.join(os.tmpdir(), 'xchats-e2e-data-'))
  const configDir = mkdtempSync(path.join(os.tmpdir(), 'xchats-e2e-config-'))
  mkdirSync(options.logDir, { recursive: true })

  let instance: RunningInstance
  let baseURL: string

  async function spawnInstance(): Promise<RunningInstance> {
    const port = await getFreePort()
    const addr = `127.0.0.1:${port}`
    const url = `http://${addr}`

    const useXvfb = process.platform === 'linux' && !process.env.DISPLAY
    const command = useXvfb ? 'xvfb-run' : stagedBinary
    const args = useXvfb ? ['-a', stagedBinary, '--data-dir', dataDir] : ['--data-dir', dataDir]

    const logPath = path.join(options.logDir, `app-${Date.now()}.log`)
    const logFd = openSync(logPath, 'a')

    const child = spawn(command, args, {
      cwd: path.dirname(stagedBinary),
      env: {
        ...process.env,
        XCHATS_CONFIG_DIR: configDir,
        XCHATS_ALLOW_FILE_CREDENTIALS: '1',
        XCHATS_DESKTOP_E2E_HTTP: '1',
        HTTP_ADDR: addr,
      },
      stdio: ['ignore', logFd, logFd],
      // detached (POSIX only — harmless no-op flag on Windows) puts the
      // child in its own process group, so RunningInstance.stop can signal
      // xvfb-run AND the real xchats process it launched together, and a
      // Ctrl-C delivered to this Node process's own group does not also
      // reach (and prematurely kill) the app mid-test.
      detached: process.platform !== 'win32',
    })
    closeSync(logFd)

    child.once('exit', (code, signal) => {
      if (code !== 0 && code !== null) {
        console.error(`desktop-app: process exited early (code=${code} signal=${signal}); see ${logPath}`)
      }
    })
    child.once('error', (err) => {
      console.error(
        `desktop-app: failed to launch ${command}: ${err.message}` +
          (useXvfb ? ' — is xvfb-run installed? (apt-get install xvfb)' : ` — does the binary exist at ${stagedBinary}? Set DESKTOP_E2E_BINARY or run \`make desktop-build\` first.`)
      )
    })

    return new RunningInstance(child, url)
  }

  instance = await spawnInstance()
  baseURL = instance.baseURL

  return {
    get baseURL() {
      return baseURL
    },
    dataDir,
    configDir,
    async waitForReady(timeoutMs = 60_000) {
      await pollReady(baseURL, timeoutMs)
    },
    async stop() {
      await instance.stop()
    },
    async restart() {
      await instance.stop()
      instance = await spawnInstance()
      baseURL = instance.baseURL
      await pollReady(baseURL, 60_000)
    },
  }
}
