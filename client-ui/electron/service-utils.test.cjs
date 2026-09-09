const assert = require('node:assert/strict')
const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const test = require('node:test')

const {
  appleScriptQuote,
  buildManagedServiceConfig,
  buildWindowsElevationLauncher,
  buildWindowsServiceScript,
  dotenvValue,
  hasInsecureRemoteEndpoint,
  powershellQuote,
  redactSecrets,
  rpcPort,
  shellQuote,
  validateProfile,
} = require('./service-utils.cjs')

const secureProfile = {
  clientId: 'client-1',
  secret: 'top secret',
  apiUrl: 'https://manager.example.com',
  rpcUrl: 'grpc://manager.example.com:9001',
}

test('quotes shell and AppleScript input without interpolation', () => {
  assert.equal(shellQuote("a'b"), "'a'\\''b'")
  assert.equal(appleScriptQuote('a"b\\c'), '"a\\"b\\\\c"')
  assert.equal(dotenvValue('a"\n\\b'), '"a\\"\\n\\\\b"')
  assert.equal(powershellQuote("a'b"), "'a''b'")
})

test('builds a self-contained Windows service helper', () => {
  const helper = buildWindowsServiceScript({
    action: 'start',
    installDir: "C:\\Program Files\\frp-manager's",
    targetBinary: "C:\\Program Files\\frp-manager's\\frpp.exe",
    targetConfig: "C:\\Program Files\\frp-manager's\\.env",
    outputPath: "C:\\Temp\\service output's.txt",
    verifyService: false,
    managerRpcPort: 9001,
  })
  assert.doesNotMatch(helper, /\$env:FRP_MANAGER_/)
  assert.match(helper, /\$action = 'start'/)
  assert.match(helper, /frp-manager''s/)
  assert.match(helper, /\$exitCode = \[int\]\$process\.ExitCode/)
  assert.match(helper, /\$verifyService = \$false/)
  assert.match(helper, /stopped shortly after startup/)
  assert.match(helper, /did not register with the manager within 20 seconds/)
  assert.match(helper, /Windows service log/)
  assert.match(helper, /\$managerRpcPort = 9001/)
  assert.match(helper, /FileShare\]::ReadWrite/)
  assert.match(helper, /Get-NetTCPConnection/)
  assert.throws(
    () =>
      buildWindowsServiceScript({
        action: 'invalid',
        installDir: 'C:\\frp-manager',
        targetBinary: 'C:\\frp-manager\\frpp.exe',
        targetConfig: 'C:\\frp-manager\\.env',
        outputPath: 'C:\\Temp\\output.txt',
      }),
    /Unsupported Windows service action/,
  )
})

test('enables core startup updates for GUI-managed services', () => {
  const config = buildManagedServiceConfig(secureProfile, 'global-secret')
  assert.match(config, /^APP_GLOBAL_SECRET="global-secret"$/m)
  assert.match(config, /^APP_AUTO_UPDATE=true$/m)
  assert.doesNotMatch(config, /^LOGGER_FILE=/m)
  assert.match(config, /^CLIENT_ID="client-1"$/m)
  assert.match(config, /^CLIENT_SECRET="top secret"$/m)
})

test('runs the Windows helper and reports the real core exit code', { skip: process.platform !== 'win32' }, (t) => {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'frp-manager-powershell-'))
  t.after(() => fs.rmSync(tempDir, { recursive: true, force: true }))

  const fakeCore = path.join(tempDir, 'fake-core.cmd')
  const successOutput = path.join(tempDir, 'success.txt')
  fs.writeFileSync(fakeCore, '@echo off\r\necho command=%*\r\necho normal info log 1>&2\r\nexit /b 0\r\n')
  fs.writeFileSync(successOutput, '')

  const successHelper = buildWindowsServiceScript({
    action: 'start',
    installDir: tempDir,
    targetBinary: fakeCore,
    targetConfig: path.join(tempDir, '.env'),
    outputPath: successOutput,
    verifyService: false,
  })
  const success = spawnSync(
    'powershell.exe',
    ['-NoProfile', '-NonInteractive', '-EncodedCommand', Buffer.from(successHelper, 'utf16le').toString('base64')],
    { encoding: 'utf8' },
  )
  assert.equal(success.status, 0, success.stderr)
  assert.match(fs.readFileSync(successOutput, 'utf8'), /command=start[\s\S]*normal info log/)

  const installDir = path.join(tempDir, 'installed')
  const sourceCore = path.join(tempDir, 'source-core.cmd')
  const targetCore = path.join(installDir, 'frpp.cmd')
  const sourceConfig = path.join(tempDir, 'source.env')
  const targetConfig = path.join(installDir, '.env')
  const applyOutput = path.join(tempDir, 'apply.txt')
  fs.writeFileSync(sourceCore, '@echo off\r\necho %*>>"%~dp0commands.log"\r\nexit /b 0\r\n')
  fs.writeFileSync(sourceConfig, 'CLIENT_ID=test-client\r\n')
  fs.writeFileSync(applyOutput, '')
  fs.mkdirSync(installDir)
  fs.writeFileSync(`${targetCore}.new`, 'stale update')
  const applyHelper = buildWindowsServiceScript({
    action: 'apply',
    sourceBinary: sourceCore,
    tempConfig: sourceConfig,
    installDir,
    targetBinary: targetCore,
    targetConfig,
    outputPath: applyOutput,
    verifyService: false,
  })
  const apply = spawnSync(
    'powershell.exe',
    ['-NoProfile', '-NonInteractive', '-EncodedCommand', Buffer.from(applyHelper, 'utf16le').toString('base64')],
    { encoding: 'utf8' },
  )
  assert.equal(apply.status, 0, `${apply.stderr}\n${fs.readFileSync(applyOutput, 'utf8')}`)
  assert.match(fs.readFileSync(path.join(installDir, 'commands.log'), 'utf8'), /install client[\s\S]*start/)
  assert.equal(fs.existsSync(`${targetCore}.new`), false)
  spawnSync('icacls.exe', [targetConfig, '/reset'], { encoding: 'utf8' })

  const failureOutput = path.join(tempDir, 'failure.txt')
  fs.writeFileSync(fakeCore, '@echo off\r\necho intentional failure 1>&2\r\nexit /b 7\r\n')
  fs.writeFileSync(failureOutput, '')
  const failureHelper = buildWindowsServiceScript({
    action: 'start',
    installDir: tempDir,
    targetBinary: fakeCore,
    targetConfig: path.join(tempDir, '.env'),
    outputPath: failureOutput,
    verifyService: false,
  })
  const failure = spawnSync(
    'powershell.exe',
    ['-NoProfile', '-NonInteractive', '-EncodedCommand', Buffer.from(failureHelper, 'utf16le').toString('base64')],
    { encoding: 'utf8' },
  )
  assert.equal(failure.status, 1)
  assert.match(fs.readFileSync(failureOutput, 'utf8'), /exit code 7/)
})

test('builds a PowerShell elevation launcher with UTF-8 output forwarding', () => {
  const launcher = buildWindowsElevationLauncher('exit 0', "C:\\Temp\\result's.txt")
  assert.match(launcher, /-Verb RunAs/)
  assert.match(launcher, /Get-Content .* -Encoding UTF8/)
  assert.match(launcher, /result''s\.txt/)
})

test('redacts secrets from command output', () => {
  assert.equal(redactSecrets('client -s top-secret --secret=other', ['top-secret']), 'client -s [redacted] --secret=[redacted]')
  assert.equal(redactSecrets('CLIENT_SECRET=secret-value'), 'CLIENT_SECRET=[redacted]')
})

test('requires encrypted remote endpoints by default', () => {
  assert.doesNotThrow(() => validateProfile(secureProfile))
  assert.throws(
    () => validateProfile({ ...secureProfile, apiUrl: 'http://manager.example.com' }),
    /must use HTTPS/,
  )
  assert.throws(
    () => validateProfile({ ...secureProfile, rpcUrl: 'ws://manager.example.com' }),
    /must use grpc/,
  )
  assert.doesNotThrow(() =>
    validateProfile({
      ...secureProfile,
      apiUrl: 'http://127.0.0.1:9000',
      rpcUrl: 'ws://127.0.0.1:9001',
    }),
  )
  assert.doesNotThrow(() =>
    validateProfile({ ...secureProfile, apiUrl: 'http://manager.example.com', allowInsecure: true }),
  )
})

test('detects insecure remote profiles for the UI warning', () => {
  assert.equal(hasInsecureRemoteEndpoint(secureProfile), false)
  assert.equal(hasInsecureRemoteEndpoint({ ...secureProfile, apiUrl: 'http://manager.example.com' }), true)
})

test('derives the manager RPC port used by Windows health checks', () => {
  assert.equal(rpcPort('grpc://manager.example.com:9001'), 9001)
  assert.equal(rpcPort('wss://manager.example.com/wsgrpc'), 443)
  assert.equal(rpcPort('ws://manager.example.com/wsgrpc'), 80)
  assert.equal(rpcPort('invalid'), 0)
})

test('enrollment profiles persist the token and keep startup updates enabled', () => {
  const profile = { ...secureProfile, clientId: '', secret: '', joinToken: 'enroll-token' }
  assert.doesNotThrow(() => validateProfile(profile))
  assert.throws(() => validateProfile({ ...profile, joinToken: '' }), /enrollment token/)
  const config = buildManagedServiceConfig(profile, 'global-secret')
  assert.match(config, /^CLIENT_JOIN_TOKEN="enroll-token"$/m)
  assert.match(config, /^APP_AUTO_UPDATE=true$/m)
  assert.equal(redactSecrets('CLIENT_JOIN_TOKEN=abc --join-token=def -j ghi'), 'CLIENT_JOIN_TOKEN=[redacted] --join-token=[redacted] -j [redacted]')
})

test('removes ANSI colors from service errors', () => {
  assert.equal(redactSecrets('\x1b[31mLoad failed: 5\x1b[0m'), 'Load failed: 5')
})
