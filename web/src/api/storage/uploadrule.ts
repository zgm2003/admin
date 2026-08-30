import { request } from '../../utils/request'
import type { YesNo } from '../../enums/yes-no'
import type { PageRequest, PageResult } from '../../types/pagination'
export interface UploadRule { id:number; platformId:number; platformCode:string; platformName:string; code:string; name:string; cosConfigId:number; cosConfigName:string; pathPrefix:string; maxFileSizeBytes:number; maxFileCount:number; allowedExtensions:string[]; allowedMimeTypes:string[]; accessMode:'private'|'public'; isEnabled:YesNo; remark:string; createdAt:string; updatedAt:string }
export interface UploadRuleQuery extends PageRequest { platformId?:number; cosConfigId?:number; keyword?:string; isEnabled?:YesNo }
export interface UploadRuleInput { platformId?:number; code?:string; name:string; cosConfigId:number; pathPrefix:string; maxFileSizeBytes:number; maxFileCount:number; allowedExtensions:string[]; allowedMimeTypes:string[]; accessMode:'private'|'public'; isEnabled?:YesNo; remark:string }
export interface PlatformOption {id:number;code:string;name:string;isEnabled:YesNo};export interface ConfigSummary {id:number;name:string;bucket:string;region:string;isEnabled:YesNo};export interface UploadRulePageInit {platforms:PlatformOption[];configs:ConfigSummary[]}
export async function listUploadRules(query:UploadRuleQuery):Promise<PageResult<UploadRule>>{return request({method:'GET',url:'/api/admin/v1/storage/upload-rules',params:query})}
export async function getUploadRule(id:number):Promise<UploadRule>{return request({method:'GET',url:`/api/admin/v1/storage/upload-rules/${id}`})}
export async function getUploadRulePageInit():Promise<UploadRulePageInit>{const result=await request<{platforms:PlatformOption[]|null;configs:ConfigSummary[]|null}>({method:'GET',url:'/api/admin/v1/storage/upload-rules/page-init'});return {platforms:result.platforms??[],configs:result.configs??[]}}
export async function createUploadRule(data:UploadRuleInput):Promise<{id:number}>{return request({method:'POST',url:'/api/admin/v1/storage/upload-rules',data})}
export async function updateUploadRule(id:number,data:UploadRuleInput):Promise<Record<string,never>>{return request({method:'PUT',url:`/api/admin/v1/storage/upload-rules/${id}`,data})}
export async function updateUploadRuleStatus(id:number,isEnabled:YesNo):Promise<{id:number;isEnabled:YesNo}>{return request({method:'PATCH',url:`/api/admin/v1/storage/upload-rules/${id}/status`,data:{isEnabled}})}
export async function deleteUploadRule(id:number):Promise<Record<string,never>>{return request({method:'DELETE',url:`/api/admin/v1/storage/upload-rules/${id}`})}
