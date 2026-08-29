import { request } from '../../utils/request'
import { isYesNo, type YesNo } from '../../enums/yes-no'
import type { PageRequest, PageResult } from '../../types/pagination'
import { ProtocolError } from '../../types/http'

export interface CosConfig { id:number; name:string; appId:string; bucket:string; region:string; endpoint:string|null; bucketDomain:string|null; isEnabled:YesNo; hasCredentials:boolean; remark:string; createdAt:string; updatedAt:string }
export interface CosConfigQuery extends PageRequest { keyword?:string; isEnabled?:YesNo }
export interface CreateCosConfigInput { name:string; appId:string; secretId:string; secretKey:string; bucket:string; region:string; endpoint?:string|null; bucketDomain?:string|null; isEnabled:YesNo; remark:string }
export type UpdateCosConfigInput = Omit<CreateCosConfigInput,'secretId'|'secretKey'|'isEnabled'> & { secretId?:string; secretKey?:string }
const configKeys=['id','name','appId','bucket','region','endpoint','bucketDomain','isEnabled','hasCredentials','remark','createdAt','updatedAt'] as const
function record(v:unknown):Record<string,unknown>{if(typeof v!=='object'||v===null||Array.isArray(v))throw new ProtocolError('COS config response is invalid');return v as Record<string,unknown>}
function exact(v:Record<string,unknown>,keys:readonly string[]):void{const actual=Object.keys(v).sort(),expected=[...keys].sort();if(actual.length!==expected.length||actual.some((k,i)=>k!==expected[i]))throw new ProtocolError('COS config response fields are invalid')}
function parseConfig(v:unknown):CosConfig{const r=record(v);exact(r,configKeys);if(!Number.isInteger(r.id)||Number(r.id)<=0||typeof r.name!=='string'||typeof r.appId!=='string'||typeof r.bucket!=='string'||typeof r.region!=='string'||!(r.endpoint===null||typeof r.endpoint==='string')||!(r.bucketDomain===null||typeof r.bucketDomain==='string')||!isYesNo(r.isEnabled)||typeof r.hasCredentials!=='boolean'||typeof r.remark!=='string'||typeof r.createdAt!=='string'||typeof r.updatedAt!=='string')throw new ProtocolError('COS config response values are invalid');return r as unknown as CosConfig}
export function parseCosConfigResponse(v:unknown):CosConfig{return parseConfig(v)}
export async function listCosConfigs(query:CosConfigQuery):Promise<PageResult<CosConfig>>{return request<PageResult<CosConfig>>({method:'GET',url:'/api/admin/v1/storage/cos-configs',params:query})}
export async function getCosConfig(id:number):Promise<CosConfig>{return request<CosConfig>({method:'GET',url:`/api/admin/v1/storage/cos-configs/${id}`})}
export async function createCosConfig(data:CreateCosConfigInput):Promise<{id:number}>{return request<{id:number}>({method:'POST',url:'/api/admin/v1/storage/cos-configs',data})}
export async function updateCosConfig(id:number,data:UpdateCosConfigInput):Promise<Record<string,never>>{return request<Record<string,never>>({method:'PUT',url:`/api/admin/v1/storage/cos-configs/${id}`,data})}
export async function updateCosConfigStatus(id:number,isEnabled:YesNo):Promise<{id:number;isEnabled:YesNo}>{return request({method:'PATCH',url:`/api/admin/v1/storage/cos-configs/${id}/status`,data:{isEnabled}})}
export async function testCosConfig(id:number):Promise<Record<string,never>>{return request({method:'POST',url:`/api/admin/v1/storage/cos-configs/${id}/test`})}
export async function deleteCosConfig(id:number):Promise<Record<string,never>>{return request({method:'DELETE',url:`/api/admin/v1/storage/cos-configs/${id}`})}
