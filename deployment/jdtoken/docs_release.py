"""Static docs delivery through the existing immutable production tag receiver."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time

SOURCE = 'jdtoken/gateway-docs/'


def file_hashes(directory):
    directory = Path(directory)
    if directory.is_symlink() or not directory.is_dir():
        raise ValueError('Expected a regular artifact directory')
    result = {}
    for path in sorted(directory.rglob('*')):
        if path.is_symlink() or not (path.is_file() or path.is_dir()):
            raise ValueError('Artifact must contain only regular files and directories')
        if path.is_file():
            result[path.relative_to(directory).as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
    if not result:
        raise ValueError('Empty documentation artifact')
    return result


def validate_docs(repo, commit, artifact, manifest, stage):
    """Rebuild from the tagged source, and compare every byte with staged files."""
    def git(*args):
        return subprocess.check_output(['git', '--git-dir', str(repo), *args])
    app_commit = manifest.get('app_commit', '')
    if len(app_commit) != 40 or any(c not in '0123456789abcdef' for c in app_commit):
        raise ValueError('Documentation release requires its exact application baseline')
    subprocess.run(['git', '--git-dir', str(repo), 'merge-base', '--is-ancestor', app_commit, commit], check=True)
    changed = git('diff', '--name-only', '-z', app_commit, commit).decode().split('\0')
    if any(path and not path.startswith((SOURCE, 'deployment/jdtoken/')) for path in changed):
        raise ValueError('Docs-only release contains application changes')
    if (stage.get('release_type') != 'docs' or stage.get('success') is not True
            or stage.get('version') != manifest.get('version')
            or stage.get('app_image_id') != manifest.get('app_image_id')
            or not manifest.get('app_image_id')):
        raise ValueError('Documentation has not passed staging with this application baseline')
    metadata = json.loads((artifact / 'docs' / 'release.json').read_text())
    if metadata.get('version') != manifest['version'].removeprefix('jdtoken-v'):
        raise ValueError('Documentation version differs from release tag')
    expected = manifest.get('docs_files')
    if not expected or stage.get('docs_files') != expected or file_hashes(artifact / 'docs') != expected:
        raise ValueError('Documentation differs from validated artifact')
    with tempfile.TemporaryDirectory(prefix='jdtoken-docs-build-') as tmp:
        source = Path(tmp)
        for item in git('ls-tree', '-rz', commit, '--', SOURCE).split(b'\0'):
            if not item:
                continue
            meta, raw_path = item.split(b'\t', 1)
            mode, kind, oid = meta.split()
            if mode not in (b'100644', b'100755') or kind != b'blob':
                raise ValueError('Documentation sources must be regular files')
            relative = Path(raw_path.decode().removeprefix(SOURCE))
            if relative.is_absolute() or '..' in relative.parts:
                raise ValueError('Unsafe source path')
            path = source / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(git('cat-file', 'blob', oid.decode()))
        subprocess.run(['/usr/bin/python3', '-B', str(source / 'build.py')], check=True,
                       stdout=subprocess.DEVNULL, timeout=30, cwd=source)
        if file_hashes(source / 'dist') != expected:
            raise ValueError('Staged documentation differs from tagged source build')


def app_snapshot():
    app = json.loads(subprocess.check_output(['docker', 'inspect', 'jdtoken-new-api-prod']))[0]
    if not app['State']['Running'] or app['State'].get('Health', {}).get('Status') != 'healthy':
        raise ValueError('Production application is not healthy')
    labels = app['Config']['Labels']
    return {'container_id': app['Id'], 'image_id': app['Image'],
            'started_at': app['State']['StartedAt'], 'restart_count': app['RestartCount'],
            'commit': labels['org.opencontainers.image.revision'],
            'version': labels['org.opencontainers.image.version']}


def publish_docs(root, repo, commit, manifest, site=Path('/data/www/jdtoken-docs'),
                 backups=Path('/data/backups/jdtoken-new-api')):
    import requests
    tag = manifest['version']
    stage = json.loads((root / 'stage-result.json').read_text())
    validate_docs(repo, commit, root, manifest, stage)
    before = app_snapshot()
    if before['commit'] != manifest['app_commit'] or before['image_id'] != manifest['app_image_id']:
        raise ValueError('Live application no longer matches the documentation baseline')
    domains = ('https://www.jdotoken.com', 'https://jdotoken.com')
    for domain in domains:
        data = requests.get(domain + '/api/status', timeout=20).json()['data']
        if data['version'] != before['version'] or data['system_name'] != 'JDOTOKEN':
            raise ValueError('Unexpected public application identity')
    current = site / 'current'
    previous = current.resolve(strict=True)
    if not current.is_symlink() or previous.parent != site / 'releases':
        raise ValueError('Unexpected current documentation location')
    destination = site / 'releases' / tag
    if destination.exists():
        raise ValueError('Immutable documentation release already exists')
    backup = backups / (tag + '-docs-' + str(time.time_ns()))
    backup.mkdir(parents=True)
    state = {'git_tag': tag, 'commit': commit, 'release_type': 'docs', 'backup': str(backup),
             'previous_docs': str(previous), 'new_docs': str(destination), 'application': before}
    (backup / 'before.json').write_text(json.dumps(state, indent=2))
    (root / 'before.json').write_text(json.dumps(state, indent=2))
    shutil.copytree(previous, backup / 'docs-release')
    shutil.copytree(root / 'docs', destination / 'docs')
    for path in [destination, *destination.rglob('*')]:
        path.chmod(0o755 if path.is_dir() else 0o644)
    if file_hashes(destination / 'docs') != manifest['docs_files']:
        raise ValueError('Published candidate copy differs from validated artifact')
    pending = site / ('.current-' + tag)
    switched = False
    try:
        pending.symlink_to(destination)
        os.replace(pending, current)
        switched = True
        for domain in domains:
            for name, digest in manifest['docs_files'].items():
                path = '' if name == 'index.html' else name
                response = requests.get(domain + '/docs/' + path, timeout=30,
                                        headers={'Cache-Control': 'no-cache'})
                if response.status_code != 200 or hashlib.sha256(response.content).hexdigest() != digest:
                    raise ValueError('Public documentation verification failed: ' + domain + '/docs/' + path)
            data = requests.get(domain + '/api/status', timeout=20).json()['data']
            if data['version'] != before['version']:
                raise ValueError('Public application version changed')
        if app_snapshot() != before:
            raise ValueError('Application changed during documentation publication')
        result = {**state, 'success': True, 'version': tag, 'application_preserved': True,
                  'docs_files': manifest['docs_files'], 'public_domains_verified': list(domains)}
        (root / 'deployment-result.json').write_text(json.dumps(result, indent=2))
        print(json.dumps(result), flush=True)
    except Exception:
        if switched:
            pending.unlink(missing_ok=True)
            pending.symlink_to(previous)
            os.replace(pending, current)
        (root / 'deployment-result.json').write_text(json.dumps({**state, 'success': False,
              'rolled_back': current.resolve() == previous}, indent=2))
        raise
    finally:
        pending.unlink(missing_ok=True)
