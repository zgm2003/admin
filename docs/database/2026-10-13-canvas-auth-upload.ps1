param([switch]$CheckOnly, [string]$RecoverFrom)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$serverRoot = Join-Path $repoRoot 'server'
$sqlPath = Join-Path $PSScriptRoot '2026-10-13-canvas-auth-upload.sql'
$auditRoot = Join-Path $env:LOCALAPPDATA ('Admin/maintenance/canvas-auth-upload/' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff'))
$helperRoot = Join-Path $serverRoot ('tmp/canvasAuthUpload-' + [guid]::NewGuid().ToString('N'))
$audit = [ordered]@{ migration='canvas-auth-upload'; checkOnly=[bool]$CheckOnly; startedAt=(Get-Date).ToString('o'); backupVerified=$false }
$oldOptions = $env:PGOPTIONS
$oldAppName = $env:PGAPPNAME

$helper = @'
package main

import (
 "context"
 "crypto/sha256"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "log/slog"
 "os"
 "path/filepath"
 "reflect"
 "strings"
 "time"

 "admin/server/internal/database"
 auth "admin/server/internal/module/auth/login"
 access "admin/server/internal/module/permission/access"
 "admin/server/internal/module/permission/role"
 state "admin/server/internal/module/permission/state"
 redis "admin/server/internal/redis"
 "github.com/joho/godotenv"
 goredis "github.com/redis/go-redis/v9"
 "gorm.io/gorm"
 "gorm.io/gorm/logger"
)

type platform struct { ID int64; Code string; MenuVersion,PolicyVersion int64 }
type facts struct {
 Platforms []platform `gorm:"-"`
 Versions []state.Version `gorm:"-"`
 PageID,ActionID int64
 PageShape,ActionShape,GrantCount,PageGrants int64
 RuleID,RuleCount int64
 Rule json.RawMessage `gorm:"-"`
 TargetDigest,UnrelatedDigest string
}
type audit struct {
 Before,After *facts
 DatabaseIdentity,RedisIdentity,SQLSHA256 string
 PostgresCommitted,RedisSynchronized,Verified bool
 FailureStage string
}

func main() {
 check:=flag.Bool("check",false,""); sqlPath:=flag.String("sql","",""); auditDir:=flag.String("audit","",""); recoverFrom:=flag.String("recover","",""); flag.Parse()
 report:=audit{}
 err:=run(*check,*sqlPath,*auditDir,*recoverFrom,&report)
 encoded,encodeErr:=json.MarshalIndent(report,"","  ")
 if encodeErr==nil { encodeErr=os.WriteFile(filepath.Join(*auditDir,"result.json"),encoded,0600) }
 if err!=nil || encodeErr!=nil {
  _=os.WriteFile(filepath.Join(*auditDir,"helper-errors.log"),[]byte(fmt.Sprint(errors.Join(err,encodeErr))),0600)
  fmt.Fprintln(os.Stderr,"Canvas 维护未完成；检查审计 result.json 和本地 helper-errors.log。")
  os.Exit(1)
 }
 fmt.Printf("Canvas 维护完成：postgresCommitted=%t redisSynchronized=%t verified=%t\n",report.PostgresCommitted,report.RedisSynchronized,report.Verified)
}

func run(check bool,sqlPath,auditDir,recoverFrom string,report *audit) (result error) {
 ctx,cancel:=context.WithTimeout(context.Background(),3*time.Minute); defer cancel()
 report.FailureStage="open"
 if err:=godotenv.Load(); err!=nil && !os.IsNotExist(err) { return err }
 pg,err:=database.Open(ctx,os.Getenv("POSTGRES_DSN")); if err!=nil{return err}; defer pg.Close()
 pg.GORM=pg.GORM.Session(&gorm.Session{Logger:logger.Default.LogMode(logger.Silent)})
 rc,err:=redis.Open(ctx,os.Getenv("REDIS_URL"));if err!=nil{return err}; defer rc.Close()
 if err=pg.GORM.WithContext(ctx).Raw(`SELECT current_database()||':'||current_schema()||':'||coalesce(inet_server_addr()::text,'local')||':'||coalesce(inet_server_port()::text,'local')`).Scan(&report.DatabaseIdentity).Error;err!=nil{return err}
 redisOptions,err:=goredis.ParseURL(os.Getenv("REDIS_URL"));if err!=nil{return err}
 serverInfo,err:=rc.UniversalClient().Info(ctx,"server").Result();if err!=nil{return err}
 runID:="";for _,line:=range strings.Split(serverInfo,"\n"){if strings.HasPrefix(line,"run_id:"){runID=strings.TrimSpace(strings.TrimPrefix(line,"run_id:"))}}
 if runID==""{return errors.New("Redis server identity unavailable")}
 report.RedisIdentity=fmt.Sprintf("%s/%d/%s",redisOptions.Addr,redisOptions.DB,runID)
 sql,err:=os.ReadFile(sqlPath);if err!=nil{return err};report.SQLSHA256=fmt.Sprintf("%x",sha256.Sum256(sql))
 initial,err:=inspect(ctx,pg.GORM);if err!=nil{return err};report.Before=&initial
 if recoverFrom!="" {return recoverRedis(ctx,pg.GORM,rc,recoverFrom,initial,report)}
 if check { report.FailureStage=""; fmt.Printf("Canvas page=%d action=%d rule=%d\n",initial.PageID,initial.ActionID,initial.RuleID);return nil }
 menuStore:=state.NewMenuStore(rc)
 userStore:=state.NewStore(rc)
 var menuLease *state.MenuLease;var userLease *state.MutationLease
 var stopMenu,stopUser func()
 committed:=false
 defer func(){
  if stopUser!=nil {stopUser()};if stopMenu!=nil {stopMenu()}
  if !committed {
   if userLease!=nil {result=errors.Join(result,userLease.Rollback(ctx))}
   if menuLease!=nil {result=errors.Join(result,menuLease.Rollback(ctx))}
  }
 }()
 report.FailureStage="transaction"
 var before,after facts
 err=pg.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB)error{
  if err:=tx.Exec(`SET LOCAL lock_timeout='5s'; SET LOCAL statement_timeout='60s'; LOCK TABLE permission_auth_platform,permission_menu,permission_role,permission_role_menu,permission_user_role,permission_access_version,user_account,storage_cos_config,storage_upload_rule,storage_upload_rule_code IN SHARE ROW EXCLUSIVE MODE`).Error;err!=nil{return err}
  before,err=inspect(ctx,tx);if err!=nil{return err};report.Before=&before
  cp,err:=canvasPlatform(before);if err!=nil{return err}
  current,err:=menuStore.Current(ctx,cp.ID,func(c context.Context,id int64)(int64,error){return cp.MenuVersion,nil});if err!=nil{return err}
  if current!=cp.MenuVersion{return errors.New("Canvas Redis menu version differs from PostgreSQL")}
  menuLease,err=menuStore.Acquire(ctx,cp.ID,cp.MenuVersion);if err!=nil{return err}
  menuCtx,stop:=menuLease.StartRenewal(ctx);stopMenu=stop
  candidates:=[]state.Version{}
  if before.GrantCount==0 {candidates=before.Versions}
  userLease,err=state.NewInvalidator(userStore).Acquire(menuCtx,candidates);if err!=nil{return err}
  workCtx,stop:=userLease.StartRenewal(menuCtx);stopUser=stop
  if err:=tx.WithContext(workCtx).Exec(string(sql)).Error;err!=nil{return err}
  after,err=inspect(workCtx,tx);if err!=nil{return err}
  if err:=verify(before,after);err!=nil{return err}
  report.After=&after
  if err:=saveReport(auditDir,report);err!=nil{return err}
  return workCtx.Err()
 });if err!=nil{return err}
 committed=true;report.PostgresCommitted=true;report.After=&after
 if err:=saveReport(auditDir,report);err!=nil{return err}
 report.FailureStage="redis-publish"
 cpBefore,_:=canvasPlatform(before);cpAfter,_:=canvasPlatform(after)
 if cpBefore.MenuVersion==cpAfter.MenuVersion {err=menuLease.Rollback(ctx)} else {err=menuLease.Commit(ctx,cpAfter.MenuVersion)}
 if err!=nil{return err}
 if before.GrantCount==0 {
  versions:=map[int64]int64{};for _,v:=range after.Versions {versions[v.UserID]=v.Version}
  err=userLease.Commit(ctx,versions)
 } else {err=userLease.Rollback(ctx)}
 if err!=nil{return err}
 report.RedisSynchronized=true
 report.FailureStage="verify"
 if err:=verifyRedis(ctx,rc,after);err!=nil{return err}
 if err:=verifyAccess(ctx,pg.GORM,rc,after);err!=nil{return err}
 report.Verified=true;report.FailureStage=""
 return nil
}

func inspect(ctx context.Context,db *gorm.DB)(facts,error){
 f:=facts{Platforms:[]platform{},Versions:[]state.Version{}}
 if err:=db.WithContext(ctx).Raw(`SELECT id,code,menu_version,policy_version FROM permission_auth_platform WHERE deleted_at IS NULL ORDER BY id`).Scan(&f.Platforms).Error;err!=nil{return f,err}
 var roleID int64
 if err:=db.WithContext(ctx).Raw(`SELECT id FROM permission_role WHERE code='registered_user' AND is_enabled=1 AND deleted_at IS NULL`).Scan(&roleID).Error;err!=nil{return f,err}
 if roleID<1{return f,errors.New("registered_user role missing")}
 var err error;f.Versions,err=role.NewRepository(db).FindEffectiveAccessVersionsByRole(ctx,roleID);if err!=nil{return f,err}
 query:=`SELECT
 coalesce((SELECT id FROM permission_menu WHERE platform_id=(SELECT id FROM permission_auth_platform WHERE code='canvas' AND deleted_at IS NULL) AND code='canvas:workspace:view' AND deleted_at IS NULL),0) page_id,
 coalesce((SELECT id FROM permission_menu WHERE platform_id=(SELECT id FROM permission_auth_platform WHERE code='canvas' AND deleted_at IS NULL) AND code='storage:object:upload' AND deleted_at IS NULL),0) action_id,
 (SELECT count(*) FROM permission_menu m JOIN permission_auth_platform p ON p.id=m.platform_id WHERE p.code='canvas' AND m.code='canvas:workspace:view' AND m.parent_id IS NULL AND m.menu_type='page' AND m.is_hidden=1 AND m.is_enabled=1 AND m.path='/canvas/workspace' AND m.component_path='canvas/workspace' AND m.i18n_key='navigation.canvasWorkspace' AND m.deleted_at IS NULL) page_shape,
 (SELECT count(*) FROM permission_menu a JOIN permission_menu m ON m.id=a.parent_id JOIN permission_auth_platform p ON p.id=a.platform_id WHERE p.code='canvas' AND m.platform_id=a.platform_id AND m.code='canvas:workspace:view' AND a.code='storage:object:upload' AND a.menu_type='action' AND a.is_hidden=1 AND a.is_enabled=1 AND a.path IS NULL AND a.component_path IS NULL AND a.i18n_key IS NULL AND a.icon IS NULL AND a.deleted_at IS NULL) action_shape,
 (SELECT count(*) FROM permission_role_menu g JOIN permission_role r ON r.id=g.role_id JOIN permission_menu m ON m.id=g.menu_id JOIN permission_auth_platform p ON p.id=m.platform_id WHERE p.code='canvas' AND r.code='registered_user' AND m.code='storage:object:upload' AND g.deleted_at IS NULL AND m.deleted_at IS NULL) grant_count,
 (SELECT count(*) FROM permission_role_menu g JOIN permission_role r ON r.id=g.role_id JOIN permission_menu m ON m.id=g.menu_id JOIN permission_auth_platform p ON p.id=m.platform_id WHERE p.code='canvas' AND r.code='registered_user' AND m.code='canvas:workspace:view' AND g.deleted_at IS NULL) page_grants,
 coalesce((SELECT max(r.id) FROM storage_upload_rule r JOIN permission_auth_platform p ON p.id=r.platform_id JOIN storage_upload_rule_code c ON c.rule_id=r.id AND c.code='canvas' AND c.deleted_at IS NULL WHERE p.code='canvas' AND r.deleted_at IS NULL),0) rule_id,
 (SELECT count(*) FROM storage_upload_rule r JOIN permission_auth_platform p ON p.id=r.platform_id JOIN storage_upload_rule_code c ON c.rule_id=r.id AND c.code='canvas' AND c.deleted_at IS NULL WHERE p.code='canvas' AND r.deleted_at IS NULL AND r.is_enabled=1) rule_count,
 md5(jsonb_build_object('menus',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM permission_menu m),'rules',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM storage_upload_rule r),'codes',(SELECT jsonb_agg(to_jsonb(c) ORDER BY id) FROM storage_upload_rule_code c),'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM permission_role_menu g))::text) target_digest,
 md5(jsonb_build_object('menus',(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.id) FROM permission_menu m JOIN permission_auth_platform p ON p.id=m.platform_id WHERE p.code<>'canvas' OR m.code NOT IN ('canvas:workspace:view','storage:object:upload')),'rules',(SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM storage_upload_rule r JOIN permission_auth_platform p ON p.id=r.platform_id WHERE p.code<>'canvas'),'codes',(SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM storage_upload_rule_code c JOIN storage_upload_rule r ON r.id=c.rule_id JOIN permission_auth_platform p ON p.id=r.platform_id WHERE p.code<>'canvas'),'grants',(SELECT jsonb_agg(to_jsonb(g) ORDER BY g.id) FROM permission_role_menu g JOIN permission_menu m ON m.id=g.menu_id JOIN permission_auth_platform p ON p.id=m.platform_id WHERE p.code<>'canvas' OR m.code NOT IN ('canvas:workspace:view','storage:object:upload')))::text) unrelated_digest`
 platforms,versions:=f.Platforms,f.Versions
 if err:=db.WithContext(ctx).Raw(query).Scan(&f).Error;err!=nil{return f,err}
 f.Platforms,f.Versions=platforms,versions
 var ruleJSON string
 if err:=db.WithContext(ctx).Raw(`SELECT coalesce((SELECT row_to_json(r)::text FROM storage_upload_rule r WHERE id=?),'null')`,f.RuleID).Scan(&ruleJSON).Error;err!=nil{return f,err};f.Rule=json.RawMessage(ruleJSON)
 return f,nil
}

func canvasPlatform(f facts)(platform,error){for _,p:=range f.Platforms{if p.Code=="canvas"{return p,nil}};return platform{},errors.New("Canvas platform missing")}

func saveReport(dir string,report *audit)error{
 bytes,err:=json.MarshalIndent(report,"","  ");if err!=nil{return err}
 temporary:=filepath.Join(dir,"result.next.json")
 if err=os.WriteFile(temporary,bytes,0600);err!=nil{return err}
 return os.Rename(temporary,filepath.Join(dir,"result.json"))
}

func recoverRedis(ctx context.Context,db *gorm.DB,rc *redis.Client,path string,current facts,report *audit)error{
 report.FailureStage="recovery"
 bytes,err:=os.ReadFile(path);if err!=nil{return err}
 var previous audit;if err=json.Unmarshal(bytes,&previous);err!=nil{return err}
 if previous.DatabaseIdentity!=report.DatabaseIdentity||previous.RedisIdentity!=report.RedisIdentity||previous.SQLSHA256!=report.SQLSHA256{return errors.New("recovery audit belongs to a different database, Redis instance or migration version")}
 if previous.Before==nil||previous.After==nil{return errors.New("recovery audit lacks before/after facts")}
 if !sameFacts(*previous.After,current){return errors.New("database no longer matches audited committed/prepared facts; manual review required")}
 if err:=verify(*previous.Before,current);err!=nil{return err}
 report.Before=previous.Before;report.After=&current;report.PostgresCommitted=true
 base,_:=canvasPlatform(*previous.Before);next,_:=canvasPlatform(current)
 keys:=[]string{state.MenuStateKey(next.ID)};args:=[]any{base.MenuVersion,next.MenuVersion}
 for i,v:=range current.Versions {
  keys=append(keys,state.StateKey(v.UserID))
  args=append(args,previous.Before.Versions[i].Version,v.Version)
 }
 const script=`
for i,key in ipairs(KEYS) do
 local raw=redis.call('GET',key)
 if raw then
  local ok,s=pcall(cjson.decode,raw)
  if not ok or s.schemaVersion~=2 then return redis.error_reply('corrupt state') end
  if s.state~='ready' then return redis.error_reply('mutation lease still active') end
  local prior=tonumber(ARGV[(i-1)*2+1]); local next=tonumber(ARGV[(i-1)*2+2])
  if s.version~=prior and s.version~=next then return redis.error_reply('unexpected version') end
 end
end
for i,key in ipairs(KEYS) do
 redis.call('SET',key,cjson.encode({schemaVersion=2,state='ready',version=tonumber(ARGV[(i-1)*2+2])}))
end
return 'recovered'
`
 reply,err:=rc.EvalString(ctx,script,keys,args...);if err!=nil{return err};if reply!="recovered"{return errors.New("Redis recovery not confirmed")}
 report.RedisSynchronized=true
 if err=verifyRedis(ctx,rc,current);err!=nil{return err}
 if err=verifyAccess(ctx,db,rc,current);err!=nil{return err}
 report.Verified=true;report.FailureStage="";return nil
}

func sameFacts(left,right facts)bool{
 var l,r any
 lb,le:=json.Marshal(left);rb,re:=json.Marshal(right)
 if le!=nil||re!=nil{return false}
 if json.Unmarshal(lb,&l)!=nil||json.Unmarshal(rb,&r)!=nil{return false}
 return reflect.DeepEqual(l,r)
}

func verify(before,after facts)error{
 if after.PageShape!=1||after.ActionShape!=1||after.GrantCount!=1||after.PageGrants!=0||after.RuleCount!=1{return errors.New("Canvas target shape incomplete")}
 if before.UnrelatedDigest!=after.UnrelatedDigest{return errors.New("unrelated facts changed")}
 for _,p:=range before.Platforms{
  var match platform;for _,a:=range after.Platforms{if a.ID==p.ID{match=a}}
  want:=p.MenuVersion;if p.Code=="canvas"&&(before.PageID==0||before.ActionID==0){want++}
  if match.Code!=p.Code||match.PolicyVersion!=p.PolicyVersion||match.MenuVersion!=want{return errors.New("platform versions changed unexpectedly")}
 }
 if len(before.Versions)!=len(after.Versions){return errors.New("role membership changed")}
 for i,v:=range before.Versions{want:=v.Version;if before.GrantCount==0{want++};if after.Versions[i].UserID!=v.UserID||after.Versions[i].Version!=want{return errors.New("access version increment mismatch")}}
 if before.PageID>0&&before.ActionID>0&&before.GrantCount==1&&before.RuleID>0&&!sameFacts(before,after){return errors.New("repeat migration changed facts")}
 return nil
}

func verifyRedis(ctx context.Context,rc *redis.Client,f facts)error{
 cp,_:=canvasPlatform(f)
 m,found,err:=state.NewMenuStore(rc).Read(ctx,cp.ID);if err!=nil{return err};if !found||m.State!=state.StateReady||m.Version!=cp.MenuVersion{return errors.New("Canvas menu Redis state unconfirmed")}
 for _,v:=range f.Versions{s,found,err:=state.NewStore(rc).Read(ctx,v.UserID);if err!=nil{return err};if !found||s.State!=state.StateReady||s.Version!=v.Version{return errors.New("user access Redis state unconfirmed")}}
 return nil
}

func verifyAccess(ctx context.Context,db *gorm.DB,rc *redis.Client,f facts)error{
 cp,_:=canvasPlatform(f)
 for _,v:=range f.Versions{
  source,err:=access.NewRepository(db).FindSourceWithVersion(ctx,v.UserID,cp.ID);if err!=nil{return err}
  pageExpected:=source.SuperAdmin
  for _,id:=range source.GrantedMenuIDs{if id==f.PageID{pageExpected=true}}
  // Use two service/client cache instances to exercise version confirmation.
  for i:=0;i<2;i++{
   client,err:=redis.Open(ctx,os.Getenv("REDIS_URL"));if err!=nil{return err}
   service:=access.NewService(access.NewRepository(db),state.NewStore(client),access.NewSnapshotCache(client),access.NewLocalSnapshotCache(16),slog.Default(),state.NewMenuStore(client))
   snapshot,err:=service.Current(ctx,auth.Identity{UserID:v.UserID,SessionID:1,PlatformID:cp.ID,Platform:"canvas",PolicyVersion:cp.PolicyVersion,AccessCacheTTL:time.Minute})
   client.Close();if err!=nil{return err}
   allowed,pageAllowed:=false,false;for _,code:=range snapshot.PermissionCodes{if code=="storage:object:upload"{allowed=true};if code=="canvas:workspace:view"{pageAllowed=true}}
   if !allowed{return errors.New("Canvas upload permission missing")}
   pageInTree:=false;for _,m:=range snapshot.MenuTree{if m.Code=="canvas:workspace:view"{pageInTree=true}}
   if pageAllowed!=pageExpected||pageInTree!=pageExpected{return errors.New("workspace page differs from explicit PostgreSQL grants")}
  }
 }
 return nil
}
'@

try {
    foreach ($tool in @('go','psql','pg_dump','pg_restore')) { Get-Command $tool -ErrorAction Stop | Out-Null }
    New-Item -ItemType Directory -Force -Path $auditRoot,$helperRoot | Out-Null
    $helperPath = Join-Path $helperRoot 'main.go'
    $helper | Set-Content -LiteralPath $helperPath -Encoding utf8
    $audit.sqlSHA256 = (Get-FileHash -LiteralPath $sqlPath).Hash
    $audit.helperSHA256 = (Get-FileHash -LiteralPath $helperPath).Hash
    Copy-Item -LiteralPath $sqlPath,$helperPath -Destination $auditRoot
    if (-not $CheckOnly) {
        $taskDsn = [Environment]::GetEnvironmentVariable('POSTGRES_DSN')
        if ([string]::IsNullOrWhiteSpace($taskDsn)) {
            $line = Get-Content (Join-Path $serverRoot '.env') | Where-Object { $_ -match '^POSTGRES_DSN=' } | Select-Object -First 1
            if ($null -eq $line) { throw '缺少 POSTGRES_DSN。' }
            $taskDsn = $line.Substring(13).Trim().Trim('"').Trim("'")
        }
        $taskDsn = $taskDsn -replace '(?i)\s+TimeZone=\S+', ''
        $env:PGAPPNAME='admin-canvas-upload-maintenance'
        $backupPath = Join-Path $auditRoot 'public-before.dump'
        & pg_dump --format=custom --schema=public --no-owner --no-acl --dbname=$taskDsn --file=$backupPath 2> (Join-Path $auditRoot 'postgres-errors.log')
        if ($LASTEXITCODE -ne 0) { throw '数据库备份失败，迁移未执行。' }
        & pg_restore --list $backupPath > (Join-Path $auditRoot 'backup-list.txt') 2>> (Join-Path $auditRoot 'postgres-errors.log')
        if ($LASTEXITCODE -ne 0) { throw '备份目录校验失败，迁移未执行。' }
        & pg_restore --no-owner --no-acl --file=NUL $backupPath 2>> (Join-Path $auditRoot 'postgres-errors.log')
        if ($LASTEXITCODE -ne 0) { throw '备份完整读取失败，迁移未执行。' }
        $audit.backupSHA256=(Get-FileHash -LiteralPath $backupPath).Hash
        $audit.backupVerified=$true
    }
    if ($RecoverFrom) {
        if ($CheckOnly) { throw 'CheckOnly 与 RecoverFrom 不能同时使用。' }
        $resolvedRecovery=[IO.Path]::GetFullPath($RecoverFrom)
        $recoveryParent=[IO.Path]::GetFullPath((Join-Path $env:LOCALAPPDATA 'Admin/maintenance/canvas-auth-upload')) + [IO.Path]::DirectorySeparatorChar
        if (-not $resolvedRecovery.StartsWith($recoveryParent,[StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($resolvedRecovery) -ne 'result.json') { throw '恢复文件必须来自本次 Canvas 维护的审计目录。' }
        $audit.recoverFrom=$resolvedRecovery
    }
    Push-Location $serverRoot
    try {
        $executablePath=Join-Path $auditRoot 'canvas-auth-upload.exe'
        & go build -o $executablePath ('./tmp/' + (Split-Path $helperRoot -Leaf))
        if ($LASTEXITCODE -ne 0) { throw '维护程序编译失败，迁移未执行。' }
        $taskArgs=@('-sql', $sqlPath, '-audit', $auditRoot)
        if ($CheckOnly) { $taskArgs += '-check' }
        if ($RecoverFrom) { $taskArgs += @('-recover',$resolvedRecovery) }
        & $executablePath @taskArgs
        if ($LASTEXITCODE -ne 0) { throw 'Canvas 维护未完成；PostgreSQL 是否已提交以 result.json 为准，请保留审计后重试。' }
    } finally { Pop-Location }
    $audit.result=Get-Content -Raw (Join-Path $auditRoot 'result.json') | ConvertFrom-Json
    Write-Output "审计记录：$auditRoot"
} catch {
    $audit.error=$_.Exception.Message
    Write-Error $_.Exception.Message -ErrorAction Continue
    exit 1
} finally {
    $env:PGOPTIONS=$oldOptions; $env:PGAPPNAME=$oldAppName
    $audit.finishedAt=(Get-Date).ToString('o')
    if (Test-Path $auditRoot) { $audit | ConvertTo-Json -Depth 12 | Set-Content (Join-Path $auditRoot 'audit.json') -Encoding utf8 }
    if (Test-Path $helperRoot) {
        $resolvedHelper=[IO.Path]::GetFullPath($helperRoot)
        $expectedParent=[IO.Path]::GetFullPath((Join-Path $serverRoot 'tmp')) + [IO.Path]::DirectorySeparatorChar
        if (-not $resolvedHelper.StartsWith($expectedParent,[StringComparison]::OrdinalIgnoreCase)) { throw '临时目录越界，未清理。' }
        Remove-Item -LiteralPath $resolvedHelper -Recurse -Force
    }
}
