<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title-group">
        <h2>RKE2 集群</h2>
        <p class="page-subtitle">管理 RKE2 Kubernetes 集群的全生命周期</p>
      </div>
      <button class="btn-primary">
        <svg viewBox="0 0 24 24" fill="none" width="16" height="16"><path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>
        创建集群
      </button>
    </div>

    <div class="dg-card empty-cluster">
      <div class="empty-illustration">
        <svg viewBox="0 0 120 120" fill="none" width="100" height="100">
          <circle cx="60" cy="60" r="20" stroke="currentColor" stroke-width="2" opacity="0.3"/>
          <circle cx="60" cy="20" r="12" stroke="currentColor" stroke-width="2" opacity="0.4"/>
          <circle cx="100" cy="60" r="12" stroke="currentColor" stroke-width="2" opacity="0.4"/>
          <circle cx="60" cy="100" r="12" stroke="currentColor" stroke-width="2" opacity="0.4"/>
          <circle cx="20" cy="60" r="12" stroke="currentColor" stroke-width="2" opacity="0.4"/>
          <path d="M60 32v8M60 72v8M88 60h-8M44 60h-8" stroke="currentColor" stroke-width="2" opacity="0.3"/>
        </svg>
      </div>
      <p style="font-size:16px;font-weight:600;color:var(--dg-text)">还没有创建任何集群</p>
      <p style="font-size:13px;color:var(--dg-text-tertiary);margin-top:4px">选择节点创建 RKE2 server/agent 集群，实现 Kubernetes 编排</p>
      <button class="btn-primary-outline" style="margin-top:20px">立即创建</button>
    </div>
  </div>
</template>

<style scoped lang="scss">
.btn-primary {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 18px;
  border: none;
  border-radius: var(--dg-radius-sm);
  background: var(--dg-primary);
  color: #fff;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  font-family: inherit;
  &:hover { background: var(--dg-primary-dark); }
}

.btn-primary-outline {
  padding: 10px 24px;
  border: 1.5px solid var(--dg-primary);
  border-radius: var(--dg-radius-sm);
  background: transparent;
  color: var(--dg-primary);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  font-family: inherit;
  transition: all 0.2s;
  &:hover { background: var(--dg-info-bg); }
}

.empty-cluster {
  min-height: 450px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;

  .empty-illustration { margin-bottom: 20px; }
}
</style>
