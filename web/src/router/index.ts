import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/kanban',
    children: [
      {
        path: 'kanban',
        name: 'Kanban',
        component: () => import('@/views/Kanban.vue'),
        meta: { title: 'nav.dashboard', icon: 'Monitor' },
      },
      {
        path: 'nodes',
        name: 'Nodes',
        component: () => import('@/views/Nodes.vue'),
        meta: { title: 'nav.nodes', icon: 'Cpu' },
      },
      {
        path: 'nodes/:id',
        name: 'NodeDetail',
        component: () => import('@/views/NodeDetail.vue'),
        meta: { title: 'nav.nodeDetail', hidden: true },
      },
      {
        path: 'deploy',
        name: 'Deploy',
        component: () => import('@/views/Deploy.vue'),
        meta: { title: 'nav.deploy', icon: 'Promotion' },
      },
      {
        path: 'docker',
        name: 'Docker',
        component: () => import('@/views/Docker.vue'),
        meta: { title: 'nav.docker', icon: 'Box' },
      },
      {
        path: 'clusters',
        name: 'Clusters',
        component: () => import('@/views/Clusters.vue'),
        meta: { title: 'nav.clusters', icon: 'Connection' },
      },
      {
        path: 'terminal',
        name: 'Terminal',
        component: () => import('@/views/Terminal.vue'),
        meta: { title: 'nav.terminal', icon: 'Terminal' },
      },
      {
        path: 'sftp',
        name: 'SFTP',
        component: () => import('@/views/SFTP.vue'),
        meta: { title: 'nav.sftp', icon: 'SFTP' },
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/Settings.vue'),
        meta: { title: 'nav.settings', icon: 'Setting' },
      },
      {
        path: 'automation',
        name: 'Automation',
        component: () => import('@/views/Automation.vue'),
        meta: { title: 'nav.automation', icon: 'AlarmClock' },
      },
      {
        path: 'ssh-keys',
        name: 'SSHKeys',
        component: () => import('@/views/SSHKeys.vue'),
        meta: { title: 'nav.sshKeys', icon: 'Key' },
      },
      {
        path: 'compare',
        name: 'Compare',
        component: () => import('@/views/Compare.vue'),
        meta: { title: 'nav.compare', icon: 'Switch' },
      },
      {
        path: 'audit',
        name: 'Audit',
        component: () => import('@/views/Audit.vue'),
        meta: { title: 'nav.audit', icon: 'Document' },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/kanban',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _from, next) => {
  const auth = useAuthStore()
  if (to.meta.public) {
    if (auth.isAuthenticated) {
      next('/kanban')
    } else {
      next()
    }
    return
  }
  if (!auth.isAuthenticated) {
    next('/login')
    return
  }
  next()
})

export default router
