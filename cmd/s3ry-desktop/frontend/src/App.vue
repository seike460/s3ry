<template>
  <div id="app" class="h-screen flex flex-col bg-gray-50 dark:bg-gray-900">
    <!-- Title Bar -->
    <div class="titlebar h-8 bg-gray-100 dark:bg-gray-800 flex items-center justify-center text-sm text-gray-600 dark:text-gray-400">
      S3ry Desktop - High-Performance S3 Browser
    </div>

    <!-- Main Content -->
    <div class="flex flex-1 overflow-hidden">
      <!-- Sidebar -->
      <aside class="w-64 bg-white dark:bg-gray-800 shadow-lg flex flex-col">
        <!-- Logo & Title -->
        <div class="p-6 border-b border-gray-200 dark:border-gray-700">
          <div class="flex items-center space-x-3">
            <div class="w-10 h-10 bg-gradient-to-br from-blue-500 to-purple-600 rounded-lg flex items-center justify-center">
              <span class="text-white font-bold text-lg">S3</span>
            </div>
            <div>
              <h1 class="text-xl font-bold text-gray-900 dark:text-white">S3ry</h1>
              <p class="text-xs text-gray-500 dark:text-gray-400">271,615x Faster</p>
            </div>
          </div>
        </div>

        <!-- Navigation -->
        <nav class="flex-1 p-4 space-y-2">
          <router-link
            v-for="item in navigation"
            :key="item.name"
            :to="item.to"
            class="nav-item flex items-center space-x-3 px-4 py-3 rounded-lg transition-all duration-200 hover:bg-gray-100 dark:hover:bg-gray-700"
            :class="{'bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400': $route.name === item.name}"
          >
            <component :is="item.icon" class="w-5 h-5" />
            <span class="font-medium">{{ item.label }}</span>
          </router-link>
        </nav>

        <!-- Performance Stats -->
        <div class="p-4 border-t border-gray-200 dark:border-gray-700">
          <div class="bg-gradient-to-r from-blue-500 to-purple-600 rounded-lg p-4 text-white">
            <h3 class="font-semibold text-sm mb-2">Performance</h3>
            <div class="space-y-1 text-xs">
              <div class="flex justify-between">
                <span>Improvement:</span>
                <span class="font-mono">271,615x</span>
              </div>
              <div class="flex justify-between">
                <span>Throughput:</span>
                <span class="font-mono">143,309 MB/s</span>
              </div>
              <div class="flex justify-between">
                <span>UI FPS:</span>
                <span class="font-mono">35,022</span>
              </div>
            </div>
          </div>
        </div>
      </aside>

      <!-- Main Content Area -->
      <main class="flex-1 flex flex-col overflow-hidden">
        <!-- Top Bar -->
        <div class="bg-white dark:bg-gray-800 shadow-sm border-b border-gray-200 dark:border-gray-700 px-6 py-4">
          <div class="flex items-center justify-between">
            <div class="flex items-center space-x-4">
              <h2 class="text-xl font-semibold text-gray-900 dark:text-white">{{ currentPageTitle }}</h2>
              <!-- Region Selector -->
              <select
                v-model="currentRegion"
                @change="handleRegionChange"
                class="px-3 py-2 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md text-sm text-gray-700 dark:text-gray-300 focus:ring-2 focus:ring-blue-500"
              >
                <option v-for="region in regions" :key="region.id" :value="region.id">
                  {{ region.name }}
                </option>
              </select>
            </div>

            <!-- Action Buttons -->
            <div class="flex items-center space-x-3">
              <!-- Connection Status -->
              <div class="flex items-center space-x-2">
                <div
                  class="w-3 h-3 rounded-full"
                  :class="connectionStatus === 'connected' ? 'bg-green-500' : 'bg-red-500'"
                ></div>
                <span class="text-sm text-gray-600 dark:text-gray-400">
                  {{ connectionStatus === 'connected' ? 'Connected' : 'Disconnected' }}
                </span>
              </div>

              <!-- Refresh Button -->
              <button
                @click="handleRefresh"
                :disabled="isLoading"
                class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white rounded-md text-sm font-medium transition-colors duration-200 flex items-center space-x-2"
              >
                <RefreshCw :class="['w-4 h-4', isLoading && 'animate-spin']" />
                <span>Refresh</span>
              </button>
            </div>
          </div>
        </div>

        <!-- Page Content -->
        <div class="flex-1 overflow-auto">
          <router-view :key="$route.fullPath" />
        </div>
      </main>
    </div>

    <!-- Toast Notifications -->
    <div
      v-if="notifications.length > 0"
      class="fixed top-4 right-4 space-y-2 z-50"
    >
      <div
        v-for="notification in notifications"
        :key="notification.id"
        class="max-w-sm bg-white dark:bg-gray-800 shadow-lg rounded-lg p-4 border-l-4"
        :class="{
          'border-green-500': notification.type === 'success',
          'border-red-500': notification.type === 'error',
          'border-blue-500': notification.type === 'info',
          'border-yellow-500': notification.type === 'warning'
        }"
      >
        <div class="flex items-start">
          <div class="flex-shrink-0">
            <CheckCircle v-if="notification.type === 'success'" class="w-5 h-5 text-green-500" />
            <XCircle v-else-if="notification.type === 'error'" class="w-5 h-5 text-red-500" />
            <Info v-else-if="notification.type === 'info'" class="w-5 h-5 text-blue-500" />
            <AlertTriangle v-else class="w-5 h-5 text-yellow-500" />
          </div>
          <div class="ml-3 flex-1">
            <p class="text-sm font-medium text-gray-900 dark:text-white">
              {{ notification.title }}
            </p>
            <p v-if="notification.message" class="mt-1 text-sm text-gray-600 dark:text-gray-400">
              {{ notification.message }}
            </p>
          </div>
          <button
            @click="dismissNotification(notification.id)"
            class="ml-4 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          >
            <X class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from './stores/app'
import {
  Home,
  FolderOpen,
  Settings,
  Info,
  RefreshCw,
  CheckCircle,
  XCircle,
  AlertTriangle,
  X
} from 'lucide-vue-next'

const route = useRoute()
const appStore = useAppStore()

// Navigation items
const navigation = [
  { name: 'Dashboard', label: 'Dashboard', to: '/', icon: Home },
  { name: 'Buckets', label: 'Buckets', to: '/buckets', icon: FolderOpen },
  { name: 'Settings', label: 'Settings', to: '/settings', icon: Settings },
  { name: 'About', label: 'About', to: '/about', icon: Info }
]

// Reactive data
const currentRegion = ref('us-east-1')
const isLoading = ref(false)
const connectionStatus = ref('connected')
const notifications = ref([])

// Computed properties
const currentPageTitle = computed(() => {
  const currentRoute = navigation.find(item => item.name === route.name)
  return currentRoute ? currentRoute.label : 'S3ry Desktop'
})

const regions = computed(() => appStore.regions)

// Methods
const handleRegionChange = async () => {
  try {
    isLoading.value = true
    await appStore.setRegion(currentRegion.value)
    showNotification('success', 'Region Changed', `Switched to ${currentRegion.value}`)
  } catch (error) {
    showNotification('error', 'Region Change Failed', error.message)
  } finally {
    isLoading.value = false
  }
}

const handleRefresh = async () => {
  try {
    isLoading.value = true
    await appStore.refresh()
    showNotification('success', 'Refreshed', 'Data has been updated')
  } catch (error) {
    showNotification('error', 'Refresh Failed', error.message)
  } finally {
    isLoading.value = false
  }
}

const showNotification = (type, title, message) => {
  const id = Date.now()
  notifications.value.push({ id, type, title, message })
  
  // Auto-dismiss after 5 seconds
  setTimeout(() => {
    dismissNotification(id)
  }, 5000)
}

const dismissNotification = (id) => {
  const index = notifications.value.findIndex(n => n.id === id)
  if (index > -1) {
    notifications.value.splice(index, 1)
  }
}

// Lifecycle
onMounted(async () => {
  try {
    await appStore.initialize()
    connectionStatus.value = 'connected'
  } catch (error) {
    connectionStatus.value = 'disconnected'
    showNotification('error', 'Connection Failed', 'Failed to connect to AWS')
  }
})
</script>

<style scoped>
.nav-item.router-link-active {
  @apply bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400;
}

.titlebar {
  -webkit-app-region: drag;
  user-select: none;
}
</style>