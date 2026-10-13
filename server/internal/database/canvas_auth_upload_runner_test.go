package database_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	projectredis "admin/server/internal/redis"
)

// Compile and execute the actual embedded maintenance helper, not source-text assertions.
func TestCanvasAuthUploadRunner(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL/Redis maintenance integration")
	}
	root := repoRoot(t)
	dsn := mustPostgresDSN(t)
	parsed, err := url.Parse(os.Getenv("REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/14"
	redisURL := parsed.String()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	rc, err := projectredis.Open(ctx, redisURL)
	if err != nil {
		t.Fatal(err)
	}
	count, err := rc.UniversalClient().DBSize(ctx).Result()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		keys, _ := rc.UniversalClient().Keys(ctx, "*").Result()
		_ = rc.Close()
		t.Fatalf("isolated Redis DB14 must be empty; no keys were changed: %v", keys)
	}
	marker := "test:canvas-upload-runner:owner"
	owned, err := rc.UniversalClient().SetNX(ctx, marker, fmt.Sprint(os.Getpid()), 3*time.Minute).Result()
	if err != nil || !owned {
		t.Fatalf("claim isolated Redis DB: %v", err)
	}
	t.Cleanup(func() {
		defer rc.Close()
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = rc.Delete(cleanup, marker)
		remaining, err := rc.UniversalClient().DBSize(cleanup).Result()
		if err != nil || remaining != 0 {
			t.Errorf("isolated Redis cleanup: remaining=%d error=%v", remaining, err)
		}
	})
	content, err := os.ReadFile(filepath.Join(root, "docs/database/2026-10-13-canvas-auth-upload.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	_, body, found := strings.Cut(normalized, "$helper = @'\n")
	if !found {
		t.Fatal("embedded helper missing")
	}
	body, _, found = strings.Cut(body, "\n'@")
	if !found {
		t.Fatal("embedded helper terminator missing")
	}
	fixture, err := os.ReadFile(filepath.Join(root, "server/internal/database/canvas_auth_upload_migration_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	_, fixtureBody, _ := strings.Cut(string(fixture), "func canvasUploadFixture")
	fixtureBody, _, _ = strings.Cut(fixtureBody, "func runCanvasMigration")
	temp, err := os.MkdirTemp(filepath.Join(root, "server/tmp"), "canvasRunnerTest-")
	if err != nil {
		t.Fatal(err)
	}
	// MkdirTemp returns a checked child of server/tmp; remove only that test-owned directory.
	t.Cleanup(func() {
		if err := os.RemoveAll(temp); err != nil {
			t.Error(err)
		}
	})
	if err := os.WriteFile(filepath.Join(temp, "main.go"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	generated := canvasRunnerTests + "\nfunc canvasUploadFixture" + fixtureBody
	if err := os.WriteFile(filepath.Join(temp, "main_test.go"), []byte(generated), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "go", "test", "-v", "-count=1", "./tmp/"+filepath.Base(temp))
	command.Dir = filepath.Join(root, "server")
	command.Env = append(os.Environ(), "POSTGRES_DSN="+dsn, "REDIS_URL="+redisURL, "CANVAS_SQL="+filepath.Join(root, "docs/database/2026-10-13-canvas-auth-upload.sql"))
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
}

const canvasRunnerTests = `package main
import (
 "context"
 "encoding/json"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "admin/server/internal/database/testschema"
 state "admin/server/internal/module/permission/state"
 redis "admin/server/internal/redis"
 "gorm.io/gorm"
 "gorm.io/gorm/logger"
)
func mustPostgresDSN(t *testing.T) string { t.Helper(); return os.Getenv("POSTGRES_DSN") }
func runnerFixture(t *testing.T) (*gorm.DB,context.Context,*redis.Client) {
 t.Helper()
 db,ctx:=canvasUploadFixture(t)
 rc,err:=redis.Open(ctx,os.Getenv("REDIS_URL"));if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){
  ctx,stop:=context.WithTimeout(context.Background(),5*time.Second);defer stop()
  keys,err:=rc.UniversalClient().Keys(ctx,"authz:*").Result();if err!=nil{t.Error(err)}
  for _,key:=range keys{
   if key==state.MenuStateKey(2)||key==state.StateKey(100)||key==state.StateKey(102)||strings.HasPrefix(key,"authz:permission:v8:canvas:1:100:")||strings.HasPrefix(key,"authz:permission:v8:canvas:1:102:"){
    if err:=rc.Delete(ctx,key);err!=nil{t.Error(err)}
   }else{t.Errorf("unexpected isolated key left untouched: %s",key)}
  }
  rc.Close()
 })
 sql,err:=os.ReadFile(os.Getenv("CANVAS_SQL"));if err!=nil{t.Fatal(err)}
 if err=db.WithContext(ctx).Transaction(func(tx *gorm.DB)error{return tx.Exec(string(sql)).Error});err!=nil{t.Fatal(err)}
 return db,ctx,rc
}
func TestRunnerMixedRoleAccess(t *testing.T){
 db,ctx,rc:=runnerFixture(t)
 f,err:=inspect(ctx,db);if err!=nil{t.Fatal(err)}
 if err=verifyAccess(ctx,db,rc,f);err!=nil{t.Fatalf("action-only ordinary member: %v",err)}
 if err=db.WithContext(ctx).Exec("INSERT INTO permission_user_role VALUES(102,42,NULL)").Error;err!=nil{t.Fatal(err)}
 f,err=inspect(ctx,db);if err!=nil{t.Fatal(err)}
 if err=verifyAccess(ctx,db,rc,f);err!=nil{t.Fatalf("super_admin plus registered_user is legitimate: %v",err)}
 if err=db.WithContext(ctx).Exec("INSERT INTO permission_role(id,code,is_default,is_enabled) VALUES(43,'workspace_editor',0,1); INSERT INTO permission_user_role VALUES(100,43,NULL); INSERT INTO permission_role_menu(role_id,menu_id) SELECT 43,id FROM permission_menu WHERE platform_id=2 AND code='canvas:workspace:view'; UPDATE permission_access_version SET version=version+1 WHERE user_id=100").Error;err!=nil{t.Fatal(err)}
 // Remove only this test user's version/snapshot so the next source is authoritative.
 if err=rc.Delete(ctx,state.StateKey(100));err!=nil{t.Fatal(err)}
 f,err=inspect(ctx,db);if err!=nil{t.Fatal(err)}
 if err=verifyAccess(ctx,db,rc,f);err!=nil{t.Fatalf("explicit page grant from another role is legitimate: %v",err)}
 if f.PageGrants!=0{t.Fatal("registered_user unexpectedly gained page grant")}
}
func TestRunnerRecoverAfterRestart(t *testing.T){
 db,ctx,rc:=runnerFixture(t)
 f,err:=inspect(ctx,db);if err!=nil{t.Fatal(err)}
 old:=audit{Before:&f,After:&f,DatabaseIdentity:"fixture/public",RedisIdentity:"127.0.0.1:6379/14/0123456789012345678901234567890123456789",SQLSHA256:"fixture-sha",PostgresCommitted:true}
 bytes,err:=json.Marshal(old);if err!=nil{t.Fatal(err)}
 path:=filepath.Join(t.TempDir(),"result.json");if err=os.WriteFile(path,bytes,0600);err!=nil{t.Fatal(err)}
 report:=audit{DatabaseIdentity:old.DatabaseIdentity,RedisIdentity:"127.0.0.1:6379/14",SQLSHA256:old.SQLSHA256}
 if err=recoverRedis(ctx,db,rc,path,f,&report);err!=nil{t.Fatalf("same endpoint/database after restart must recover missing state: %v",err)}
 if !report.PostgresCommitted||!report.RedisSynchronized||!report.Verified{t.Fatal("recovery not fully verified")}
 for _,field:=range []string{"database","redis","sql"}{
  bad:=report;switch field{case "database":bad.DatabaseIdentity="other/public";case "redis":bad.RedisIdentity="127.0.0.1:6379/13";case "sql":bad.SQLSHA256="other"}
  if err=recoverRedis(ctx,db,rc,path,f,&bad);err==nil{t.Fatalf("accepted different %s target",field)}
 }
 for _,raw:=range []string{"{broken","{\"schemaVersion\":2,\"state\":\"ready\",\"version\":999}","{\"schemaVersion\":2,\"state\":\"invalidating\",\"version\":0,\"baseVersion\":5,\"mutationToken\":\"other-writer\"}"}{
  if err=rc.SetString(ctx,state.StateKey(100),raw,time.Minute);err!=nil{t.Fatal(err)}
  if err=recoverRedis(ctx,db,rc,path,f,&report);err==nil{t.Fatalf("accepted unsafe state %s",raw)}
  got,_,err:=rc.GetString(ctx,state.StateKey(100));if err!=nil||got!=raw{t.Fatal("recovery overwrote rejected state")}
 }
}
`
