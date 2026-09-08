const { app, BrowserWindow, dialog, ipcMain, shell } = require('electron')
const fs = require('node:fs/promises')
const fsSync = require('node:fs')
const path = require('node:path')
const { randomBytes } = require('node:crypto')
const { spawn } = require('node:child_process')
const {
  appleScriptQuote,
  buildManagedServiceConfig,
  buildWindowsElevationLauncher,
  buildWindowsServiceScript,
  redactSecrets,
  rpcPort,
  shellQuote,
  validateProfile,
} = require('./service-utils.cjs')

const serviceName = 'frpp'
const macServiceDir = '/usr/local/libexec/frp-manager'
const macServiceBinary = `${macServiceDir}/frpp`
const macServiceConfigDir = '/etc/frpp'
const macServiceConfig = `${macServiceConfigDir}/.env`
const windowsServiceDir = path.join(process.env.ProgramFiles || 'C:\\Program Files', 'frp-manager')
const windowsServiceBinary = path.join(windowsServiceDir, 'frpp.exe')
const windowsServiceConfig = path.join(windowsServiceDir, '.env')

function dataDir() {
  return app.getPath('userData')
}

function profilesPath() {
  return path.join(dataDir(), 'profiles.json')
}

async function readProfiles() {
  try {
    const raw = await fs.readFile(profilesPath(), 'utf8')
    return JSON.parse(raw)
  } catch {
    return []
  }
}

async function writeProfiles(profiles) {
  await fs.mkdir(dataDir(), { recursive: true })
  await fs.writeFile(profilesPath(), JSON.stringify(profiles, null, 2), 'utf8')
  await fs.chmod(profilesPath(), 0o600)
  return profiles
}

function createWindow() {
  const win = new BrowserWindow({
    show: false,
    width: 1180,
    height: 760,
    minWidth: 920,
    minHeight: 620,
    title: 'frp-manager Client',
    backgroundColor: '#f6f7f9',
    icon: path.join(__dirname, '..', 'resources', 'icon.png'),
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
      webSecurity: true,
    },
  })

  const devUrl = process.env.VITE_DEV_SERVER_URL || 'http://127.0.0.1:5173'
  win.webContents.setWindowOpenHandler(() => ({ action: 'deny' }))
  win.webContents.on('will-navigate', (event, url) => {
    if (!url.startsWith('file://') && !url.startsWith(devUrl)) {
      event.preventDefault()
    }
  })

  if (!app.isPackaged) {
    win.loadURL(devUrl)
  } else {
    win.loadFile(path.join(__dirname, '..', 'dist', 'index.html'))
  }

  win.once('ready-to-show', () => win.show())
  win.webContents.on('did-fail-load', (_event, code, description, url) => {
    const message = `Unable to load the client interface (${code}): ${description}\n${url}`
    console.error(message)
    dialog.showErrorBox('frp-manager Client', message)
  })
}

app.whenReady().then(() => {
  registerIpc()
  createWindow()
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})

function registerIpc() {
  ipcMain.handle('profiles:list', readProfiles)

  ipcMain.handle('profiles:save', async (_event, profile) => {
    const profiles = await readProfiles()
    const next = [profile, ...profiles.filter((item) => item.id !== profile.id)]
    return writeProfiles(next)
  })

  ipcMain.handle('profiles:delete', async (_event, id) => {
    const profiles = await readProfiles()
    return writeProfiles(profiles.filter((profile) => profile.id !== id))
  })

  ipcMain.handle('runtime:status', async () => runtimeStatus())

  ipcMain.handle('runtime:chooseBinary', async () => {
    const result = await dialog.showOpenDialog({
      title: 'Choose frp-manager client binary',
      properties: ['openFile'],
      filters: [{ name: 'frp-manager client', extensions: process.platform === 'win32' ? ['exe'] : ['*'] }],
    })
    return result.canceled ? null : result.filePaths[0]
  })

  ipcMain.handle('runtime:openDataDir', async () => {
    await fs.mkdir(dataDir(), { recursive: true })
    await shell.openPath(dataDir())
  })

  ipcMain.handle('runtime:action', async (_event, action, profile) => executeAction(action, profile))
}

async function runtimeStatus(profile = {}) {
  const binaryPath = resolveBinary(profile)
  return {
    platform: process.platform,
    serviceStatus: await serviceStatus(),
    binaryPath,
    dataDir: dataDir(),
  }
}

async function executeAction(action, profile) {
  try {
    if (process.platform === 'win32' && ['apply', 'install', 'restart', 'start', 'stop', 'uninstall'].includes(action)) {
      const output = await runWindowsServiceAction(action, profile)
      const messages = {
        apply: `Applied and started ${profile.name}.`,
        install: `Installed ${profile.name}.`,
        restart: `Restarted ${profile.name}.`,
        start: `Started ${profile.name}.`,
        stop: `Stopped ${profile.name}.`,
        uninstall: `Uninstalled ${profile.name}.`,
      }
      return ok(messages[action], output, profile)
    }
    if (process.platform === 'darwin' && ['apply', 'install', 'restart', 'start', 'stop', 'uninstall'].includes(action)) {
      const output = await runMacServiceAction(action, profile)
      const messages = {
        apply: `Applied and started ${profile.name}.`,
        install: `Installed ${profile.name}.`,
        restart: `Restarted ${profile.name}.`,
        start: `Started ${profile.name}.`,
        stop: `Stopped ${profile.name}.`,
        uninstall: `Uninstalled ${profile.name}.`,
      }
      return ok(messages[action], output, profile)
    }
    if (action === 'apply') {
      await runFrpp(['stop'], profile, { allowFailure: true })
      await runFrpp(['uninstall'], profile, { allowFailure: true })
      const installed = await runFrpp(installArgs(profile), profile)
      const started = await runFrpp(['start'], profile)
      return ok(`Applied and started ${profile.name}.`, `${installed}\n${started}`, profile)
    }
    if (action === 'install') {
      const output = await runFrpp(installArgs(profile), profile)
      return ok(`Installed ${profile.name}.`, output, profile)
    }
    if (action === 'restart') {
      const stopped = await runFrpp(['stop'], profile, { allowFailure: true })
      const started = await runFrpp(['start'], profile)
      return ok(`Restarted ${profile.name}.`, `${stopped}\n${started}`, profile)
    }
    if (action === 'run') {
      const output = await runFrpp(clientArgs(profile), profile)
      return ok(`Client exited for ${profile.name}.`, output, profile)
    }
    if (['start', 'stop', 'uninstall'].includes(action)) {
      const output = await runFrpp([action], profile)
      return ok(`${action} completed for ${profile.name}.`, output, profile)
    }
    throw new Error(`Unsupported action: ${action}`)
  } catch (error) {
    const safeMessage = redactSecrets(error.message, [profile?.secret, profile?.joinToken])
    const safeOutput = redactSecrets(error.output || safeMessage, [profile?.secret, profile?.joinToken])
    return {
      ok: false,
      message: safeMessage,
      output: safeOutput,
      status: await runtimeStatus(profile),
    }
  }
}

async function ok(message, output, profile) {
  const safeOutput = redactSecrets(output, [profile?.secret, profile?.joinToken])
  return {
    ok: true,
    message,
    output: safeOutput.trim() || message,
    status: await runtimeStatus(profile),
  }
}

function clientArgs(profile) {
  assertProfile(profile)
  return ['client', ...(profile.joinToken ? ['--join-token', profile.joinToken] : ['-s', profile.secret, '-i', profile.clientId]), '--api-url', profile.apiUrl, '--rpc-url', profile.rpcUrl]
}

function installArgs(profile) {
  return ['install', ...clientArgs(profile)]
}

function assertProfile(profile) {
  validateProfile(profile)
}

async function runWindowsServiceAction(action, profile) {
  const sourceBinary = resolveBinary(profile)
  const needsSourceBinary = action === 'apply' || action === 'install'
  if (needsSourceBinary && (!sourceBinary || !fsSync.existsSync(sourceBinary))) {
    throw new Error(`frp-manager client binary not found: ${sourceBinary || 'not configured'}`)
  }

  let tempDir
  try {
    tempDir = await fs.mkdtemp(path.join(dataDir(), 'service-config-'))
    const tempConfig = path.join(tempDir, '.env')
    const outputPath = path.join(tempDir, 'service-output.txt')
    await fs.writeFile(outputPath, '', { encoding: 'utf8', mode: 0o600 })
    if (needsSourceBinary) {
      assertProfile(profile)
      const config = buildManagedServiceConfig(profile, randomBytes(48).toString('base64url'))
      await fs.writeFile(tempConfig, config, { encoding: 'utf8', mode: 0o600 })
    }

    const helper = buildWindowsServiceScript({
      action,
      sourceBinary,
      tempConfig,
      installDir: windowsServiceDir,
      targetBinary: windowsServiceBinary,
      targetConfig: windowsServiceConfig,
      outputPath,
      managerRpcPort: rpcPort(profile?.rpcUrl),
    })
    const launcher = buildWindowsElevationLauncher(helper, outputPath)
    return await runProcess('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', launcher])
  } catch (error) {
    const errorMessage = String(error.message).toLowerCase()
    if (
      error.exitCode === 1223 ||
      errorMessage.includes('canceled by the user') ||
      errorMessage.includes('operation was canceled')
    ) {
      throw new Error('Administrator authorization was canceled.')
    }
    throw error
  } finally {
    if (tempDir) await fs.rm(tempDir, { recursive: true, force: true })
  }
}

async function runMacServiceAction(action, profile) {
  const sourceBinary = resolveBinary(profile)
  const needsSourceBinary = action === 'apply' || action === 'install'
  if (needsSourceBinary && (!sourceBinary || !fsSync.existsSync(sourceBinary))) {
    throw new Error(`frp-manager client binary not found: ${sourceBinary || 'not configured'}`)
  }

  let tempDir
  try {
    const commands = []
    if (action === 'apply' || action === 'install') {
      assertProfile(profile)
      tempDir = await fs.mkdtemp(path.join(dataDir(), 'service-config-'))
      const tempConfig = path.join(tempDir, '.env')
      const config = buildManagedServiceConfig(profile, randomBytes(48).toString('base64url'))
      await fs.writeFile(tempConfig, config, { encoding: 'utf8', mode: 0o600 })

      if (action === 'apply') {
        const controlBinary = fsSync.existsSync(macServiceBinary) ? macServiceBinary : sourceBinary
        commands.push(
          `${processCommand(controlBinary, ['stop'])} >/dev/null 2>&1 || true`,
          `${processCommand(controlBinary, ['uninstall'])} >/dev/null 2>&1 || true`,
        )
      }
      commands.push(
        processCommand('/usr/bin/install', ['-d', '-m', '0755', macServiceDir]),
        processCommand('/usr/bin/install', ['-d', '-m', '0700', macServiceConfigDir]),
        processCommand('/usr/bin/install', ['-m', '0755', sourceBinary, macServiceBinary]),
        processCommand('/usr/bin/install', ['-m', '0600', tempConfig, macServiceConfig]),
        processCommand(macServiceBinary, ['install', 'client']),
      )
      if (action === 'apply') commands.push(processCommand(macServiceBinary, ['start']))
    } else {
      commands.push(`[ -x ${shellQuote(macServiceBinary)} ] || { echo 'frp-manager service is not installed' >&2; exit 1; }`)
      commands.push(processCommand(macServiceBinary, [action]))
      if (action === 'uninstall') {
        commands.push(
          processCommand('/bin/rm', ['-f', macServiceConfig]),
          processCommand('/bin/rm', ['-f', macServiceBinary]),
        )
      }
    }

    return await runMacPrivileged(commands)
  } finally {
    if (tempDir) await fs.rm(tempDir, { recursive: true, force: true })
  }
}

function processCommand(command, args) {
  return [command, ...args].map(shellQuote).join(' ')
}

async function runMacPrivileged(commands) {
  const shellCommand = ['set -e', ...commands].join('; ')
  const script = `do shell script ${appleScriptQuote(shellCommand)} with administrator privileges`
  try {
    return await runProcess('/usr/bin/osascript', ['-e', script])
  } catch (error) {
    if (String(error.message).includes('(-128)') || String(error.message).toLowerCase().includes('user canceled')) {
      throw new Error('Administrator authorization was canceled.')
    }
    throw error
  }
}

async function runFrpp(args, profile, options = {}) {
  const binary = resolveBinary(profile)
  if (!binary || !fsSync.existsSync(binary)) {
    throw new Error(`frp-manager client binary not found: ${binary || 'not configured'}`)
  }

  return new Promise((resolve, reject) => {
    const child = spawn(binary, args, {
      cwd: path.dirname(binary),
      windowsHide: true,
    })
    let output = ''
    child.stdout.on('data', (data) => {
      output += data.toString()
    })
    child.stderr.on('data', (data) => {
      output += data.toString()
    })
    child.on('error', reject)
    child.on('close', (code) => {
      if (code !== 0 && !options.allowFailure) {
        const error = new Error(`frp-manager exited with code ${code}.`)
        error.output = redactSecrets(output, [profile?.secret, profile?.joinToken])
        reject(error)
        return
      }
      resolve(redactSecrets(output, [profile?.secret, profile?.joinToken]))
    })
  })
}

async function serviceStatus() {
  if (process.platform === 'darwin') {
    try {
      const output = await runProcess('/bin/launchctl', ['print', `system/${serviceName}`])
      const match = output.match(/\bstate\s*=\s*(\w+)/)
      return match ? match[1].toLowerCase() : 'installed'
    } catch {
      return fsSync.existsSync(`/Library/LaunchDaemons/${serviceName}.plist`) ? 'stopped' : 'not installed'
    }
  }
  if (process.platform !== 'win32') return 'manual'
  try {
    const output = await runProcess('sc.exe', ['query', serviceName])
    const match = output.match(/STATE\s*:\s*\d+\s+(\w+)/)
    return match ? match[1].toLowerCase() : 'installed'
  } catch {
    return 'not installed'
  }
}

function runProcess(command, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { windowsHide: true })
    let output = ''
    child.stdout.on('data', (data) => {
      output += data.toString()
    })
    child.stderr.on('data', (data) => {
      output += data.toString()
    })
    child.on('error', reject)
    child.on('close', (code) => {
      if (code === 0) resolve(output)
      else {
        const error = new Error(output || `${command} exited with ${code}`)
        error.output = output
        error.exitCode = code
        reject(error)
      }
    })
  })
}

function resolveBinary(profile = {}) {
  if (profile.binaryPath) return profile.binaryPath
  if (process.env.FRP_MANAGER_CLIENT_BIN) return process.env.FRP_MANAGER_CLIENT_BIN

  const exeName = process.platform === 'win32' ? 'frpp.exe' : 'frpp'
  const candidates = [
    path.join(process.resourcesPath || '', exeName),
    path.join(app.getAppPath(), 'resources', 'binaries', exeName),
    process.platform === 'win32' ? path.join(process.env.ProgramFiles || 'C:\\Program Files', 'frp-manager', 'frpp.exe') : '',
    path.resolve(app.getAppPath(), '..', 'dist', desktopAssetName()),
  ].filter(Boolean)

  return candidates.find((candidate) => fsSync.existsSync(candidate)) || candidates[0]
}

function desktopAssetName() {
  const arch = process.arch === 'x64' ? 'amd64' : process.arch
  if (process.platform === 'win32') return `frp-manager-client-windows-${arch}.exe`
  if (process.platform === 'darwin') return `frp-manager-client-darwin-${arch}`
  return `frp-manager-client-linux-${arch}`
}
