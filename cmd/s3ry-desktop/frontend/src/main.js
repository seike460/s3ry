import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHashHistory } from 'vue-router'
import App from './App.vue'
import './style.css'

// Import views
import Dashboard from './views/Dashboard.vue'
import Buckets from './views/Buckets.vue'
import BucketDetail from './views/BucketDetail.vue'
import Settings from './views/Settings.vue'
import About from './views/About.vue'

// Router configuration
const routes = [
  { path: '/', name: 'Dashboard', component: Dashboard },
  { path: '/buckets', name: 'Buckets', component: Buckets },
  { path: '/buckets/:name', name: 'BucketDetail', component: BucketDetail, props: true },
  { path: '/settings', name: 'Settings', component: Settings },
  { path: '/about', name: 'About', component: About }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

// Pinia store
const pinia = createPinia()

// Create Vue app
const app = createApp(App)

app.use(pinia)
app.use(router)

// Global error handler
app.config.errorHandler = (err, vm, info) => {
  console.error('Vue Error:', err, info)
}

// Mount app
app.mount('#app')

// Hide loading screen once Vue is mounted
setTimeout(() => {
  const loading = document.getElementById('loading')
  if (loading) {
    loading.style.display = 'none'
  }
}, 1000)