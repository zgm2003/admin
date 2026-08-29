import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { appI18n } from '@src/i18n'
import { useAccessStore } from '@src/store/access'
import ObjectStorage from '@src/modules/storage/object/index.vue'
import { listCosConfigs } from '@src/api/storage/cosconfig'
import { listUploadRules, getUploadRulePageInit } from '@src/api/storage/uploadrule'
vi.mock('@src/api/storage/cosconfig',()=>({listCosConfigs:vi.fn(),createCosConfig:vi.fn(),updateCosConfig:vi.fn(),updateCosConfigStatus:vi.fn(),testCosConfig:vi.fn(),deleteCosConfig:vi.fn()}));vi.mock('@src/api/storage/uploadrule',()=>({listUploadRules:vi.fn(),getUploadRulePageInit:vi.fn(),createUploadRule:vi.fn(),updateUploadRule:vi.fn(),updateUploadRuleStatus:vi.fn(),deleteUploadRule:vi.fn()}))
describe('ObjectStorage',()=>{beforeEach(()=>{const pinia=createPinia();setActivePinia(pinia);useAccessStore().permissionCodes=['storage:object:list'];vi.mocked(listCosConfigs).mockResolvedValue({list:[],total:0,page:1,pageSize:20});vi.mocked(listUploadRules).mockResolvedValue({list:[],total:0,page:1,pageSize:20});vi.mocked(getUploadRulePageInit).mockResolvedValue({platforms:[],configs:[]})});it('renders exactly two tabs and loads only config initially',async()=>{const wrapper=mount(ObjectStorage,{global:{plugins:[ElementPlus,appI18n]}});await flushPromises();expect(wrapper.findAllComponents({name:'ElTabPane'})).toHaveLength(2);expect(listCosConfigs).toHaveBeenCalledOnce();expect(listUploadRules).not.toHaveBeenCalled()})})
