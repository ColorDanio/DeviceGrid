import { describe, expect, it } from 'vitest'
import { i18n, setLocale } from './index'

describe('Docker locale catalog', () => {
  it('provides Docker workflow labels in both supported locales', () => {
    setLocale('zh-CN')
    expect(i18n.global.t('feature.dockerPullImage')).toBe('拉取镜像')
    expect(i18n.global.t('feature.dockerContainerTerminal', { name: 'api' })).toBe('容器终端: api')
    expect(i18n.global.t('feature.clusterCreate')).toBe('创建集群')
    expect(i18n.global.t('feature.nodesBatchTrust')).toBe('批量授信')
    expect(i18n.global.t('feature.detailLiveMetrics')).toBe('实时监控')

    setLocale('en-US')
    expect(i18n.global.t('feature.dockerPullImage')).toBe('Pull image')
    expect(i18n.global.t('feature.dockerContainerTerminal', { name: 'api' })).toBe('Container terminal: api')
    expect(i18n.global.t('feature.clusterCreate')).toBe('Create cluster')
    expect(i18n.global.t('feature.nodesBatchTrust')).toBe('Batch trust')
    expect(i18n.global.t('feature.detailLiveMetrics')).toBe('Live metrics')
  })
})
