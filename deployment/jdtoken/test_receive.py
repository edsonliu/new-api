"""Verify release authorization against real Git objects, without deployment."""
import json
import hashlib
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

from receive import validate


class ReleaseGateTest(unittest.TestCase):
    def test_release_gate(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = root / 'repository.git'
            source = root / 'source'
            source.mkdir()
            def git(*args):
                return subprocess.check_output(['git', '-C', str(source), *args], stderr=subprocess.DEVNULL).decode().strip()
            git('init')
            scripts = source / 'deployment' / 'jdtoken'
            scripts.mkdir(parents=True)
            (root / 'bin').mkdir()
            for filename in ('receive.py', 'deploy.py'):
                shutil.copyfile(Path(__file__).parent / filename, scripts / filename)
                shutil.copyfile(scripts / filename, root / 'bin' / filename)
            git('add', '.')
            git('-c', 'user.name=Release test', '-c', 'user.email=release@example.invalid', 'commit', '-m', 'fixture')
            commit = git('rev-parse', 'HEAD')
            tag = 'jdtoken-v2026.10.22.1'
            git('-c', 'user.name=Release test', '-c', 'user.email=release@example.invalid', 'tag', '-a', tag, '-m', 'fixture')
            oid = git('rev-parse', tag)
            subprocess.run(['git', 'clone', '--bare', str(source), str(repo)], check=True, capture_output=True)
            artifact = root / 'artifacts' / tag
            artifact.mkdir(parents=True)
            manifest = {'commit': commit, 'version': tag, 'target': '115.29.196.54', 'validated': True, 'image_id': 'sha256:test'}
            stage = {'success': True, 'version': tag, 'image_id': 'sha256:test'}
            def save():
                (artifact / 'manifest.json').write_text(json.dumps(manifest))
                (artifact / 'stage-result.json').write_text(json.dumps(stage))
            save()
            ref = 'refs/tags/' + tag
            self.assertEqual(validate('0'*40, oid, ref, root), tag)
            for old, new, bad_ref in [(oid, oid, ref), ('0'*40, '0'*40, ref), ('0'*40, oid, 'refs/heads/main'), ('0'*40, commit, ref), ('0'*40, oid, ref+'-beta')]:
                with self.subTest(ref=bad_ref, old=old, new=new), self.assertRaises(ValueError):
                    validate(old, new, bad_ref, root)
            for key, bad in [('commit', 'f'*40), ('validated', False), ('target', '34.47.118.212')]:
                before = manifest[key]
                manifest[key] = bad
                save()
                with self.subTest(key=key), self.assertRaises(ValueError):
                    validate('0'*40, oid, ref, root)
                manifest[key] = before
            stage['image_id'] = 'sha256:other'
            save()
            with self.assertRaises(ValueError):
                validate('0'*40, oid, ref, root)
            stage['image_id'] = manifest['image_id']
            save()
            (artifact / 'runtime.env').write_text('JDOTOKEN_SEEDANCE_UPLOAD_SECRET=fixture-only')
            manifest['runtime_sha256'] = hashlib.sha256((artifact / 'runtime.env').read_bytes()).hexdigest()
            stage['runtime_sha256'] = manifest['runtime_sha256']
            save()
            self.assertEqual(validate('0'*40, oid, ref, root), tag)
            (artifact / 'runtime.env').write_text('changed')
            with self.assertRaises(ValueError):
                validate('0'*40, oid, ref, root)
            del manifest['runtime_sha256']
            save()
            (root / 'bin' / 'deploy.py').write_text('changed')
            with self.assertRaises(ValueError):
                validate('0'*40, oid, ref, root)


if __name__ == '__main__':
    unittest.main()
