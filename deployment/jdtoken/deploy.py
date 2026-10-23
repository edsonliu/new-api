"""Publish JDOTOKEN application images or isolated static documentation via tags."""
import sys
import re
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import requests

os.umask(0o077)
tag = sys.argv[1]
assert re.fullmatch(r"jdtoken-v[0-9]{4}\.[0-9]{2}\.[0-9]{2}\.[1-9][0-9]*", tag)
release_root = Path('/data/app/jdtoken-new-api/git-release')
lock = (release_root/'deploy.lock').open('w')
fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
root = release_root/'artifacts'/tag
manifest = json.loads((root/'manifest.json').read_text())
version = tag
repo = release_root/'repository.git'
commit = subprocess.check_output(['git','--git-dir',str(repo),'rev-parse',f'refs/tags/{tag}^{{commit}}'],text=True).strip()
assert commit == manifest['commit']
assert manifest['version'] == tag and manifest['target'] == '115.29.196.54'
assert manifest['validated'] is True
if manifest.get('release_type') == 'docs':
    from docs_release import publish_docs
    publish_docs(root, repo, commit, manifest)
    sys.exit(0)
assert manifest.get('release_type', 'app') == 'app'
image = manifest['image']
image_meta = json.loads(subprocess.check_output(['docker','image','inspect',image]))[0]
assert image_meta['Id'] == manifest['image_id']
assert image_meta['Config']['Labels']['org.opencontainers.image.revision'] == commit
assert image_meta['Config']['Labels']['org.opencontainers.image.version'] == tag
appdir = Path('/data/app/jdtoken-new-api')
nginx = Path('/etc/nginx/sites-available/jdtoken-new-api')
compose = appdir/'compose.yml'
base = 'https://www.jdotoken.com'
backup = Path('/data/backups/jdtoken-new-api') / (tag+'-'+str(time.time_ns()))
backup.mkdir(parents=True)

def run(*args, **kwargs):
    return subprocess.check_output(list(args), text=True, **kwargs).strip()

def sql(query):
    return run('docker','exec','jdtoken-postgres-prod','psql','-U','jdtoken','-d','jdtoken_new_api','-At','-c',query)

def fingerprint():
    return {name:hashlib.sha256(sql(query).encode()).hexdigest() for name,query in {
        'options': 'SELECT key,value FROM options ORDER BY key',
        'channels': 'SELECT row_to_json(c) FROM (SELECT id,name,type,status,key,base_url,models,model_mapping,"group",setting FROM channels ORDER BY id)c',
    }.items()}

def wait_ready(expected):
    for _ in range(50):
        try:
            d=requests.get('http://127.0.0.1:3001/api/status',timeout=2).json()
            if d.get('success') and d['data'].get('version')==expected:return d['data']
        except (requests.RequestException,ValueError):pass
        time.sleep(1)
    raise RuntimeError('Application did not become ready')

stage=json.loads((root/'stage-result.json').read_text())
assert stage['success'] and stage['version']==version and stage['image_id']==manifest['image_id']
old=json.loads(run('docker','inspect','jdtoken-new-api-prod'))[0]
old_image=old['Config']['Image']
status=requests.get(base+'/api/status',timeout=20).json()['data']
assert status['system_name']=='JDOTOKEN' and status['server_address']==base
assert sql("SELECT count(*) FROM tasks WHERE status NOT IN ('SUCCESS','FAILURE')")=='0'
old_config=compose.read_text(); old_nginx=nginx.read_text()
new_nginx=old_nginx
runtime_path=appdir/'seedance.env'
old_runtime=runtime_path.read_bytes() if runtime_path.exists() else None
runtime={}
if manifest.get('runtime_sha256'):
    runtime_bytes=(root/'runtime.env').read_bytes()
    assert hashlib.sha256(runtime_bytes).hexdigest()==manifest['runtime_sha256']
    assert stage.get('runtime_sha256')==manifest['runtime_sha256']
    runtime=dict(line.split('=',1) for line in runtime_bytes.decode().splitlines() if line)
    assert set(runtime)=={'JDOTOKEN_SEEDANCE_CONFIG','JDOTOKEN_SEEDANCE_UPLOAD_DIR','JDOTOKEN_SEEDANCE_PUBLIC_URL','JDOTOKEN_SEEDANCE_UPLOAD_SECRET'}
    assert runtime['JDOTOKEN_SEEDANCE_UPLOAD_DIR']=='/data/seedance-uploads'
    assert runtime['JDOTOKEN_SEEDANCE_PUBLIC_URL']==base
    assert len(runtime['JDOTOKEN_SEEDANCE_UPLOAD_SECRET'])>=32
    assert json.loads(runtime['JDOTOKEN_SEEDANCE_CONFIG'])['enabled'] is True
    if 'log_format jdtoken_safe ' not in new_nginx:
        new_nginx = ('log_format jdtoken_safe \'$remote_addr [$time_local] "$request_method $uri $server_protocol" $status $body_bytes_sent\';\n' + new_nginx)
        new_nginx = new_nginx.replace('access_log /var/log/nginx/jdtoken-new-api.access.log;', 'access_log /var/log/nginx/jdtoken-new-api.access.log jdtoken_safe;')
assert old_config.count(old_image)==1
env_before = dict(s.split('=',1) for s in old['Config']['Env'])
assert env_before['SESSION_COOKIE_TRUSTED_URL']=='https://www.jdotoken.com,https://jdotoken.com'
for name in ('RELAY_TIMEOUT','RELAY_RESPONSE_HEADER_TIMEOUT','STREAMING_TIMEOUT'): assert env_before[name]=='3601'
target='        proxy_pass http://127.0.0.1:3001;'
assert old_nginx.count(target)==1
before=fingerprint()
shutil.copy2(compose,backup/'compose.yml')
shutil.copy2(nginx,backup/'nginx.conf')
if (appdir/'.env').exists():shutil.copy2(appdir/'.env',backup/'.env')
if old_runtime is not None:(backup/'seedance.env').write_bytes(old_runtime)
if runtime and (appdir/'app-data'/'seedance-uploads').exists():
    shutil.copytree(appdir/'app-data'/'seedance-uploads', backup/'seedance-uploads')
about_before=requests.get(base+'/api/about',timeout=20).json()['data']
state={'backup':str(backup),'previous_image':old_image,'previous_version':status.get('version',''),'new_image':image,'git_tag':tag,'commit':commit,'fingerprints':before}
(root/'before.json').write_text(json.dumps(state,indent=2))
maintenance=False
switched=False
completed=False
try:
    nginx.write_text(new_nginx.replace(target,'        add_header Retry-After 60 always;\n        return 503;\n'+target))
    run('nginx','-t');run('systemctl','reload','nginx');maintenance=True
    print('Maintenance enabled; waiting for existing requests',flush=True)
    for _ in range(45):
        active=run('ss','-Htn','state','established','(','dport','=',':3001',')')
        if not active:break
        time.sleep(1)
    else:raise RuntimeError('Requests still in flight; publication canceled')
    assert sql("SELECT count(*) FROM tasks WHERE status NOT IN ('SUCCESS','FAILURE')")=='0'
    run('docker','stop','--time','3605','jdtoken-new-api-prod')
    switched=True
    with (backup/'database.dump').open('wb') as out:
        subprocess.run(['docker','exec','jdtoken-postgres-prod','pg_dump','-U','jdtoken','-d','jdtoken_new_api','-Fc'],stdout=out,check=True)
    assert (backup/'database.dump').stat().st_size>0
    with (backup/'database.dump').open('rb') as src:
        subprocess.run(['docker','exec','-i','jdtoken-postgres-prod','pg_restore','--list'],stdin=src,stdout=subprocess.DEVNULL,check=True)
    next_config=old_config.replace(old_image,image)
    if runtime:
        assert not any(key in env_before for key in runtime), 'Runtime additions must not override existing environment'
        assert '    env_file:' not in next_config, 'Review existing env_file before changing runtime'
        next_config=next_config.replace('    environment:', '    env_file: [./seedance.env]\n    environment:', 1)
        runtime_path.write_bytes(runtime_bytes)
        runtime_path.chmod(0o600)
        (appdir/'app-data'/'seedance-uploads').mkdir(mode=0o700,exist_ok=True)
    compose.write_text(next_config)
    run('docker','compose','config','--quiet',cwd=appdir)
    run('docker','compose','up','-d','--no-deps','app',cwd=appdir)
    wait_ready(version)
    assert fingerprint()==before, 'Site settings or channel configuration changed'
    assert requests.get('http://127.0.0.1:3001/api/about',timeout=10).json()['data']==about_before
    env_after = dict(s.split('=',1) for s in json.loads(run('docker','inspect','jdtoken-new-api-prod'))[0]['Config']['Env'])
    assert env_after == (env_before | runtime), 'Runtime environment changed'
    creds=json.loads((appdir/'admin-credentials.json').read_text())
    session=requests.Session();session.headers['Origin']=base
    login=session.post('http://127.0.0.1:3001/api/user/login',json={'username':creds['username'],'password':creds['password']},timeout=15).json()
    assert login.get('success'), 'Admin sign-in failed'
    session.headers['Authorization']='Bearer '+login['data']['access_token']
    try:
        assert session.get('http://127.0.0.1:3001/api/user/self',timeout=10).json()['success']
    finally:
        assert session.post('http://127.0.0.1:3001/api/user/auth/logout',json={},timeout=10).json()['success']
        session.close()
    nginx.write_text(new_nginx);run('nginx','-t');run('systemctl','reload','nginx');maintenance=False
    # A successful reload signal precedes worker replacement. Wait for the
    # public listener to stop answering with the old maintenance response.
    for _ in range(25):
        try:
            response=requests.get(base+'/api/status',headers={'Connection':'close'},timeout=5)
            final=response.json().get('data',{}) if response.status_code==200 else {}
            if final.get('version')==version:break
        except (requests.RequestException,ValueError):pass
        time.sleep(1)
    else:raise RuntimeError('Public listener did not become ready after reload')
    for domain in ('https://www.jdotoken.com','https://jdotoken.com'):
        assert requests.get(domain+'/api/status',timeout=10).json()['data']['version']==version
        assert requests.get(domain+'/docs/',timeout=10).status_code==200
    result={**state,'success':True,'version':version,'configuration_preserved':True,'about_preserved':True,'admin_login_logout':True,'nginx_restored':True,'database_dump_bytes':(backup/'database.dump').stat().st_size}
    result['runtime_sha256']=manifest.get('runtime_sha256')
    completed=True
    (root/'deployment-result.json').write_text(json.dumps(result,indent=2));print(json.dumps(result),flush=True)
except Exception:
    if switched:
        compose.write_text(old_config)
        if old_runtime is None:runtime_path.unlink(missing_ok=True)
        else:runtime_path.write_bytes(old_runtime)
        run('docker','compose','up','-d','--no-deps','app',cwd=appdir)
        wait_ready(status.get('version',''))
        # Added audit index and cache-stat columns are tolerated by the old image.
        # Do not restore the database over newer user transactions.
    raise
finally:
    desired_nginx=new_nginx if completed else old_nginx
    if nginx.read_text()!=desired_nginx:
        nginx.write_text(desired_nginx);run('nginx','-t');run('systemctl','reload','nginx')
