#!/usr/bin/env python3
"""Tag-only deployment receiver; installed outside the bare repository."""
import json
import hashlib
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path('/data/app/jdtoken-new-api/git-release')
TAG = re.compile(r'jdtoken-v[0-9]{4}\.[0-9]{2}\.[0-9]{2}\.[1-9][0-9]*')


def validate(old, new, ref, root=ROOT):
    if old != '0' * 40 or not re.fullmatch('[0-9a-f]{40}', new) or new == '0' * 40:
        raise ValueError('Only new immutable release tags are accepted')
    prefix = 'refs/tags/'
    tag = ref.removeprefix(prefix)
    if not ref.startswith(prefix) or not TAG.fullmatch(tag):
        raise ValueError('Expected jdtoken-vYYYY.MM.DD.N production tag')
    repo = root / 'repository.git'
    def git(*args):
        return subprocess.check_output(['git', '--git-dir', str(repo), *args])
    if git('cat-file', '-t', new).strip() != b'tag':
        raise ValueError('An annotated tag is required')
    commit = git('rev-parse', new + '^{commit}').decode().strip()
    artifact = root / 'artifacts' / tag
    manifest = json.loads((artifact / 'manifest.json').read_text())
    stage = json.loads((artifact / 'stage-result.json').read_text())
    if (manifest.get('commit') != commit or manifest.get('version') != tag
            or manifest.get('target') != '115.29.196.54'
            or manifest.get('validated') is not True):
        raise ValueError('Release manifest does not authorize this exact commit and target')
    if (stage.get('success') is not True or stage.get('version') != tag
            or stage.get('image_id') != manifest.get('image_id')):
        raise ValueError('This image has not passed the production-copy rehearsal')
    if manifest.get('runtime_sha256'):
        actual = hashlib.sha256((artifact / 'runtime.env').read_bytes()).hexdigest()
        if actual != manifest['runtime_sha256'] or stage.get('runtime_sha256') != actual:
            raise ValueError('Runtime configuration differs from the rehearsed release')
    for filename in ('receive.py', 'deploy.py'):
        committed = git('show', commit + ':deployment/jdtoken/' + filename)
        if committed != (root / 'bin' / filename).read_bytes():
            raise ValueError('Installed delivery script differs from tagged source')
    if (artifact / 'deployment-result.json').exists():
        raise ValueError('Release has already been deployed')
    return tag


def main():
    updates = [line.split() for line in sys.stdin if line.strip()]
    if len(updates) != 1 or len(updates[0]) != 3:
        raise ValueError('Push exactly one release tag')
    tag = validate(*updates[0])
    if sys.argv[1] == 'post':
        # The push is the trigger. Never run deployment from branch pushes or
        # by manually invoking deploy.py. systemd keeps durable execution logs.
        subprocess.run([
            'systemd-run', '--unit=jdtoken-release-' + tag,
            '--property=Type=oneshot', '--property=TimeoutStartSec=5400',
            '/usr/bin/python3', str(ROOT / 'bin' / 'deploy.py'), tag,
        ], check=True)
    print('Validated production release tag:', tag)


if __name__ == '__main__':
    main()
