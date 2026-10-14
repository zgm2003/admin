[CmdletBinding()]
param([switch]$ServicesStopped, [string]$RecoverFrom)
$ErrorActionPreference = 'Stop'
if (-not $ServicesStopped) { throw '请先停止所有 API/Worker 实例，再显式传入 -ServicesStopped；脚本不会操作服务' }
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$server = Join-Path $root 'server'
$sql = Join-Path $PSScriptRoot '2026-10-14-payment-config-remove-code.sql'
if (-not $env:POSTGRES_DSN -or -not $env:REDIS_URL) {
    foreach ($line in Get-Content -LiteralPath (Join-Path $server '.env')) {
        if ($line -match '^\s*(POSTGRES_DSN|REDIS_URL)\s*=\s*(.+)\s*$') {
            if (-not [Environment]::GetEnvironmentVariable($matches[1])) {
                [Environment]::SetEnvironmentVariable($matches[1], $matches[2].Trim().Trim([char]34).Trim([char]39))
            }
        }
    }
}
if (-not $env:POSTGRES_DSN -or -not $env:REDIS_URL) { throw '缺少 POSTGRES_DSN / REDIS_URL' }
$auditRoot = Join-Path $env:LOCALAPPDATA 'Admin/maintenance/payment-config-remove-code'
$audit = Join-Path $auditRoot ([DateTime]::Now.ToString('yyyyMMdd-HHmmss-fff') + '-' + [Guid]::NewGuid().ToString('N'))
$tempRoot = [IO.Path]::GetFullPath((Join-Path $server 'tmp'))
$temp = [IO.Path]::GetFullPath((Join-Path $tempRoot ('payment-config-' + [Guid]::NewGuid().ToString('N'))))
if (-not $temp.StartsWith($tempRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw '临时路径超出 workspace' }
New-Item -ItemType Directory -Path $temp -Force | Out-Null
$helper = @'
package main

import (
 "bytes"; "context"; "crypto/sha256"; "encoding/hex"; "encoding/json"; "errors"; "fmt"; "os"; "os/exec"; "path/filepath"; "strconv"; "strings"; "time"
 "github.com/jackc/pgx/v5"
 redis "github.com/redis/go-redis/v9"
)
const migration = "2026-10-14-payment-config-remove-code"
type Snapshot struct {MenuVersion int64;Generation int64;MenuCount int64;OutboxCount int64;Digest string;HasCode bool;PaymentDigest string;SchemaDigest string}
type Audit struct {Migration string;StartedAt time.Time;SQLSHA256 string;DatabaseID string;RedisID string;RedisRunID string;BackupPath string;BackupSHA256 string;Before Snapshot;Expected Snapshot;After Snapshot;PostgresAttempted bool;PostgresOutcome string;PostgresCommitted bool;RedisSynchronized bool;Verified bool;RecoverySource string;Error string}
func hash(raw []byte)string{sum:=sha256.Sum256(raw);return hex.EncodeToString(sum[:])}
func writeAudit(path string,a Audit)error{raw,err:=json.MarshalIndent(a,"","  ");if err!=nil{return err};temp:=path+".tmp";f,err:=os.OpenFile(temp,os.O_WRONLY|os.O_CREATE|os.O_TRUNC,0600);if err!=nil{return err};_,writeErr:=f.Write(raw);syncErr:=f.Sync();closeErr:=f.Close();if err=errors.Join(writeErr,syncErr,closeErr);err!=nil{return err};return os.Rename(temp,path)}
func main(){if err:=run();err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(1)}}
func run()(resultErr error){
 ctx,cancel:=context.WithTimeout(context.Background(),3*time.Minute);defer cancel()
 directory:=os.Getenv("PAYMENT_CONFIG_AUDIT");if directory==""{return errors.New("audit directory required")}
 if err:=os.MkdirAll(directory,0700);err!=nil{return errors.New("create audit directory failed")}
 auditPath:=filepath.Join(directory,"audit.json");a:=Audit{Migration:migration,StartedAt:time.Now().UTC(),RecoverySource:os.Getenv("PAYMENT_CONFIG_RECOVER")}
 defer func(){if resultErr!=nil {a.Error=resultErr.Error()};if err:=writeAudit(auditPath,a);err!=nil{resultErr=errors.Join(resultErr,errors.New("write audit failed"))};if resultErr!=nil&&a.PostgresCommitted&&!a.RedisSynchronized {resultErr=errors.Join(errors.New("PostgreSQL 已提交、Redis 同步未完成；不要启动服务，修复后用 -RecoverFrom 恢复"),resultErr)}}()
 sql,err:=os.ReadFile(os.Getenv("PAYMENT_CONFIG_SQL"));if err!=nil{return errors.New("read migration SQL failed")};a.SQLSHA256=hash(sql)
 cfg,err:=pgx.ParseConfig(os.Getenv("POSTGRES_DSN"));if err!=nil{return errors.New("PostgreSQL configuration invalid")};cfg.DefaultQueryExecMode=pgx.QueryExecModeSimpleProtocol
 db,err:=pgx.ConnectConfig(ctx,cfg);if err!=nil{return errors.New("PostgreSQL connection failed")};defer db.Close(ctx)
 var schema,dbName string;if err=db.QueryRow(ctx,"SELECT current_schema(),current_database()").Scan(&schema,&dbName);err!=nil{return errors.New("database identity read failed")}
 a.DatabaseID=hash([]byte(fmt.Sprintf("%s:%d/%s/%s",cfg.Host,cfg.Port,dbName,schema)))
 ro,err:=redis.ParseURL(os.Getenv("REDIS_URL"));if err!=nil{return errors.New("Redis configuration invalid")};ro.DialTimeout=2*time.Second;ro.ReadTimeout=2*time.Second;ro.WriteTimeout=2*time.Second;ro.MaxRetries=-1;ro.ContextTimeoutEnabled=true;r:=redis.NewClient(ro);defer r.Close()
 info,err:=r.Info(ctx,"server").Result();if err!=nil{return errors.New("Redis identity read failed")};for _,line:=range strings.Split(info,"\n"){if strings.HasPrefix(line,"run_id:"){a.RedisRunID=strings.TrimSpace(strings.TrimPrefix(line,"run_id:"))}};if a.RedisRunID==""{return errors.New("Redis identity unavailable")};a.RedisID=hash([]byte(fmt.Sprintf("%s/%s/%d/tls=%t",ro.Network,strings.ToLower(ro.Addr),ro.DB,ro.TLSConfig!=nil)))
 var adminID int64;if err=db.QueryRow(ctx,"SELECT id FROM permission_auth_platform WHERE code='admin' AND deleted_at IS NULL").Scan(&adminID);err!=nil{return errors.New("Admin platform missing or ambiguous")}
 configKey:="config-cache:state:v1:payment.config:global"
 for _,key:=range []string{configKey}{raw,err:=r.Get(ctx,key).Result();if err!=nil&&!errors.Is(err,redis.Nil){return errors.New("Redis state preflight failed")};if err==nil {var v struct{State string};if json.Unmarshal([]byte(raw),&v)!=nil||v.State=="invalidating"{return errors.New("Redis state corrupt or active mutation; do not override")}}}
 before,err:=snapshot(ctx,db);if err!=nil{return errors.New("read pre-migration facts failed")};a.Before=before
 if a.RecoverySource!=""{
  raw,err:=os.ReadFile(a.RecoverySource);if err!=nil{return errors.New("read recovery audit failed")};var prior Audit;if json.Unmarshal(raw,&prior)!=nil||prior.Migration!=migration||(!prior.PostgresCommitted&&(!prior.PostgresAttempted||prior.PostgresOutcome!="commit_unknown"))||prior.SQLSHA256!=a.SQLSHA256||prior.DatabaseID!=a.DatabaseID||prior.RedisID!=a.RedisID{return errors.New("recovery audit does not match this migration and endpoints")}
  backup,err:=os.ReadFile(prior.BackupPath);if err!=nil||hash(backup)!=prior.BackupSHA256{return errors.New("recovery backup missing or hash mismatch")}
  if err:=ensurePaymentSchema(ctx,db);err!=nil{return errors.New("payment schema missing or incomplete; recovery refused")}
  if prior.Expected.Generation<1||prior.Expected.MenuCount!=9||before!=prior.Expected{return errors.New("recovery facts differ from the persisted pre-COMMIT expectation; no Redis state changed")}
  a.PostgresAttempted=true;a.PostgresCommitted=true;a.PostgresOutcome="committed";a.Expected=prior.Expected;a.BackupPath=prior.BackupPath;a.BackupSHA256=prior.BackupSHA256
 }else{
  backup:=filepath.Join(directory,"before.dump");command:=exec.CommandContext(ctx,"pg_dump","--format=custom","--no-owner","--file",backup,"--schema",schema);env:=[]string{};for _,v:=range os.Environ(){name,_,_:=strings.Cut(v,"=");if !strings.HasPrefix(strings.ToUpper(name),"PG"){env=append(env,v)}};command.Env=append(env,"PGHOST="+cfg.Host,"PGPORT="+strconv.Itoa(int(cfg.Port)),"PGDATABASE="+cfg.Database,"PGUSER="+cfg.User,"PGPASSWORD="+cfg.Password,"PGCLIENTENCODING=UTF8");var stderr bytes.Buffer;command.Stderr=&stderr;if err=command.Run();err!=nil{return errors.New("PostgreSQL backup failed; no migration applied")};raw,err:=os.ReadFile(backup);if err!=nil{return errors.New("backup read failed")};a.BackupPath=backup;a.BackupSHA256=hash(raw);if err=writeAudit(auditPath,a);err!=nil{return errors.New("write preflight audit failed")}
  tx,err:=db.Begin(ctx);if err!=nil{return errors.New("begin migration failed")};defer tx.Rollback(ctx);a.PostgresAttempted=true;a.PostgresOutcome="executing";if err=writeAudit(auditPath,a);err!=nil{return errors.New("write attempted audit failed; transaction not committed")}
  if _,err=tx.Exec(ctx,string(sql));err!=nil{a.PostgresOutcome="rolled_back";return errors.New("migration conflict or database failure; transaction rolled back")}
  a.Expected,err=snapshot(ctx,tx);if err!=nil{return errors.New("read pre-COMMIT expectation failed; transaction not committed")}
  expectedGeneration:=before.Generation;if before.HasCode{expectedGeneration++}
  if a.Expected.HasCode||a.Expected.PaymentDigest!=before.PaymentDigest||a.Expected.MenuVersion!=before.MenuVersion||a.Expected.MenuCount!=before.MenuCount||a.Expected.Generation!=expectedGeneration{return errors.New("migration changed unrelated facts; transaction not committed")}

  a.PostgresOutcome="commit_unknown";if err=writeAudit(auditPath,a);err!=nil{return errors.New("persist pre-COMMIT expectation failed; transaction not committed")}
  if err=tx.Commit(ctx);err!=nil{
   // Reconnect: the COMMIT answer may have been lost after the server committed.
   confirm,openErr:=pgx.ConnectConfig(ctx,cfg);if openErr!=nil{return errors.New("migration COMMIT outcome unknown; retain this audit and use -RecoverFrom after restoring PostgreSQL")};actual,readErr:=snapshot(ctx,confirm);confirm.Close(ctx)
   if readErr!=nil||actual!=a.Expected{return errors.New("migration COMMIT outcome unknown; retain this audit and use -RecoverFrom for exact fact reconciliation")}
   db.Close(ctx);db,err=pgx.ConnectConfig(ctx,cfg);if err!=nil{return errors.New("confirmed COMMIT but PostgreSQL reconnect failed; use -RecoverFrom")};defer db.Close(ctx)
  }
  a.PostgresCommitted=true;a.PostgresOutcome="committed";if err=writeAudit(auditPath,a);err!=nil{return errors.New("PostgreSQL committed; committed audit write failed; recover from pre-COMMIT expectation")}
 }
 if err:=ensurePaymentSchema(ctx,db);err!=nil{return errors.New("payment schema missing or incomplete; verification refused")}
 after,err:=snapshot(ctx,db);if err!=nil{return errors.New("read committed facts failed")};a.After=after;if after.Generation<1||after.MenuVersion<1||after.MenuCount!=9||after!=a.Expected{return errors.New("committed facts incomplete or different from persisted expectation")}
 // Execute the exact migration again inside a rolled-back transaction: definitions and seed must be a no-op.
 tx,err:=db.Begin(ctx);if err!=nil{return errors.New("begin replay verification failed")};if _,err=tx.Exec(ctx,"SET LOCAL payment.config_verify_only='on'");err!=nil{tx.Rollback(ctx);return errors.New("set verification mode failed")};if err=ensurePaymentSchema(ctx,tx);err!=nil{tx.Rollback(ctx);return errors.New("schema replay preflight failed")};if _,err=tx.Exec(ctx,string(sql));err!=nil{tx.Rollback(ctx);return errors.New("schema replay verification failed")};replay,err:=snapshot(ctx,tx);rollbackErr:=tx.Rollback(ctx);if err!=nil||rollbackErr!=nil||replay!=after{return errors.New("migration replay altered generation, menus or outbox")}
 if err=writeAudit(auditPath,a);err!=nil{return errors.New("write committed audit failed")}
 if err=publish(ctx,r,configKey,1,"generation",after.Generation);err!=nil{return err}
 a.RedisSynchronized=true;a.Verified=true;fmt.Println("支付配置迁移验证完成；审计："+auditPath);return nil
}
type queryer interface {QueryRow(context.Context,string,...any)pgx.Row}
func ensurePaymentSchema(ctx context.Context,db queryer)error{
 var complete bool
 err:=db.QueryRow(ctx,`SELECT
  EXISTS(SELECT 1 FROM pg_class WHERE oid=to_regclass('payment_config') AND relkind='r') AND
  EXISTS(SELECT 1 FROM pg_class WHERE oid=to_regclass('payment_config_version') AND relkind='r') AND
  NOT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=to_regclass('payment_config') AND attname='code' AND attnum>0 AND NOT attisdropped) AND
  to_regprocedure('payment_config_guard()') IS NOT NULL AND to_regprocedure('payment_config_version_guard()') IS NOT NULL AND
  (SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal AND tgenabled='O' AND
   ((tgrelid=to_regclass('payment_config') AND tgname IN('payment_config_identity_guard','payment_config_truncate_guard')) OR
    (tgrelid=to_regclass('payment_config_version') AND tgname='payment_config_version_immutable')))=3`).Scan(&complete)
 if err!=nil{return err};if !complete{return errors.New("required payment schema objects missing")};return nil
}
func snapshot(ctx context.Context,db queryer)(Snapshot,error){var s Snapshot;err:=db.QueryRow(ctx,`SELECT
 (SELECT menu_version FROM permission_auth_platform WHERE code='admin' AND deleted_at IS NULL),
 COALESCE((SELECT generation FROM system_config_cache_state WHERE namespace='payment.config' AND scope_key='global'),0),
 (SELECT count(*) FROM permission_menu WHERE platform_id=(SELECT id FROM permission_auth_platform WHERE code='admin' AND deleted_at IS NULL) AND deleted_at IS NULL AND (code='payment' OR code LIKE 'payment:config:%')),
 (SELECT count(*) FROM system_config_cache_outbox WHERE namespace='payment.config' AND scope_key='global'),
 md5(jsonb_build_object('menus',(SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM permission_menu m),'platforms',(SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM permission_auth_platform p),'state',(SELECT jsonb_agg(to_jsonb(s) ORDER BY namespace,scope_key) FROM system_config_cache_state s),'outbox',(SELECT jsonb_agg(to_jsonb(o) ORDER BY id) FROM system_config_cache_outbox o))::text),
 EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=to_regclass('payment_config') AND attname='code' AND attnum>0 AND NOT attisdropped),
 md5(jsonb_build_object('configs',(SELECT jsonb_agg(to_jsonb(c)-'code' ORDER BY id) FROM payment_config c),'versions',(SELECT jsonb_agg(to_jsonb(v) ORDER BY config_id,version) FROM payment_config_version v))::text),
 md5(jsonb_build_object('columns',(SELECT jsonb_agg(jsonb_build_array(c.relname,a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attidentity,pg_get_expr(d.adbin,d.adrelid)) ORDER BY c.relname,a.attnum) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE c.oid IN(to_regclass('payment_config'),to_regclass('payment_config_version')) AND a.attnum>0 AND NOT a.attisdropped),'guards',(SELECT jsonb_agg(jsonb_build_array(p.proname,p.prosrc) ORDER BY p.proname) FROM pg_proc p WHERE p.oid IN(to_regprocedure('payment_config_guard()'),to_regprocedure('payment_config_version_guard()'))))::text)`).Scan(&s.MenuVersion,&s.Generation,&s.MenuCount,&s.OutboxCount,&s.Digest,&s.HasCode,&s.PaymentDigest,&s.SchemaDigest);return s,err}
func publish(ctx context.Context,r *redis.Client,key string,schema int,field string,value int64)error{
 raw,err:=json.Marshal(map[string]any{"schemaVersion":schema,"state":"ready",field:value});if err!=nil{return err}
 script:=`local current=redis.call('GET',KEYS[1]);if current then local ok,v=pcall(cjson.decode,current);if not ok or v.schemaVersion~=tonumber(ARGV[2]) then return 'corrupt' end;if v.state=='invalidating' then return 'updating' end;if v.state~='ready' or tonumber(v[ARGV[3]])==nil or tonumber(v[ARGV[3]])>tonumber(ARGV[4]) then return 'mismatch' end end;redis.call('SET',KEYS[1],ARGV[1]);return 'published'`
 result,err:=r.Eval(ctx,script,[]string{key},string(raw),schema,field,value).Text();if err!=nil||result!="published"{return errors.New("Redis conditional synchronization failed")}
 actual,err:=r.Get(ctx,key).Result();if err!=nil||actual!=string(raw){return errors.New("Redis state verification failed")};return nil
}
'@
$env:PAYMENT_CONFIG_SQL = $sql
$env:PAYMENT_CONFIG_AUDIT = $audit
$env:PAYMENT_CONFIG_RECOVER = if ($RecoverFrom) { [IO.Path]::GetFullPath($RecoverFrom) } else { '' }
$oldLocation = Get-Location
try {
    [IO.File]::WriteAllText((Join-Path $temp 'main.go'), $helper, [Text.UTF8Encoding]::new($false))
    Set-Location -LiteralPath $server
    & go build -o (Join-Path $temp 'maintenance.exe') (Join-Path $temp 'main.go')
    if ($LASTEXITCODE -ne 0) { throw '维护工具编译失败，未执行迁移' }
    & (Join-Path $temp 'maintenance.exe')
    if ($LASTEXITCODE -ne 0) { throw "维护未完成；检查 $audit/audit.json；不要启动服务" }
} finally {
    Set-Location -LiteralPath $oldLocation
    $resolved = [IO.Path]::GetFullPath($temp)
    if ($resolved.StartsWith($tempRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { Remove-Item -LiteralPath $resolved -Recurse -Force }
    Remove-Item Env:PAYMENT_CONFIG_SQL,Env:PAYMENT_CONFIG_AUDIT,Env:PAYMENT_CONFIG_RECOVER -ErrorAction SilentlyContinue
}
