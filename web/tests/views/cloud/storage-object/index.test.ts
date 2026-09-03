import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { ElNotification } from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'

import { appI18n } from '@/i18n'
import { AppDialog } from '@/components/AppDialog'
import { AppTable } from '@/components/AppTable'
import { AppSearch } from '@/components/AppSearch'
import { usePermissionStore } from '@/store/permission'
import ObjectStorage from '@/views/cloud/storage-object/index.vue'
import {
  createCosConfig,
  listCosConfigs,
  testCosConfig,
  updateCosConfig,
} from '@/api/storage/cosconfig'
import {
  createUploadRule,
  getUploadRulePageInit,
  listUploadRules,
  updateUploadRule,
} from '@/api/storage/uploadrule'

vi.mock('@/api/storage/cosconfig', () => ({
  listCosConfigs: vi.fn(),
  getCosConfig: vi.fn(),
  createCosConfig: vi.fn(),
  updateCosConfig: vi.fn(),
  updateCosConfigStatus: vi.fn(),
  testCosConfig: vi.fn(),
  deleteCosConfig: vi.fn(),
}))

vi.mock('@/api/storage/uploadrule', () => ({
  listUploadRules: vi.fn(),
  getUploadRule: vi.fn(),
  getUploadRulePageInit: vi.fn(),
  createUploadRule: vi.fn(),
  updateUploadRule: vi.fn(),
  updateUploadRuleStatus: vi.fn(),
  deleteUploadRule: vi.fn(),
}))

function mountPage(permissions: string[] = ['storage:object:list']): VueWrapper {
  const pinia = createPinia()
  setActivePinia(pinia)
  usePermissionStore().permissionCodes = permissions
  const wrapper = mount(ObjectStorage, {
    attachTo: document.body,
    global: { plugins: [ElementPlus, appI18n, pinia] },
  })
  wrappers.push(wrapper)
  return wrapper
}

const wrappers: VueWrapper[] = []

describe('ObjectStorage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(listCosConfigs).mockResolvedValue({
      list: [],
      total: 0,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(listUploadRules).mockResolvedValue({
      list: [],
      total: 0,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(getUploadRulePageInit).mockResolvedValue({
      platforms: [],
      configs: [],
    })
    vi.mocked(createCosConfig).mockResolvedValue({ id: 1 })
    vi.mocked(createUploadRule).mockResolvedValue({ id: 1 })
    vi.mocked(updateCosConfig).mockResolvedValue({})
    vi.mocked(updateUploadRule).mockResolvedValue({})
  })

  afterEach(() => {
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  })

  it('uses the shared management-page components and loads only COS configs initially', async () => {
    const wrapper = mountPage()
    await flushPromises()

    expect(wrapper.findAllComponents({ name: 'ElTabPane' })).toHaveLength(2)
    expect(wrapper.findComponent(AppSearch).exists()).toBe(true)
    expect(wrapper.findComponent(AppTable).exists()).toBe(true)
    expect(wrapper.findAllComponents(AppDialog)).toHaveLength(2)
    expect(listCosConfigs).toHaveBeenCalledOnce()
    expect(listCosConfigs).toHaveBeenLastCalledWith({ page: 1, pageSize: 20 })
    expect(listUploadRules).not.toHaveBeenCalled()
  })

  it('keeps add-rule disabled when page-init has no platform or COS config', async () => {
    const wrapper = mountPage(['storage:object:list', 'storage:upload-rule:create'])
    await flushPromises()
    await wrapper.findAll('.el-tabs__item')[1]?.trigger('click')
    await flushPromises()

    const addButton = wrapper.find('[data-testid="storage-add-rule"]')
    expect(addButton.exists()).toBe(true)
    expect(addButton.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('请先创建并启用 COS 配置')
    expect(wrapper.find('[data-testid="storage-rule-form"]').exists()).toBe(false)
  })

  it('opens the upload-rule AppDialog with platform and global COS config options', async () => {
    vi.mocked(getUploadRulePageInit).mockResolvedValue({
      platforms: [{ id: 1, code: 'admin', name: 'Admin', isEnabled: 1 }],
      configs: [
        {
          id: 8,
          name: '默认 COS',
          bucket: 'admin-assets',
          region: 'ap-guangzhou',
          isEnabled: 1,
        },
      ],
    })
    const wrapper = mountPage(['storage:object:list', 'storage:upload-rule:create'])
    await flushPromises()
    await wrapper.findAll('.el-tabs__item')[1]?.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="storage-add-rule"]').trigger('click')
    await flushPromises()

    expect(wrapper.findAllComponents(AppDialog)[1]?.props('modelValue')).toBe(true)
    const form = document.querySelector('[data-testid="storage-rule-form"]')
    expect(form?.textContent).toContain('认证平台')
    expect(form?.textContent).toContain('COS 配置')
    expect(document.querySelector('[data-testid="storage-rule-platform"]')).not.toBeNull()
    expect(document.querySelector('[data-testid="storage-rule-config"]')).not.toBeNull()
  })

  it('uses required business fields and creatable multi-selects in the upload-rule form', async () => {
    vi.mocked(getUploadRulePageInit).mockResolvedValue({
      platforms: [{ id: 1, code: 'admin', name: 'Admin', isEnabled: 1 }],
      configs: [
        { id: 8, name: '默认 COS', bucket: 'admin-assets', region: 'ap-guangzhou', isEnabled: 1 },
      ],
    })
    const wrapper = mountPage(['storage:object:list', 'storage:upload-rule:create'])
    await flushPromises()
    await wrapper.findAll('.el-tabs__item')[1]?.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="storage-add-rule"]').trigger('click')
    await flushPromises()

    const form = wrapper.find('[data-testid="storage-rule-form"]')
    expect(form.findAll('.el-form-item.is-required')).toHaveLength(7)
    expect(form.find('[data-testid="storage-rule-codes"]').attributes('placeholder')).toContain(
      '输入后按回车添加',
    )
    expect(form.find('[data-testid="storage-rule-extensions"]').classes()).toContain('el-select')
    expect(form.find('[data-testid="storage-rule-mime-types"]').classes()).toContain('el-select')
    expect(form.text()).toContain('可选择常用值，也可以直接输入自定义值')
  })

  it('shows the file size in MB and converts it back to bytes when creating', async () => {
    vi.mocked(getUploadRulePageInit).mockResolvedValue({
      platforms: [{ id: 1, code: 'admin', name: 'Admin', isEnabled: 1 }],
      configs: [
        { id: 8, name: '默认 COS', bucket: 'admin-assets', region: 'ap-guangzhou', isEnabled: 1 },
      ],
    })
    const wrapper = mountPage(['storage:object:list', 'storage:upload-rule:create'])
    await flushPromises()
    await wrapper.findAll('.el-tabs__item')[1]?.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="storage-add-rule"]').trigger('click')
    await flushPromises()

    const form = wrapper.find('[data-testid="storage-rule-form"]')
    const sizeInput = form
      .findAllComponents({ name: 'ElInputNumber' })
      .find(
        (item: VueWrapper) => item.attributes('data-testid') === 'storage-rule-max-file-size-mb',
      )
    expect(form.text()).toContain('单文件大小上限（MB）')
    expect(sizeInput?.props('modelValue')).toBe(1)

    sizeInput?.vm.$emit('update:modelValue', 2)
    form.findComponent({ name: 'ElInputTag' }).vm.$emit('update:modelValue', ['avatar'])
    await form.find('[data-testid="storage-rule-name"]').setValue('头像上传')
    const extensionSelect = form
      .findAllComponents({ name: 'ElSelect' })
      .find((item: VueWrapper) => item.attributes('data-testid') === 'storage-rule-extensions')
    extensionSelect?.vm.$emit('update:modelValue', ['png'])
    await wrapper
      .findAllComponents(AppDialog)[1]
      ?.find('.el-dialog__footer .el-button--primary')
      .trigger('click')
    await flushPromises()

    expect(createUploadRule).toHaveBeenCalledWith(
      expect.objectContaining({ maxFileSizeBytes: 2_097_152 }),
    )
  })

  it('selects all built-in extensions and MIME types from the dropdown headers', async () => {
    vi.mocked(getUploadRulePageInit).mockResolvedValue({
      platforms: [{ id: 1, code: 'admin', name: 'Admin', isEnabled: 1 }],
      configs: [
        { id: 8, name: '默认 COS', bucket: 'admin-assets', region: 'ap-guangzhou', isEnabled: 1 },
      ],
    })
    const wrapper = mountPage(['storage:object:list', 'storage:upload-rule:create'])
    await flushPromises()
    await wrapper.findAll('.el-tabs__item')[1]?.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="storage-add-rule"]').trigger('click')
    await flushPromises()

    const form = wrapper.find('[data-testid="storage-rule-form"]')
    const extensionSelect = form
      .findAllComponents({ name: 'ElSelect' })
      .find((item: VueWrapper) => item.attributes('data-testid') === 'storage-rule-extensions')
    const mimeSelect = form
      .findAllComponents({ name: 'ElSelect' })
      .find((item: VueWrapper) => item.attributes('data-testid') === 'storage-rule-mime-types')
    await extensionSelect?.find('.el-select__wrapper').trigger('click')
    await flushPromises()
    const extensionSelectAll = document.querySelector<HTMLElement>(
      '[data-testid="storage-rule-extensions-select-all"]',
    )
    expect(extensionSelectAll).not.toBeNull()
    extensionSelectAll?.click()
    await mimeSelect?.find('.el-select__wrapper').trigger('click')
    await flushPromises()
    const mimeSelectAll = document.querySelector<HTMLElement>(
      '[data-testid="storage-rule-mime-types-select-all"]',
    )
    expect(mimeSelectAll).not.toBeNull()
    mimeSelectAll?.click()
    await flushPromises()

    expect(extensionSelect?.props('modelValue')).toEqual([
      'jpg',
      'jpeg',
      'png',
      'gif',
      'webp',
      'pdf',
      'doc',
      'docx',
      'xls',
      'xlsx',
      'zip',
    ])
    expect(mimeSelect?.props('modelValue')).toEqual([
      'image/jpeg',
      'image/png',
      'image/gif',
      'image/webp',
      'application/pdf',
      'application/zip',
    ])
  })

  it('rejects an upload rule without allowed extensions before creating', async () => {
    vi.mocked(getUploadRulePageInit).mockResolvedValue({
      platforms: [{ id: 1, code: 'admin', name: 'Admin', isEnabled: 1 }],
      configs: [
        { id: 8, name: '默认 COS', bucket: 'admin-assets', region: 'ap-guangzhou', isEnabled: 1 },
      ],
    })
    const wrapper = mountPage(['storage:object:list', 'storage:upload-rule:create'])
    await flushPromises()
    await wrapper.findAll('.el-tabs__item')[1]?.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="storage-add-rule"]').trigger('click')
    await flushPromises()

    const form = wrapper.find('[data-testid="storage-rule-form"]')
    form.findComponent({ name: 'ElInputTag' }).vm.$emit('update:modelValue', ['avatar'])
    await form.find('[data-testid="storage-rule-name"]').setValue('头像上传')
    await wrapper
      .findAllComponents(AppDialog)[1]
      ?.find('.el-dialog__footer .el-button--primary')
      .trigger('click')
    await flushPromises()

    expect(createUploadRule).not.toHaveBeenCalled()
    const extensionItem = wrapper
      .findAllComponents({ name: 'ElFormItem' })
      .find((item) => item.props('label') === '允许扩展名')
    expect(extensionItem?.text()).toContain('请至少选择或输入一个允许扩展名')
  })

  it('shows the public-access warning when public mode is selected', async () => {
    vi.mocked(getUploadRulePageInit).mockResolvedValue({
      platforms: [{ id: 1, code: 'admin', name: 'Admin', isEnabled: 1 }],
      configs: [
        { id: 8, name: '默认 COS', bucket: 'admin-assets', region: 'ap-guangzhou', isEnabled: 1 },
      ],
    })
    const wrapper = mountPage(['storage:object:list', 'storage:upload-rule:create'])
    await flushPromises()
    await wrapper.findAll('.el-tabs__item')[1]?.trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="storage-add-rule"]').trigger('click')
    await flushPromises()

    const publicRadio = wrapper
      .findAllComponents({ name: 'ElRadio' })
      .find((item) => item.props('value') === 'public')
    await publicRadio?.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="storage-public-warning"]').text()).toContain(
      '任何获得链接的人都可以访问',
    )
  })

  it('does not emit a second error notification when connection testing fails', async () => {
    vi.mocked(listCosConfigs).mockResolvedValue({
      list: [
        {
          id: 7,
          name: '主配置',
          appId: '1250000000',
          bucket: 'admin-assets',
          region: 'ap-guangzhou',
          endpoint: null,
          bucketDomain: null,
          isEnabled: 1,
          hasCredentials: true,
          remark: '',
          createdAt: '2026-08-30T00:00:00Z',
          updatedAt: '2026-08-30T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    vi.mocked(testCosConfig).mockRejectedValue(new Error('连接失败'))
    const errorSpy = vi.spyOn(ElNotification, 'error')
    const wrapper = mountPage(['storage:object:list', 'storage:cos-config:test'])
    await flushPromises()

    const testButton = wrapper
      .findAll('.el-table__body .el-button')
      .find((button) => button.text() === '测试连接')
    await testButton?.trigger('click')
    await flushPromises()

    expect(errorSpy).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('连接失败')
  })

  it('keeps the COS config dialog global without a platform field', async () => {
    const wrapper = mountPage(['storage:object:list', 'storage:cos-config:create'])
    await flushPromises()
    await wrapper.find('[data-testid="storage-add-config"]').trigger('click')
    await flushPromises()

    expect(wrapper.findAllComponents(AppDialog)[0]?.props('modelValue')).toBe(true)
    const form = document.querySelector('[data-testid="storage-config-form"]')
    expect(form).not.toBeNull()
    expect(form?.textContent).not.toContain('认证平台')
  })

  it('marks required COS fields and defaults the region selector to Guangzhou', async () => {
    const wrapper = mountPage(['storage:object:list', 'storage:cos-config:create'])
    await flushPromises()
    await wrapper.find('[data-testid="storage-add-config"]').trigger('click')
    await flushPromises()

    const form = wrapper.find('[data-testid="storage-config-form"]')
    expect(form.findAll('.el-form-item.is-required')).toHaveLength(6)
    expect(form.find('[data-testid="storage-config-region"]').exists()).toBe(true)
    expect(form.find('[data-testid="storage-config-region"]').text()).toContain('广州')
    expect(form.find('[data-testid="storage-config-endpoint"]').attributes('placeholder')).toBe(
      '例如：https://cos.ap-guangzhou.myqcloud.com',
    )
    expect(form.find('[data-testid="storage-config-domain"]').attributes('placeholder')).toBe(
      '例如：https://cdn.example.com',
    )
  })

  it('rejects a COS domain without an HTTPS scheme before creating', async () => {
    const wrapper = mountPage(['storage:object:list', 'storage:cos-config:create'])
    await flushPromises()
    await wrapper.find('[data-testid="storage-add-config"]').trigger('click')
    await flushPromises()

    const form = wrapper.find('[data-testid="storage-config-form"]')
    await form.find('[data-testid="storage-config-name"]').setValue('腾讯云 COS')
    await form.find('[data-testid="storage-config-app-id"]').setValue('1314542588')
    await form.find('[data-testid="storage-config-secret-id"]').setValue('secret-id')
    await form.find('[data-testid="storage-config-secret-key"]').setValue('secret-key')
    await form.find('[data-testid="storage-config-bucket"]').setValue('zgm')
    await form.find('[data-testid="storage-config-domain"]').setValue('cos.zgm2003.cn')
    await wrapper
      .findAllComponents(AppDialog)[0]
      ?.find('.el-dialog__footer .el-button--primary')
      .trigger('click')
    await flushPromises()

    expect(createCosConfig).not.toHaveBeenCalled()
    const domainItem = wrapper
      .findAllComponents({ name: 'ElFormItem' })
      .find((item) => item.props('label') === '访问域名')
    expect(domainItem?.props('error')).toBe('请输入以 https:// 开头的完整访问域名')
  })

  it('sends COS keyword and status through the real list query', async () => {
    const wrapper = mountPage()
    await flushPromises()

    const search = wrapper.findComponent(AppSearch)
    search.vm.$emit('update:modelValue', { keyword: '  主配置  ', status: 1 })
    await search.vm.$emit('query')
    await flushPromises()

    expect(listCosConfigs).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: '主配置',
      isEnabled: 1,
    })
  })

  it('omits status and empty secrets from the COS update payload', async () => {
    vi.mocked(listCosConfigs).mockResolvedValue({
      list: [
        {
          id: 7,
          name: '主配置',
          appId: '1250000000',
          bucket: 'admin-assets',
          region: 'ap-guangzhou',
          endpoint: null,
          bucketDomain: null,
          isEnabled: 1,
          hasCredentials: true,
          remark: '',
          createdAt: '2026-08-30T00:00:00Z',
          updatedAt: '2026-08-30T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage(['storage:object:list', 'storage:cos-config:update'])
    await flushPromises()
    await wrapper.find('.el-table__body .el-button').trigger('click')
    await flushPromises()
    const dialog = wrapper.findAllComponents(AppDialog)[0]
    await dialog?.find('.el-dialog__footer .el-button--primary').trigger('click')
    await flushPromises()

    expect(updateCosConfig).toHaveBeenCalledWith(7, {
      name: '主配置',
      appId: '1250000000',
      bucket: 'admin-assets',
      region: 'ap-guangzhou',
      endpoint: null,
      bucketDomain: null,
      remark: '',
    })
  })

  it('omits immutable create fields from the upload-rule update payload', async () => {
    vi.mocked(getUploadRulePageInit).mockResolvedValue({
      platforms: [{ id: 1, code: 'admin', name: 'Admin', isEnabled: 1 }],
      configs: [
        { id: 8, name: '默认 COS', bucket: 'admin-assets', region: 'ap-guangzhou', isEnabled: 1 },
      ],
    })
    vi.mocked(listUploadRules).mockResolvedValue({
      list: [
        {
          id: 9,
          platformId: 1,
          platformCode: 'admin',
          platformName: 'Admin',
          codes: ['avatar', 'article-cover'],
          name: '头像上传',
          cosConfigId: 8,
          cosConfigName: '默认 COS',
          maxFileSizeBytes: 1048576,
          allowedExtensions: ['png'],
          allowedMimeTypes: ['image/png'],
          accessMode: 'private',
          isEnabled: 1,
          remark: '',
          createdAt: '2026-08-30T00:00:00Z',
          updatedAt: '2026-08-30T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    })
    const wrapper = mountPage(['storage:object:list', 'storage:upload-rule:update'])
    await flushPromises()
    await wrapper.findAll('.el-tabs__item')[1]?.trigger('click')
    await flushPromises()
    await wrapper.find('.el-table__body .el-button').trigger('click')
    await flushPromises()
    const dialog = wrapper.findAllComponents(AppDialog)[1]
    await dialog?.find('.el-dialog__footer .el-button--primary').trigger('click')
    await flushPromises()

    expect(updateUploadRule).toHaveBeenCalledWith(9, {
      name: '头像上传',
      cosConfigId: 8,
      maxFileSizeBytes: 1048576,
      allowedExtensions: ['png'],
      allowedMimeTypes: ['image/png'],
      accessMode: 'private',
      remark: '',
    })
  })
})
