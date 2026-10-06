<script setup lang="ts">
import { ref } from 'vue'

type NavigationItem = {
  label: string
  icon: 'overview' | 'history' | 'objects' | 'cleanup'
  available: boolean
}

const navigation: NavigationItem[] = [
  { label: 'Overview', icon: 'overview', available: true },
  { label: 'Commit history', icon: 'history', available: false },
  { label: 'Large objects', icon: 'objects', available: false },
  { label: 'Cleanup plan', icon: 'cleanup', available: false },
]

const activeNavigation = ref('Overview')
const notice = ref('')

function selectNavigation(item: NavigationItem) {
  if (!item.available) {
    notice.value = `Open a repository to use ${item.label.toLowerCase()}.`
    return
  }

  activeNavigation.value = item.label
  notice.value = ''
}

function requestRepository() {
  notice.value = 'Repository selection is waiting for the Wails backend binding.'
}
</script>

<template>
  <div class="app-shell">
    <header class="global-header">
      <button class="icon-button" type="button" aria-label="Open navigation">
        <svg viewBox="0 0 16 16" aria-hidden="true">
          <path d="M2 4h12M2 8h12M2 12h12" />
        </svg>
      </button>

      <a class="app-identity" href="#" aria-label="Git History Cleaner home">
        <span class="app-logo">
          <img src="/favicon.svg" alt="" />
        </span>
        <strong>Git History Cleaner</strong>
      </a>

      <div class="header-actions">
        <span class="local-label">
          <svg viewBox="0 0 16 16" aria-hidden="true">
            <path d="M8 1.5 13 3v4c0 3.5-2 5.9-5 7.5C5 12.9 3 10.5 3 7V3l5-1.5Z" />
            <path d="m5.75 7.75 1.5 1.5 3-3" />
          </svg>
          Local only
        </span>
        <button class="button button-primary" type="button" @click="requestRepository">
          <svg viewBox="0 0 16 16" aria-hidden="true">
            <path d="M1.75 3.75h5l1.5 1.5h6v7.5H1.75v-9Z" />
            <path d="M1.75 6.25h12.5" />
          </svg>
          Open repository
        </button>
      </div>
    </header>

    <section class="repository-header" aria-label="Repository navigation">
      <div class="repository-title">
        <svg viewBox="0 0 16 16" aria-hidden="true">
          <path d="M3 2.25h8.25A1.75 1.75 0 0 1 13 4v9.75H4.5A1.5 1.5 0 0 1 3 12.25v-10Z" />
          <path d="M5 2.25v11.5M3 11.75h10" />
        </svg>
        <span>No repository selected</span>
        <span class="label">Local</span>
      </div>

      <nav class="tab-navigation" aria-label="Repository sections">
        <button
          v-for="item in navigation"
          :key="item.label"
          class="tab-item"
          :class="{ selected: activeNavigation === item.label }"
          type="button"
          @click="selectNavigation(item)"
        >
          <svg v-if="item.icon === 'overview'" viewBox="0 0 16 16" aria-hidden="true">
            <path d="M2.25 2.25h4.5v4.5h-4.5v-4.5Zm7 0h4.5v4.5h-4.5v-4.5Zm-7 7h4.5v4.5h-4.5v-4.5Zm7 0h4.5v4.5h-4.5v-4.5Z" />
          </svg>
          <svg v-else-if="item.icon === 'history'" viewBox="0 0 16 16" aria-hidden="true">
            <path d="M2.25 8A5.75 5.75 0 1 0 4 3.88L2.25 5.5" />
            <path d="M2.25 2.5v3h3M8 4.5V8l2.5 1.5" />
          </svg>
          <svg v-else-if="item.icon === 'objects'" viewBox="0 0 16 16" aria-hidden="true">
            <path d="m8 1.75 5.75 3.2L8 8.25l-5.75-3.3L8 1.75Z" />
            <path d="m2.25 8.1 5.75 3.3 5.75-3.3M2.25 11.2 8 14.4l5.75-3.2" />
          </svg>
          <svg v-else viewBox="0 0 16 16" aria-hidden="true">
            <path d="M2.25 4h11.5M5 4V2.25h6V4M4 4l.6 9.75h6.8L12 4M6.5 6.5v4.75M9.5 6.5v4.75" />
          </svg>
          {{ item.label }}
        </button>
      </nav>
    </section>

    <main class="page-content">
      <div v-if="notice" class="flash" role="status">
        <svg viewBox="0 0 16 16" aria-hidden="true">
          <circle cx="8" cy="8" r="6.25" />
          <path d="M8 7v4M8 4.5v.25" />
        </svg>
        <span>{{ notice }}</span>
        <button type="button" aria-label="Dismiss message" @click="notice = ''">
          <svg viewBox="0 0 16 16" aria-hidden="true"><path d="m3.5 3.5 9 9m0-9-9 9" /></svg>
        </button>
      </div>

      <div class="layout">
        <div class="main-column">
          <section class="box" aria-labelledby="open-title">
            <header class="box-header">
              <strong>Repository</strong>
            </header>
            <div class="blank-state">
              <svg class="blank-state-icon" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M2.75 5.75h7l2 2h9.5v11.5H2.75V5.75Z" />
                <path d="M2.75 9.25h18.5" />
              </svg>
              <h1 id="open-title">Open a Git repository</h1>
              <p>Select a local repository to inspect its commits and stored objects.</p>
              <button class="button button-primary" type="button" @click="requestRepository">
                Open repository
              </button>
            </div>
          </section>

          <section class="box recent-box" aria-labelledby="recent-title">
            <header class="box-header">
              <strong id="recent-title">Recent repositories</strong>
            </header>
            <div class="empty-row">
              <svg viewBox="0 0 16 16" aria-hidden="true">
                <path d="M3 2.25h8.25A1.75 1.75 0 0 1 13 4v9.75H4.5A1.5 1.5 0 0 1 3 12.25v-10Z" />
                <path d="M5 2.25v11.5M3 11.75h10" />
              </svg>
              <div>
                <strong>No recent repositories</strong>
                <p>Repositories you open will be listed here.</p>
              </div>
            </div>
          </section>
        </div>

        <aside class="about" aria-labelledby="about-title">
          <h2 id="about-title">About</h2>
          <p>Analyze local Git history and prepare cleanup commands before rewriting commits.</p>
          <ul class="about-list">
            <li>
              <svg viewBox="0 0 16 16" aria-hidden="true">
                <path d="M2.25 8A5.75 5.75 0 1 0 4 3.88L2.25 5.5" />
                <path d="M2.25 2.5v3h3" />
              </svg>
              Commit history
            </li>
            <li>
              <svg viewBox="0 0 16 16" aria-hidden="true">
                <path d="M8 1.75v12.5M4.5 5.25 8 1.75l3.5 3.5M4.5 10.75 8 14.25l3.5-3.5" />
              </svg>
              Large objects
            </li>
            <li>
              <svg viewBox="0 0 16 16" aria-hidden="true">
                <path d="M8 1.5 13 3v4c0 3.5-2 5.9-5 7.5C5 12.9 3 10.5 3 7V3l5-1.5Z" />
              </svg>
              Cleanup preview
            </li>
          </ul>
          <hr />
          <div class="safety-note">
            <svg viewBox="0 0 16 16" aria-hidden="true">
              <rect x="3" y="7" width="10" height="7" rx="1.5" />
              <path d="M5.25 7V5a2.75 2.75 0 0 1 5.5 0v2" />
            </svg>
            <span><strong>Read-only by default</strong>Cleanup actions require a preview and confirmation.</span>
          </div>
        </aside>
      </div>
    </main>
  </div>
</template>
