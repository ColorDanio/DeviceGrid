const { app, BrowserWindow, shell } = require('electron')
const { spawn } = require('child_process')
const path = require('path')
const http = require('http')
const fs = require('fs')
const os = require('os')

let mainWindow = null
let serverProcess = null
let serverPort = 18080

function getServerBinary() {
  const platform = os.platform()
  const arch = os.arch()
  let binaryName = 'devicegrid-server'
  if (platform === 'win32') binaryName += '.exe'

  const possiblePaths = [
    path.join(process.resourcesPath, 'bin', binaryName),
    path.join(__dirname, '..', 'bin', binaryName),
    path.join(__dirname, '..', 'bin', 'devicegrid-server'),
  ]

  for (const p of possiblePaths) {
    if (fs.existsSync(p)) return p
  }
  return null
}

function getDataDir() {
  const dir = path.join(app.getPath('userData'), 'data')
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
  return dir
}

function getConfigDir() {
  const dir = path.join(app.getPath('userData'), 'config')
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
  return dir
}

function writeConfig() {
  const configDir = getConfigDir()
  const dataDir = getDataDir()
  const configPath = path.join(configDir, 'config.yaml')

  // Generate stable secrets per installation (stored in config dir)
  const secretsFile = path.join(configDir, '.secrets')
  let secrets = { jwt: '', masterKey: '' }
  if (fs.existsSync(secretsFile)) {
    try { secrets = JSON.parse(fs.readFileSync(secretsFile, 'utf8')) } catch {}
  }
  if (!secrets.jwt) {
    secrets.jwt = require('crypto').randomBytes(32).toString('hex')
    secrets.masterKey = require('crypto').randomBytes(32).toString('hex')
    fs.writeFileSync(secretsFile, JSON.stringify(secrets), { mode: 0o600 })
  }

  const config = `server:
  host: "127.0.0.1"
  port: ${serverPort}
  mode: "debug"

auth:
  jwt_secret: "${secrets.jwt}"
  jwt_expire: "168h"

crypto:
  master_key: "${secrets.masterKey}"

database:
  driver: "sqlite"
  sqlite:
    path: "${dataDir.replace(/\\/g, '/')}/device_grid.db"

redis:
  addr: "localhost:6379"
  db: 0

agent:
  grpc_port: 19090

ssh:
  connect_timeout: "10s"
  keepalive_interval: "30s"
  max_connections: 50

deploy:
  max_concurrent: 20
  timeout: "30m"

network:
  environment: "public"
`
  fs.writeFileSync(configPath, config)
  return configPath
}

function startServer() {
  return new Promise((resolve, reject) => {
    const binary = getServerBinary()
    if (!binary) {
      reject(new Error('Server binary not found'))
      return
    }

    const configPath = writeConfig()
    serverProcess = spawn(binary, [], {
      env: {
        ...process.env,
        DG_CONFIG_PATH: configPath,
      },
      stdio: ['ignore', 'pipe', 'pipe'],
    })

    serverProcess.stdout.on('data', (data) => {
      const msg = data.toString().trim()
      if (msg.includes('server starting')) {
        resolve()
      }
    })

    serverProcess.stderr.on('data', (data) => {
      const msg = data.toString().trim()
      if (msg.includes('server starting')) {
        resolve()
      }
    })

    serverProcess.on('error', (err) => {
      reject(err)
    })

    serverProcess.on('exit', (code) => {
      if (code !== 0 && code !== null) {
        console.error(`Server exited with code ${code}`)
      }
    })

    setTimeout(() => resolve(), 5000)
  })
}

function waitForServer() {
  return new Promise((resolve) => {
    const check = () => {
      const req = http.get(`http://127.0.0.1:${serverPort}/healthz`, (res) => {
        if (res.statusCode === 200) {
          resolve()
        } else {
          setTimeout(check, 500)
        }
      })
      req.on('error', () => {
        setTimeout(check, 500)
      })
      req.setTimeout(2000, () => {
        req.destroy()
        setTimeout(check, 500)
      })
    }
    check()
  })
}

async function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1440,
    height: 900,
    minWidth: 1024,
    minHeight: 680,
    title: 'DeviceGrid',
    backgroundColor: '#0b0f17',
    autoHideMenuBar: true,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
    },
    icon: path.join(__dirname, 'icon.png'),
  })

  mainWindow.loadURL(`http://127.0.0.1:${serverPort}`)

  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url)
    return { action: 'deny' }
  })

  mainWindow.on('closed', () => {
    mainWindow = null
  })
}

async function bootstrap() {
  try {
    await startServer()
    await waitForServer()
  } catch (err) {
    console.error('Failed to start server:', err)
  }
  await createWindow()
}

app.whenReady().then(bootstrap)

app.on('window-all-closed', () => {
  if (serverProcess) {
    serverProcess.kill('SIGTERM')
    setTimeout(() => {
      if (serverProcess) serverProcess.kill('SIGKILL')
      app.quit()
    }, 3000)
  } else {
    app.quit()
  }
})

app.on('before-quit', () => {
  if (serverProcess) {
    serverProcess.kill('SIGTERM')
  }
})

process.on('exit', () => {
  if (serverProcess) {
    serverProcess.kill('SIGKILL')
  }
})
