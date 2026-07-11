import { createI18n } from 'vue-i18n'

export type AppLocale = 'zh-CN' | 'en-US'

const messages = {
  'zh-CN': {
    app: { tagline: '服务器集群智能管控平台' },
    auth: {
      username: '用户名', password: '密码', showPassword: '显示', hidePassword: '隐藏',
      signIn: '登 录', signingIn: '正在登录', defaultAccount: '默认账户',
      missingCredentials: '请输入用户名和密码',
    },
    nav: {
      dashboard: '看板总览', nodes: '节点管理', deploy: '批量部署', docker: 'Docker 管理',
      clusters: 'RKE2 集群', terminal: 'Web 终端', sftp: '文件管理', settings: '系统设置',
      automation: '自动化', sshKeys: '密钥管理', compare: '配置对比', audit: '审计日志',
      nodeDetail: '节点详情',
    },
    common: {
      online: '在线', offline: '离线', total: '总数', all: '全部', untrusted: '未授信',
      error: '异常', refresh: '刷新', changeLanguage: '切换语言', changeTheme: '切换主题', collapseSidebar: '折叠侧边栏', settings: '系统设置',
      signOut: '退出登录', cancel: '取消', confirm: '确定', load: '负载', collecting: '采集中...',
      noNodes: '暂无节点', nodeStatus: '节点状态', memory: '内存', disk: '磁盘',
      networkTraffic: '网络流量', nodeCount: '节点', onlineRate: '在线率', cpuCores: 'CPU核数',
      averageCpu: '平均CPU', physicalMachine: '物理机',
    },
    roles: { admin: '管理员', operator: '操作员', viewer: '观察者' },
    confirm: { signOut: '确定要退出登录吗？' },
  },
  'en-US': {
    app: { tagline: 'Intelligent server fleet control plane' },
    auth: {
      username: 'Username', password: 'Password', showPassword: 'Show', hidePassword: 'Hide',
      signIn: 'Sign in', signingIn: 'Signing in', defaultAccount: 'Default account',
      missingCredentials: 'Enter a username and password',
    },
    nav: {
      dashboard: 'Dashboard', nodes: 'Nodes', deploy: 'Batch deploy', docker: 'Docker',
      clusters: 'RKE2 clusters', terminal: 'Web terminal', sftp: 'Files', settings: 'Settings',
      automation: 'Automation', sshKeys: 'SSH keys', compare: 'Configuration compare', audit: 'Audit log',
      nodeDetail: 'Node detail',
    },
    common: {
      online: 'Online', offline: 'Offline', total: 'Total', all: 'All', untrusted: 'Untrusted',
      error: 'Error', refresh: 'Refresh', changeLanguage: 'Change language', changeTheme: 'Change theme', collapseSidebar: 'Collapse sidebar', settings: 'Settings',
      signOut: 'Sign out', cancel: 'Cancel', confirm: 'Confirm', load: 'Load', collecting: 'Collecting...',
      noNodes: 'No nodes yet', nodeStatus: 'Node status', memory: 'Memory', disk: 'Disk',
      networkTraffic: 'Network traffic', nodeCount: 'Nodes', onlineRate: 'Online rate', cpuCores: 'CPU cores',
      averageCpu: 'Average CPU', physicalMachine: 'Physical machine',
    },
    roles: { admin: 'Administrator', operator: 'Operator', viewer: 'Viewer' },
    confirm: { signOut: 'Sign out of DeviceGrid?' },
  },
}

function initialLocale(): AppLocale {
  const saved = localStorage.getItem('dg_locale')
  if (saved === 'zh-CN' || saved === 'en-US') return saved
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale(),
  fallbackLocale: 'en-US',
  messages,
})

document.documentElement.lang = i18n.global.locale.value

export function setLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  localStorage.setItem('dg_locale', locale)
  document.documentElement.lang = locale
}
