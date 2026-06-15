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
        meta: { title: '看板总览', icon: 'Monitor' },
      },
      {
        path: 'nodes',
        name: 'Nodes',
        component: () => import('@/views/Nodes.vue'),
        meta: { title: '节点管理', icon: 'Cpu' },
      },
      {
        path: 'nodes/:id',
        name: 'NodeDetail',
        component: () => import('@/views/NodeDetail.vue'),
        meta: { title: '节点详情', hidden: true },
      },
      {
        path: 'deploy',
        name: 'Deploy',
        component: () => import('@/views/Deploy.vue'),
        meta: { title: '批量部署', icon: 'Promotion' },
      },
      {
        path: 'docker',
        name: 'Docker',
        component: () => import('@/views/Docker.vue'),
        meta: { title: 'Docker 管理', icon: 'Box' },
      },
      {
        path: 'clusters',
        name: 'Clusters',
        component: () => import('@/views/Clusters.vue'),
        meta: { title: 'RKE2 集群', icon: 'Connection' },
      },
      {
        path: 'terminal',
        name: 'Terminal',
        component: () => import('@/views/Terminal.vue'),
        meta: { title: 'Web 终端', icon: 'Terminal' },
      },
      {
        path: 'sftp',
        name: 'SFTP',
        component: () => import('@/views/SFTP.vue'),
        meta: { title: '文件管理', icon: 'SFTP' },
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/Settings.vue'),
        meta: { title: '系统设置', icon: 'Setting' },
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
