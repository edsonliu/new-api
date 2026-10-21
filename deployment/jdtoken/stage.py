"""Production-host rehearsal on a private network and a disposable database copy."""
import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import time
import requests

os.umask(0o077)
tag = sys.argv[2]
assert __import__('re').fullmatch(r"jdtoken-v[0-9]{4}\.[0-9]{2}\.[0-9]{2}\.[1-9][0-9]*", tag)
root = Path('/data/app/jdtoken-new-api/git-release/artifacts')/tag
root.mkdir(parents=True, exist_ok=True)
network = 'jdtoken-upgrade-isolated-20261022'
pg = 'jdtoken-upgrade-stage-pg-20261022'
redis = 'jdtoken-upgrade-stage-redis-20261022'
app = 'jdtoken-upgrade-stage-app-20261022'
image = 'jdtoken-new-api:'+tag

def run(*args, **kwargs):
    return subprocess.check_output(list(args), text=True, **kwargs).strip()

def sql(container, query):
    return run('docker', 'exec', container, 'psql', '-U', 'jdtoken', '-d', 'jdtoken_new_api', '-At', '-c', query)

if sys.argv[1] == 'prepare':
    assert not (root / 'stage-prepared.json').exists()
    assert sql('jdtoken-postgres-prod', "SELECT count(*) FROM tasks WHERE status NOT IN ('SUCCESS','FAILURE')") == '0'
    with (root / 'rehearsal.dump').open('wb') as out:
        subprocess.run(['docker', 'exec', 'jdtoken-postgres-prod', 'pg_dump', '-U', 'jdtoken', '-d', 'jdtoken_new_api', '-Fc'], stdout=out, check=True)
    subprocess.run(['docker','exec','jdtoken-postgres-prod','pg_dump','-U','jdtoken','-d','jdtoken_new_api','--schema-only'], stdout=(root/'schema-before.sql').open('w'), check=True)
    run('docker','network','create','--internal',network)
    password = secrets.token_urlsafe(32)
    (root/'postgres.env').write_text('POSTGRES_USER=jdtoken\nPOSTGRES_DB=jdtoken_new_api\nPOSTGRES_PASSWORD='+password+'\n')
    run('docker','run','-d','--name',pg,'--network',network,'--env-file',str(root/'postgres.env'),'postgres:17-alpine')
    run('docker','run','-d','--name',redis,'--network',network,'redis:7-alpine')
    for _ in range(30):
        ready = subprocess.run(['docker','exec',pg,'pg_isready','-h','127.0.0.1','-U','jdtoken'],capture_output=True)
        if ready.returncode == 0: break
        time.sleep(1)
    else: raise RuntimeError('Stage PostgreSQL not ready')
    with (root/'rehearsal.dump').open('rb') as src:
        subprocess.run(['docker','exec','-i',pg,'pg_restore','-U','jdtoken','-d','jdtoken_new_api','--no-owner','--exit-on-error'], stdin=src, check=True)
    # Defense in depth: no upstream traffic from the copied configuration.
    sql(pg, 'UPDATE channels SET status=2; UPDATE abilities SET enabled=false;')
    inspect = json.loads(run('docker','inspect','jdtoken-new-api-prod'))[0]
    env = dict(s.split('=',1) for s in inspect['Config']['Env'])
    env.update(SQL_DSN=f'postgres://jdtoken:{password}@{pg}:5432/jdtoken_new_api?sslmode=disable',
               REDIS_CONN_STRING=f'redis://{redis}:6379', NODE_TYPE='master',
               SESSION_COOKIE_TRUSTED_URL='https://www.jdotoken.com,https://jdotoken.com', SESSION_SECRET=secrets.token_urlsafe(32))
    (root/'app.env').write_text(''.join(f'{k}={v}\n' for k,v in env.items()))
    (root/'stage-prepared.json').write_text(json.dumps({'production_image':inspect['Config']['Image'],'internal_network':network,'channels_disabled':True,'dump_bytes':(root/'rehearsal.dump').stat().st_size},indent=2))
    print((root/'stage-prepared.json').read_text())
elif sys.argv[1] in ('start','verify'):
    if sys.argv[1]=='start':
        run('docker','run','-d','--name',app,'--network',network,'--env-file',str(root/'app.env'),image)
    inspect=json.loads(run('docker','inspect',app))[0]
    stage_base='http://'+inspect['NetworkSettings']['Networks'][network]['IPAddress']+':3000'
    for _ in range(45):
        try:
            status=requests.get(stage_base+'/api/status',timeout=2)
            if status.ok and status.json().get('success'):break
        except requests.RequestException:pass
        time.sleep(1)
    else:raise RuntimeError('New application failed to start')
    assert status.json()['data']['version']==tag
    assert sql(pg,"SELECT count(*) FROM information_schema.columns WHERE table_name='tokens' AND column_name='allow_user_quota_query'")=='1'
    assert sql(pg,"SELECT count(*) FROM pg_indexes WHERE indexname='idx_audit_time_event'")=='1'
    about=requests.get(stage_base+'/api/about',timeout=10).json()['data']
    assert 'jd-about' in about and '及第智算' in about
    keys="'About','SystemName','Logo','ServerAddress','general_setting.docs_link','passkey.origins','USDExchangeRate','GroupRatio','GroupGroupRatio','billing_setting.billing_expr','billing_setting.billing_mode','billing_setting.plugin_billing_expr'"
    query=f"SELECT key,value FROM options WHERE key IN ({keys}) ORDER BY key"
    stage_values=sql(pg,query)
    prod_values=sql('jdtoken-postgres-prod',query)
    assert stage_values==prod_values, 'Critical configuration changed during migration'
    creds=json.loads(Path('/data/app/jdtoken-new-api/admin-credentials.json').read_text())
    session=requests.Session();session.headers['Origin']='https://www.jdotoken.com'
    login=session.post(stage_base+'/api/user/login',json={'username':creds['username'],'password':creds['password']},timeout=15).json()
    assert login.get('success'), 'Staging admin login failed'
    session.headers['Authorization']='Bearer '+login['data']['access_token']
    try:
        assert session.get(stage_base+'/api/user/self',timeout=10).json()['success']
    finally:
        assert session.post(stage_base+'/api/user/auth/logout',json={},timeout=10).json()['success']
        session.close()
    subprocess.run(['docker','exec',pg,'pg_dump','-U','jdtoken','-d','jdtoken_new_api','--schema-only'],stdout=(root/'schema-after.sql').open('w'),check=True)
    result={'success':True,'version':status.json()['data']['version'],'audit_index_present':True,'image_id':inspect['Image'],'critical_settings_preserved':True,'about_preserved':True,'admin_login_logout':True,'internal_network':True}
    (root/'stage-result.json').write_text(json.dumps(result,indent=2));print(json.dumps(result))
