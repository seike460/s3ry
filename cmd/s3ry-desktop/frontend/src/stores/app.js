import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// Wails runtime imports (these will be available in the desktop app)
// For development, we'll create mock functions
const isDesktop = typeof window !== 'undefined' && window.go

const mockAPI = {
  desktop: {
    App: {
      GetAppInfo: () => Promise.resolve({
        name: 'S3ry Desktop',
        version: '2.0.0',
        platform: 'Desktop (Mock)',
        performance: {
          improvement_factor: 271615.44,
          throughput_mbps: 143309.18,
          ui_fps: 35022.6
        }
      }),
      ListRegions: () => Promise.resolve([
        { id: 'us-east-1', name: 'US East (N. Virginia)' },
        { id: 'us-west-1', name: 'US West (N. California)' },
        { id: 'us-west-2', name: 'US West (Oregon)' },
        { id: 'eu-west-1', name: 'Europe (Ireland)' },
        { id: 'eu-central-1', name: 'Europe (Frankfurt)' },
        { id: 'ap-southeast-1', name: 'Asia Pacific (Singapore)' },
        { id: 'ap-northeast-1', name: 'Asia Pacific (Tokyo)' },
      ]),
      SetRegion: (region) => Promise.resolve(),
      ListBuckets: () => Promise.resolve([
        {
          name: 'my-example-bucket',
          creation_date: '2024-01-15T10:30:00Z',
          region: 'us-east-1',
          object_count: 156,
          size: 1048576000
        },
        {
          name: 'demo-data-bucket',
          creation_date: '2024-02-20T14:45:00Z',
          region: 'us-east-1',
          object_count: 89,
          size: 524288000
        }
      ]),
      CreateBucket: (name, region) => Promise.resolve(),
      DeleteBucket: (name) => Promise.resolve(),
      ListObjects: (bucket, prefix) => Promise.resolve([
        {
          key: 'folder1/document.pdf',
          size: 2048576,
          last_modified: '2024-06-10T12:00:00Z',
          etag: '"d41d8cd98f00b204e9800998ecf8427e"',
          storage_class: 'STANDARD',
          is_folder: false
        },
        {
          key: 'images/photo.jpg',
          size: 1024000,
          last_modified: '2024-06-09T15:30:00Z',
          etag: '"098f6bcd4621d373cade4e832627b4f6"',
          storage_class: 'STANDARD',
          is_folder: false
        }
      ]),
      GetObjectMetadata: (bucket, key) => Promise.resolve({
        content_type: 'application/pdf',
        content_length: 2048576,
        last_modified: '2024-06-10T12:00:00Z',
        etag: '"d41d8cd98f00b204e9800998ecf8427e"',
        metadata: {
          'user-defined-key': 'user-defined-value'
        }
      }),
      DownloadObject: (bucket, key, targetPath) => Promise.resolve(),
      UploadObject: (bucket, key, filePath) => Promise.resolve(),
      DeleteObject: (bucket, key) => Promise.resolve(),
      SearchObjects: (bucket, pattern) => Promise.resolve([]),
      GetBucketAnalytics: (bucket) => Promise.resolve({
        total_objects: 1247,
        total_size: '15.3 GB',
        storage_classes: {
          STANDARD: 856,
          STANDARD_IA: 284,
          GLACIER: 107
        },
        recent_activity: {
          uploads_today: 23,
          downloads_today: 156,
          deletes_today: 8
        },
        cost_estimate: '$42.50/month'
      })
    }
  }
}

// Get the API (either real Wails API or mock)
const api = isDesktop ? window.go : mockAPI

export const useAppStore = defineStore('app', () => {
  // State
  const appInfo = ref(null)
  const regions = ref([])
  const currentRegion = ref('us-east-1')
  const buckets = ref([])
  const currentBucket = ref(null)
  const objects = ref([])
  const isLoading = ref(false)
  const error = ref(null)

  // Computed
  const isConnected = computed(() => appInfo.value !== null)
  const bucketsCount = computed(() => buckets.value.length)
  const totalSize = computed(() => {
    return buckets.value.reduce((total, bucket) => total + (bucket.size || 0), 0)
  })

  // Actions
  const initialize = async () => {
    try {
      isLoading.value = true
      error.value = null

      // Load app info and regions in parallel
      const [info, regionList] = await Promise.all([
        api.desktop.App.GetAppInfo(),
        api.desktop.App.ListRegions()
      ])

      appInfo.value = info
      regions.value = regionList

      // Load initial data
      await loadBuckets()
    } catch (err) {
      error.value = err.message || 'Failed to initialize application'
      throw err
    } finally {
      isLoading.value = false
    }
  }

  const setRegion = async (region) => {
    try {
      await api.desktop.App.SetRegion(region)
      currentRegion.value = region
      // Reload buckets for new region
      await loadBuckets()
    } catch (err) {
      error.value = err.message || 'Failed to set region'
      throw err
    }
  }

  const loadBuckets = async () => {
    try {
      isLoading.value = true
      const bucketList = await api.desktop.App.ListBuckets()
      buckets.value = bucketList
    } catch (err) {
      error.value = err.message || 'Failed to load buckets'
      throw err
    } finally {
      isLoading.value = false
    }
  }

  const createBucket = async (name, region = currentRegion.value) => {
    try {
      await api.desktop.App.CreateBucket(name, region)
      await loadBuckets() // Refresh bucket list
    } catch (err) {
      error.value = err.message || 'Failed to create bucket'
      throw err
    }
  }

  const deleteBucket = async (name) => {
    try {
      await api.desktop.App.DeleteBucket(name)
      await loadBuckets() // Refresh bucket list
    } catch (err) {
      error.value = err.message || 'Failed to delete bucket'
      throw err
    }
  }

  const loadObjects = async (bucketName, prefix = '') => {
    try {
      isLoading.value = true
      currentBucket.value = bucketName
      const objectList = await api.desktop.App.ListObjects(bucketName, prefix)
      objects.value = objectList
    } catch (err) {
      error.value = err.message || 'Failed to load objects'
      throw err
    } finally {
      isLoading.value = false
    }
  }

  const uploadObject = async (bucketName, key, filePath) => {
    try {
      await api.desktop.App.UploadObject(bucketName, key, filePath)
      if (currentBucket.value === bucketName) {
        await loadObjects(bucketName) // Refresh object list
      }
    } catch (err) {
      error.value = err.message || 'Failed to upload object'
      throw err
    }
  }

  const downloadObject = async (bucketName, key, targetPath) => {
    try {
      await api.desktop.App.DownloadObject(bucketName, key, targetPath)
    } catch (err) {
      error.value = err.message || 'Failed to download object'
      throw err
    }
  }

  const deleteObject = async (bucketName, key) => {
    try {
      await api.desktop.App.DeleteObject(bucketName, key)
      if (currentBucket.value === bucketName) {
        await loadObjects(bucketName) // Refresh object list
      }
    } catch (err) {
      error.value = err.message || 'Failed to delete object'
      throw err
    }
  }

  const searchObjects = async (bucketName, pattern) => {
    try {
      isLoading.value = true
      const results = await api.desktop.App.SearchObjects(bucketName, pattern)
      objects.value = results
    } catch (err) {
      error.value = err.message || 'Failed to search objects'
      throw err
    } finally {
      isLoading.value = false
    }
  }

  const getBucketAnalytics = async (bucketName) => {
    try {
      return await api.desktop.App.GetBucketAnalytics(bucketName)
    } catch (err) {
      error.value = err.message || 'Failed to get bucket analytics'
      throw err
    }
  }

  const getObjectMetadata = async (bucketName, key) => {
    try {
      return await api.desktop.App.GetObjectMetadata(bucketName, key)
    } catch (err) {
      error.value = err.message || 'Failed to get object metadata'
      throw err
    }
  }

  const refresh = async () => {
    try {
      if (currentBucket.value) {
        await loadObjects(currentBucket.value)
      } else {
        await loadBuckets()
      }
    } catch (err) {
      error.value = err.message || 'Failed to refresh'
      throw err
    }
  }

  const clearError = () => {
    error.value = null
  }

  // Format utilities
  const formatBytes = (bytes) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  const formatDate = (dateString) => {
    return new Date(dateString).toLocaleString()
  }

  return {
    // State
    appInfo,
    regions,
    currentRegion,
    buckets,
    currentBucket,
    objects,
    isLoading,
    error,

    // Computed
    isConnected,
    bucketsCount,
    totalSize,

    // Actions
    initialize,
    setRegion,
    loadBuckets,
    createBucket,
    deleteBucket,
    loadObjects,
    uploadObject,
    downloadObject,
    deleteObject,
    searchObjects,
    getBucketAnalytics,
    getObjectMetadata,
    refresh,
    clearError,

    // Utilities
    formatBytes,
    formatDate
  }
})