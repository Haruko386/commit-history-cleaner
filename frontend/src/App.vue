<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

type NavigationItem = {
  label: string
  icon: 'overview' | 'history' | 'objects' | 'cleanup'
  available: boolean
}

type RepositorySummary = {
  id: string
  name: string
  path: string
  branch: string | null
  detachedHead: boolean
  head: string | null
  commitCount: number
  workingTreeStatus: 'clean' | 'dirty' | 'unborn' | 'unavailable'
  gitDirectoryBytes: number
  analysisStatus: 'not_scanned' | 'scanning' | 'ready' | 'failed'
  openedAt: string
}

type ErrorResponse = {
  error?: {
    code?: string
    message?: string
    details?: {
      taskId?: string
    }
  }
}

type ScanTask = {
  taskId: string
  type: 'repository_scan'
  status: 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'
  progress: {
    phase: string
    current: number
    total: number | null
    percent: number | null
  }
  createdAt: string
  startedAt: string | null
  finishedAt: string | null
  error?: string | null
}

const navigation: NavigationItem[] = [
  { label: 'Overview', icon: 'overview', available: true },
  { label: 'Commit history', icon: 'history', available: false },
  { label: 'Large objects', icon: 'objects', available: false },
  { label: 'Cleanup plan', icon: 'cleanup', available: false },
]

const activeNavigation = ref('Overview')
const notice = ref('')
const noticeTone = ref<'info' | 'error' | 'success'>('info')
const backendStatus = ref<'checking' | 'online' | 'offline'>('checking')
const repositoryPath = ref('')
const repository = ref<RepositorySummary | null>(null)
const openingRepository = ref(false)
const scanTask = ref<ScanTask | null>(null)
const startingScan = ref(false)
const cancellingScan = ref(false)
const scanRequiresForce = ref(false)
const githubToken = ref('')
const githubStatus = ref<'idle' | 'checking' | 'connected' | 'error'>('idle')
const githubMessage = ref('Not connected')
let scanPollTimer: ReturnType<typeof setTimeout> | undefined

const scanIsActive = computed(() => scanTask.value?.status === 'queued' || scanTask.value?.status === 'running')
const scanButtonLabel = computed(() => {
  if (startingScan.value) return 'Starting scan…'
  if (scanIsActive.value) return 'Scanning…'
  if (scanRequiresForce.value || scanTask.value || repository.value?.analysisStatus === 'ready' || repository.value?.analysisStatus === 'failed') {
    return 'Scan again'
  }
  return 'Scan repository'
})
const scanProgressLabel = computed(() => {
  const task = scanTask.value
  if (!task) return 'Scan commit history before viewing analysis results.'
  if (task.status === 'queued') return 'Waiting for the scanner to start.'
  if (task.status === 'running') {
    const total = task.progress.total == null ? '?' : task.progress.total
    return `Scanning commits: ${task.progress.current} / ${total}`
  }
  if (task.status === 'completed') return `${task.progress.current} commits scanned.`
  if (task.status === 'cancelled') return 'The scan was cancelled.'
  return task.error ?? 'The scan failed.'
})

function showNotice(message: string, tone: 'info' | 'error' | 'success' = 'info') {
  notice.value = message
  noticeTone.value = tone
}

function selectNavigation(item: NavigationItem) {
  if (!item.available) {
    showNotice(`${item.label} is not available yet.`)
    return
  }

  activeNavigation.value = item.label
  notice.value = ''
}

function requestRepository() {
  document.querySelector<HTMLInputElement>('#repository-path')?.focus()
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = value / 1024
  let unit = units[0]
  for (let index = 1; size >= 1024 && index < units.length; index += 1) {
    size /= 1024
    unit = units[index]
  }
  return `${size.toFixed(size >= 10 ? 1 : 2)} ${unit}`
}

async function checkHealth() {
  backendStatus.value = 'checking'
  try {
    const response = await fetch('/api/v1/health')
    backendStatus.value = response.ok ? 'online' : 'offline'
  } catch {
    backendStatus.value = 'offline'
  }
}

async function openRepository() {
  const path = repositoryPath.value.trim()
  if (!path) {
    showNotice('Enter a local repository path.', 'error')
    return
  }

  openingRepository.value = true
  notice.value = ''
  try {
    const response = await fetch('/api/v1/repositories/open', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    })
    const body = (await response.json()) as { data?: RepositorySummary } & ErrorResponse
    if (!response.ok || !body.data) {
      showNotice(body.error?.message ?? 'The repository could not be opened.', 'error')
      return
    }

    clearScanPoll()
    scanTask.value = null
    scanRequiresForce.value = false
    repository.value = body.data
    repositoryPath.value = body.data.path
    showNotice(`Opened ${body.data.name}.`, 'success')
  } catch {
    showNotice('The local backend is unavailable.', 'error')
  } finally {
    openingRepository.value = false
  }
}

function clearScanPoll() {
  if (scanPollTimer !== undefined) {
    clearTimeout(scanPollTimer)
    scanPollTimer = undefined
  }
}

function updateRepositoryAnalysis(status: RepositorySummary['analysisStatus']) {
  if (repository.value) repository.value = { ...repository.value, analysisStatus: status }
}

function scheduleScanPoll(taskId: string) {
  clearScanPoll()
  scanPollTimer = setTimeout(() => void refreshScanTask(taskId), 800)
}

async function refreshScanTask(taskId: string) {
  try {
    const response = await fetch(`/api/v1/tasks/${encodeURIComponent(taskId)}`)
    const body = (await response.json()) as { data?: ScanTask } & ErrorResponse
    if (!response.ok || !body.data) {
      showNotice(body.error?.message ?? 'The scan status could not be loaded.', 'error')
      return
    }

    scanTask.value = body.data
    if (body.data.status === 'queued' || body.data.status === 'running') {
      updateRepositoryAnalysis('scanning')
      scheduleScanPoll(taskId)
      return
    }

    clearScanPoll()
    if (body.data.status === 'completed') {
      updateRepositoryAnalysis('ready')
      scanRequiresForce.value = true
      showNotice('Repository scan completed.', 'success')
    } else if (body.data.status === 'failed') {
      updateRepositoryAnalysis('failed')
      scanRequiresForce.value = true
      showNotice(body.data.error ?? 'The repository scan failed.', 'error')
    } else {
      updateRepositoryAnalysis('not_scanned')
      scanRequiresForce.value = true
      showNotice('Repository scan cancelled.')
    }
  } catch {
    showNotice('The local backend is unavailable.', 'error')
  }
}

async function startScan() {
  if (!repository.value || scanIsActive.value) return

  startingScan.value = true
  notice.value = ''
  const force = scanRequiresForce.value || scanTask.value !== null || repository.value.analysisStatus === 'ready' || repository.value.analysisStatus === 'failed'

  try {
    const response = await fetch('/api/v1/repositories/current/scans', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ force }),
    })
    const body = (await response.json()) as { data?: ScanTask } & ErrorResponse

    if (response.status === 409 && body.error?.details?.taskId) {
      const taskId = body.error.details.taskId
      if (body.error.code === 'SCAN_ALREADY_RUNNING') {
        showNotice('A repository scan is already running.')
        await refreshScanTask(taskId)
      } else {
        scanRequiresForce.value = true
        showNotice('A previous scan already exists. Select “Scan again” to replace it.')
      }
      return
    }

    if (!response.ok || !body.data) {
      showNotice(body.error?.message ?? 'The repository scan could not be started.', 'error')
      return
    }

    scanTask.value = body.data
    scanRequiresForce.value = false
    updateRepositoryAnalysis('scanning')
    showNotice('Repository scan started.', 'success')
    scheduleScanPoll(body.data.taskId)
  } catch {
    showNotice('The local backend is unavailable.', 'error')
  } finally {
    startingScan.value = false
  }
}

async function cancelScan() {
  const taskId = scanTask.value?.taskId
  if (!taskId || !scanIsActive.value) return

  cancellingScan.value = true
  try {
    const response = await fetch(`/api/v1/tasks/${encodeURIComponent(taskId)}`, { method: 'DELETE' })
    if (!response.ok) {
      const body = (await response.json()) as ErrorResponse
      showNotice(body.error?.message ?? 'The repository scan could not be cancelled.', 'error')
      return
    }

    clearScanPoll()
    await refreshScanTask(taskId)
  } catch {
    showNotice('The local backend is unavailable.', 'error')
  } finally {
    cancellingScan.value = false
  }
}

async function checkGithubConnection() {
  const token = githubToken.value.trim()
  if (!token) {
    githubStatus.value = 'error'
    githubMessage.value = 'Enter a GitHub access token.'
    return
  }

  githubStatus.value = 'checking'
  githubMessage.value = 'Checking connection…'
  try {
    const response = await fetch('/api/v1/github/connection', {
      headers: { Authorization: `Bearer ${token}` },
    })
    const body = (await response.json()) as { data?: boolean } & ErrorResponse
    if (!response.ok || body.data !== true) {
      githubStatus.value = 'error'
      githubMessage.value = body.error?.message ?? 'GitHub connection failed.'
      return
    }

    githubStatus.value = 'connected'
    githubMessage.value = 'Connected to GitHub'
    githubToken.value = ''
  } catch {
    githubStatus.value = 'error'
    githubMessage.value = 'The local backend is unavailable.'
  }
}

onMounted(checkHealth)
onBeforeUnmount(clearScanPoll)
</script>

<template>
  <div class="app-shell">
    <header class="global-header">
      <button class="icon-button" type="button" aria-label="Open navigation">
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M2 4h12M2 8h12M2 12h12" /></svg>
      </button>

      <a class="app-identity" href="#" aria-label="Git History Cleaner home">
        <span class="app-logo"><img src="/favicon.svg" alt="" /></span>
        <strong>Git History Cleaner</strong>
      </a>

      <div class="header-actions">
        <span class="backend-status" :class="`backend-${backendStatus}`">
          <span class="status-dot" aria-hidden="true"></span>
          {{ backendStatus === 'online' ? 'Backend online' : backendStatus === 'offline' ? 'Backend offline' : 'Checking backend' }}
        </span>
        <button class="button button-primary" type="button" @click="requestRepository">
          <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M1.75 3.75h5l1.5 1.5h6v7.5H1.75v-9Z" /><path d="M1.75 6.25h12.5" /></svg>
          Open repository
        </button>
      </div>
    </header>

    <section class="repository-header" aria-label="Repository navigation">
      <div class="repository-title">
        <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 2.25h8.25A1.75 1.75 0 0 1 13 4v9.75H4.5A1.5 1.5 0 0 1 3 12.25v-10Z" /><path d="M5 2.25v11.5M3 11.75h10" /></svg>
        <span>{{ repository?.name ?? 'No repository selected' }}</span>
        <span class="label">Local</span>
      </div>

      <nav class="tab-navigation" aria-label="Repository sections">
        <button v-for="item in navigation" :key="item.label" class="tab-item" :class="{ selected: activeNavigation === item.label }" type="button" @click="selectNavigation(item)">
          <svg v-if="item.icon === 'overview'" viewBox="0 0 16 16" aria-hidden="true"><path d="M2.25 2.25h4.5v4.5h-4.5v-4.5Zm7 0h4.5v4.5h-4.5v-4.5Zm-7 7h4.5v4.5h-4.5v-4.5Zm7 0h4.5v4.5h-4.5v-4.5Z" /></svg>
          <svg v-else-if="item.icon === 'history'" viewBox="0 0 16 16" aria-hidden="true"><path d="M2.25 8A5.75 5.75 0 1 0 4 3.88L2.25 5.5" /><path d="M2.25 2.5v3h3M8 4.5V8l2.5 1.5" /></svg>
          <svg v-else-if="item.icon === 'objects'" viewBox="0 0 16 16" aria-hidden="true"><path d="m8 1.75 5.75 3.2L8 8.25l-5.75-3.3L8 1.75Z" /><path d="m2.25 8.1 5.75 3.3 5.75-3.3M2.25 11.2 8 14.4l5.75-3.2" /></svg>
          <svg v-else viewBox="0 0 16 16" aria-hidden="true"><path d="M2.25 4h11.5M5 4V2.25h6V4M4 4l.6 9.75h6.8L12 4M6.5 6.5v4.75M9.5 6.5v4.75" /></svg>
          {{ item.label }}
        </button>
      </nav>
    </section>

    <main class="page-content">
      <div v-if="notice" class="flash" :class="`flash-${noticeTone}`" role="status">
        <svg viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="6.25" /><path d="M8 7v4M8 4.5v.25" /></svg>
        <span>{{ notice }}</span>
        <button type="button" aria-label="Dismiss message" @click="notice = ''"><svg viewBox="0 0 16 16" aria-hidden="true"><path d="m3.5 3.5 9 9m0-9-9 9" /></svg></button>
      </div>

      <div class="layout">
        <div class="main-column">
          <section class="box" aria-labelledby="repository-title">
            <header class="box-header"><strong id="repository-title">Repository</strong></header>

            <div v-if="!repository" class="blank-state">
              <svg class="blank-state-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M2.75 5.75h7l2 2h9.5v11.5H2.75V5.75Z" /><path d="M2.75 9.25h18.5" /></svg>
              <h1>Open a Git repository</h1>
              <p>Enter the absolute path to a local Git repository.</p>
              <form class="repository-form" @submit.prevent="openRepository">
                <label for="repository-path">Repository path</label>
                <div class="input-group">
                  <input id="repository-path" v-model="repositoryPath" type="text" placeholder="E:\projects\example" autocomplete="off" spellcheck="false" />
                  <button class="button button-primary" type="submit" :disabled="openingRepository">
                    {{ openingRepository ? 'Opening…' : 'Open repository' }}
                  </button>
                </div>
              </form>
            </div>

            <div v-else class="repository-summary">
              <div class="summary-heading">
                <div><strong>{{ repository.name }}</strong><span>{{ repository.path }}</span></div>
                <button class="button" type="button" @click="repository = null">Open another</button>
              </div>
              <dl class="summary-grid">
                <div><dt>Branch</dt><dd>{{ repository.branch ?? (repository.detachedHead ? 'Detached HEAD' : 'Unborn') }}</dd></div>
                <div><dt>HEAD</dt><dd class="monospace">{{ repository.head?.slice(0, 12) ?? '—' }}</dd></div>
                <div><dt>Commits</dt><dd>{{ repository.commitCount }}</dd></div>
                <div><dt>Working tree</dt><dd class="status-value">{{ repository.workingTreeStatus }}</dd></div>
                <div><dt>.git size</dt><dd>{{ formatBytes(repository.gitDirectoryBytes) }}</dd></div>
                <div><dt>Analysis</dt><dd>{{ repository.analysisStatus.replace('_', ' ') }}</dd></div>
              </dl>

              <div class="scan-panel">
                <div class="scan-copy">
                  <strong>Repository scan</strong>
                  <span>{{ scanProgressLabel }}</span>
                  <progress v-if="scanTask && (scanIsActive || scanTask.status === 'completed')" :value="scanTask.progress.percent ?? 0" max="100">
                    {{ scanTask.progress.percent ?? 0 }}%
                  </progress>
                </div>
                <div class="scan-actions">
                  <button v-if="scanIsActive" class="button" type="button" :disabled="cancellingScan" @click="cancelScan">
                    {{ cancellingScan ? 'Cancelling…' : 'Cancel' }}
                  </button>
                  <button class="button button-primary" type="button" :disabled="startingScan || scanIsActive" @click="startScan">
                    {{ scanButtonLabel }}
                  </button>
                </div>
              </div>
            </div>
          </section>

          <section class="box recent-box" aria-labelledby="recent-title">
            <header class="box-header"><strong id="recent-title">Recent repositories</strong></header>
            <div class="empty-row">
              <svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 2.25h8.25A1.75 1.75 0 0 1 13 4v9.75H4.5A1.5 1.5 0 0 1 3 12.25v-10Z" /><path d="M5 2.25v11.5M3 11.75h10" /></svg>
              <div><strong>No recent repositories</strong><p>Recent repository storage will be added with the desktop app.</p></div>
            </div>
          </section>
        </div>

        <aside class="about" aria-labelledby="about-title">
          <section class="sidebar-section">
            <h2 id="about-title">About</h2>
            <p>Analyze local Git history and prepare cleanup commands before rewriting commits.</p>
            <div class="safety-note">
              <svg viewBox="0 0 16 16" aria-hidden="true"><rect x="3" y="7" width="10" height="7" rx="1.5" /><path d="M5.25 7V5a2.75 2.75 0 0 1 5.5 0v2" /></svg>
              <span><strong>Read-only by default</strong>Cleanup actions require a preview and confirmation.</span>
            </div>
          </section>

          <section class="sidebar-section github-section" aria-labelledby="github-title">
            <h2 id="github-title">GitHub connection</h2>
            <p>Check an access token without saving it in the browser.</p>
            <form @submit.prevent="checkGithubConnection">
              <label for="github-token">Personal access token</label>
              <input id="github-token" v-model="githubToken" type="password" placeholder="github_pat_…" autocomplete="off" spellcheck="false" />
              <button class="button" type="submit" :disabled="githubStatus === 'checking'">
                {{ githubStatus === 'checking' ? 'Checking…' : 'Check connection' }}
              </button>
            </form>
            <p class="connection-status" :class="`connection-${githubStatus}`"><span class="status-dot" aria-hidden="true"></span>{{ githubMessage }}</p>
          </section>
        </aside>
      </div>
    </main>
  </div>
</template>
