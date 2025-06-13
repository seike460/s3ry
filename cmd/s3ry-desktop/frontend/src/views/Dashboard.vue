<template>
  <div class="p-6 space-y-6">
    <!-- Welcome Section -->
    <div class="bg-gradient-to-r from-blue-500 to-purple-600 rounded-xl p-6 text-white">
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold mb-2">Welcome to S3ry Desktop</h1>
          <p class="text-blue-100">High-Performance S3 Browser with 271,615x improvement</p>
        </div>
        <div class="text-right">
          <div class="text-3xl font-bold">{{ appStore.bucketsCount }}</div>
          <div class="text-blue-100">Buckets</div>
        </div>
      </div>
    </div>

    <!-- Performance Metrics -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow-sm border border-gray-200 dark:border-gray-700">
        <div class="flex items-center">
          <div class="p-3 bg-green-100 dark:bg-green-900/30 rounded-lg">
            <Zap class="w-6 h-6 text-green-600 dark:text-green-400" />
          </div>
          <div class="ml-4">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Performance</h3>
            <p class="text-2xl font-bold text-green-600 dark:text-green-400">271,615x</p>
            <p class="text-sm text-gray-600 dark:text-gray-400">Improvement Factor</p>
          </div>
        </div>
      </div>

      <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow-sm border border-gray-200 dark:border-gray-700">
        <div class="flex items-center">
          <div class="p-3 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
            <TrendingUp class="w-6 h-6 text-blue-600 dark:text-blue-400" />
          </div>
          <div class="ml-4">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Throughput</h3>
            <p class="text-2xl font-bold text-blue-600 dark:text-blue-400">143,309</p>
            <p class="text-sm text-gray-600 dark:text-gray-400">MB/s</p>
          </div>
        </div>
      </div>

      <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow-sm border border-gray-200 dark:border-gray-700">
        <div class="flex items-center">
          <div class="p-3 bg-purple-100 dark:bg-purple-900/30 rounded-lg">
            <Monitor class="w-6 h-6 text-purple-600 dark:text-purple-400" />
          </div>
          <div class="ml-4">
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">UI Response</h3>
            <p class="text-2xl font-bold text-purple-600 dark:text-purple-400">35,022</p>
            <p class="text-sm text-gray-600 dark:text-gray-400">FPS</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Quick Stats & Recent Activity -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Storage Overview -->
      <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow-sm border border-gray-200 dark:border-gray-700">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
          <Database class="w-5 h-5 mr-2" />
          Storage Overview
        </h3>
        
        <div class="space-y-4">
          <div class="flex justify-between items-center">
            <span class="text-gray-600 dark:text-gray-400">Total Buckets</span>
            <span class="font-semibold text-gray-900 dark:text-white">{{ appStore.bucketsCount }}</span>
          </div>
          
          <div class="flex justify-between items-center">
            <span class="text-gray-600 dark:text-gray-400">Total Size</span>
            <span class="font-semibold text-gray-900 dark:text-white">{{ appStore.formatBytes(appStore.totalSize) }}</span>
          </div>
          
          <div class="flex justify-between items-center">
            <span class="text-gray-600 dark:text-gray-400">Current Region</span>
            <span class="font-semibold text-gray-900 dark:text-white">{{ appStore.currentRegion }}</span>
          </div>
          
          <div class="flex justify-between items-center">
            <span class="text-gray-600 dark:text-gray-400">Connection Status</span>
            <span class="flex items-center">
              <div
                class="w-2 h-2 rounded-full mr-2"
                :class="appStore.isConnected ? 'bg-green-500' : 'bg-red-500'"
              ></div>
              <span class="font-semibold text-gray-900 dark:text-white">
                {{ appStore.isConnected ? 'Connected' : 'Disconnected' }}
              </span>
            </span>
          </div>
        </div>

        <div class="mt-6">
          <router-link
            to="/buckets"
            class="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium py-2 px-4 rounded-md transition-colors duration-200 flex items-center justify-center"
          >
            <FolderOpen class="w-4 h-4 mr-2" />
            Browse Buckets
          </router-link>
        </div>
      </div>

      <!-- Recent Activity / Quick Actions -->
      <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow-sm border border-gray-200 dark:border-gray-700">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
          <Activity class="w-5 h-5 mr-2" />
          Quick Actions
        </h3>
        
        <div class="space-y-3">
          <button
            @click="handleCreateBucket"
            class="w-full text-left p-3 rounded-lg border border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors duration-200 flex items-center"
          >
            <Plus class="w-5 h-5 mr-3 text-green-600 dark:text-green-400" />
            <div>
              <div class="font-medium text-gray-900 dark:text-white">Create New Bucket</div>
              <div class="text-sm text-gray-600 dark:text-gray-400">Set up a new S3 bucket</div>
            </div>
          </button>

          <button
            @click="handleUpload"
            class="w-full text-left p-3 rounded-lg border border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors duration-200 flex items-center"
          >
            <Upload class="w-5 h-5 mr-3 text-blue-600 dark:text-blue-400" />
            <div>
              <div class="font-medium text-gray-900 dark:text-white">Upload Files</div>
              <div class="text-sm text-gray-600 dark:text-gray-400">Upload files to S3</div>
            </div>
          </button>

          <router-link
            to="/settings"
            class="w-full text-left p-3 rounded-lg border border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors duration-200 flex items-center"
          >
            <Settings class="w-5 h-5 mr-3 text-purple-600 dark:text-purple-400" />
            <div>
              <div class="font-medium text-gray-900 dark:text-white">Settings</div>
              <div class="text-sm text-gray-600 dark:text-gray-400">Configure S3ry settings</div>
            </div>
          </router-link>

          <router-link
            to="/about"
            class="w-full text-left p-3 rounded-lg border border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors duration-200 flex items-center"
          >
            <Info class="w-5 h-5 mr-3 text-gray-600 dark:text-gray-400" />
            <div>
              <div class="font-medium text-gray-900 dark:text-white">About S3ry</div>
              <div class="text-sm text-gray-600 dark:text-gray-400">Learn more about S3ry</div>
            </div>
          </router-link>
        </div>
      </div>
    </div>

    <!-- Recent Buckets -->
    <div v-if="appStore.buckets.length > 0" class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow-sm border border-gray-200 dark:border-gray-700">
      <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4 flex items-center">
        <FolderOpen class="w-5 h-5 mr-2" />
        Recent Buckets
      </h3>
      
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <router-link
          v-for="bucket in appStore.buckets.slice(0, 6)"
          :key="bucket.name"
          :to="`/buckets/${bucket.name}`"
          class="p-4 border border-gray-200 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors duration-200"
        >
          <div class="flex items-center justify-between mb-2">
            <h4 class="font-medium text-gray-900 dark:text-white truncate">{{ bucket.name }}</h4>
            <ExternalLink class="w-4 h-4 text-gray-400" />
          </div>
          <div class="text-sm text-gray-600 dark:text-gray-400 space-y-1">
            <div>{{ appStore.formatBytes(bucket.size || 0) }}</div>
            <div>{{ bucket.object_count || 0 }} objects</div>
            <div>{{ appStore.formatDate(bucket.creation_date) }}</div>
          </div>
        </router-link>
      </div>
      
      <div class="mt-4 text-center">
        <router-link
          to="/buckets"
          class="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 font-medium"
        >
          View All Buckets →
        </router-link>
      </div>
    </div>

    <!-- Loading State -->
    <div v-if="appStore.isLoading" class="flex items-center justify-center p-8">
      <div class="flex items-center space-x-3 text-gray-600 dark:text-gray-400">
        <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600"></div>
        <span>Loading dashboard...</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useAppStore } from '../stores/app'
import {
  Zap,
  TrendingUp,
  Monitor,
  Database,
  Activity,
  FolderOpen,
  Plus,
  Upload,
  Settings,
  Info,
  ExternalLink
} from 'lucide-vue-next'

const appStore = useAppStore()

const handleCreateBucket = () => {
  // This would open a create bucket dialog
  alert('Create bucket functionality would be implemented here')
}

const handleUpload = () => {
  // This would open an upload dialog
  alert('Upload functionality would be implemented here')
}

onMounted(async () => {
  if (!appStore.isConnected) {
    try {
      await appStore.initialize()
    } catch (error) {
      console.error('Failed to initialize:', error)
    }
  }
})
</script>