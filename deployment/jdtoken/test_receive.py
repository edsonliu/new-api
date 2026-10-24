"""Verify release authorization against real Git objects, without deployment."""
import json
import hashlib
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch
from docs_release import file_hashes, publish_docs

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


class DocsReleaseTest(unittest.TestCase):
    def test_docs_gate_publication_and_rollback(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source = root / 'source'
            source.mkdir()
            def git(*args):
                return subprocess.check_output(['git', '-C', str(source), *args], stderr=subprocess.DEVNULL).decode().strip()
            git('init')
            (source / 'app.txt').write_text('unchanged application')
            git('add', '.')
            git('-c', 'user.name=Release test', '-c', 'user.email=release@example.invalid', 'commit', '-m', 'app')
            baseline = git('rev-parse', 'HEAD')
            scripts = source / 'deployment' / 'jdtoken'
            scripts.mkdir(parents=True)
            (root / 'bin').mkdir()
            for filename in ('receive.py', 'deploy.py', 'docs_release.py'):
                shutil.copyfile(Path(__file__).parent / filename, scripts / filename)
                shutil.copyfile(scripts / filename, root / 'bin' / filename)
            docs = source / 'jdtoken' / 'gateway-docs'
            shutil.copytree(Path(__file__).parents[2] / 'jdtoken' / 'gateway-docs', docs,
                            ignore=shutil.ignore_patterns('dist', 'examples.json', '__pycache__'))
            git('add', '.')
            git('-c', 'user.name=Release test', '-c', 'user.email=release@example.invalid', 'commit', '-m', 'docs')
            commit = git('rev-parse', 'HEAD')
            subprocess.run(['/usr/bin/python3', str(docs / 'build.py')], check=True, stdout=subprocess.DEVNULL)
            tag = 'jdtoken-v' + json.loads((docs / 'dist' / 'release.json').read_text())['version']
            git('-c', 'user.name=Release test', '-c', 'user.email=release@example.invalid', 'tag', '-a', tag, '-m', 'docs')
            oid = git('rev-parse', tag)
            repo = root / 'repository.git'
            subprocess.run(['git', 'clone', '--bare', str(source), str(repo)], check=True, capture_output=True)
            artifact = root / 'artifacts' / tag
            shutil.copytree(docs / 'dist', artifact / 'docs')
            hashes = file_hashes(artifact / 'docs')
            manifest = {'commit': commit, 'version': tag, 'target': '115.29.196.54', 'validated': True,
                        'release_type': 'docs', 'app_commit': baseline, 'app_image_id': 'sha256:fixture', 'docs_files': hashes}
            stage = {'success': True, 'version': tag, 'release_type': 'docs', 'app_image_id': 'sha256:fixture', 'docs_files': hashes}
            def save():
                (artifact / 'manifest.json').write_text(json.dumps(manifest))
                (artifact / 'stage-result.json').write_text(json.dumps(stage))
            save()
            ref = 'refs/tags/' + tag
            self.assertEqual(validate('0'*40, oid, ref, root), tag)
            index = artifact / 'docs' / 'index.html'
            original = index.read_bytes()
            index.write_text('tampered')
            with self.assertRaises(ValueError):
                validate('0'*40, oid, ref, root)
            # Even a matching forged manifest cannot substitute a different build.
            manifest['docs_files'] = stage['docs_files'] = file_hashes(artifact / 'docs')
            save()
            with self.assertRaisesRegex(ValueError, 'tagged source build'):
                validate('0'*40, oid, ref, root)
            index.write_bytes(original)
            manifest['docs_files'] = stage['docs_files'] = hashes
            stage['success'] = False
            save()
            with self.assertRaises(ValueError):
                validate('0'*40, oid, ref, root)
            stage['success'] = True
            save()
            (artifact / 'docs' / 'link').symlink_to(index)
            with self.assertRaises(ValueError):
                validate('0'*40, oid, ref, root)
            (artifact / 'docs' / 'link').unlink()
            # A change outside the docs/delivery directories cannot use docs-only delivery.
            (source / 'app.txt').write_text('changed application')
            git('add', '.')
            git('-c', 'user.name=Release test', '-c', 'user.email=release@example.invalid', 'commit', '-m', 'bad scope')
            bad_commit = git('rev-parse', 'HEAD')
            subprocess.run(['git', '--git-dir', str(repo), 'fetch', str(source), 'HEAD'], check=True, capture_output=True)
            from docs_release import validate_docs
            with self.assertRaisesRegex(ValueError, 'application changes'):
                validate_docs(repo, bad_commit, artifact, manifest, stage)
            app = {'container_id': 'same', 'image_id': manifest['app_image_id'], 'commit': baseline,
                   'version': 'jdtoken-v2026.10.23.1', 'started_at': 'same', 'restart_count': 0}
            # Exercise real filesystem copying, modes, atomic switch and rollback.
            for fail in (False, True):
                with self.subTest(fail=fail):
                    site = root / ('fail-site' if fail else 'site')
                    previous = site / 'releases' / 'old'
                    (previous / 'docs').mkdir(parents=True)
                    (previous / 'docs' / 'index.html').write_text('old docs')
                    (site / 'current').symlink_to(previous)
                    def get(url, **kwargs):
                        from types import SimpleNamespace
                        if url.endswith('/api/status'):
                            return SimpleNamespace(json=lambda: {'data': {'version':app['version'], 'system_name':'JDOTOKEN'}})
                        name = url.split('/docs/', 1)[1] or 'index.html'
                        return SimpleNamespace(status_code=503 if fail else 200,
                            content=(site / 'current' / 'docs' / name).read_bytes())
                    with patch('docs_release.app_snapshot', return_value=app), patch('requests.get', side_effect=get):
                        if fail:
                            with self.assertRaisesRegex(ValueError, 'Public documentation verification failed'):
                                publish_docs(artifact, repo, commit, manifest, site, root / 'backups')
                            self.assertEqual((site / 'current').resolve(), previous)
                            self.assertFalse(json.loads((artifact / 'deployment-result.json').read_text())['success'])
                        else:
                            publish_docs(artifact, repo, commit, manifest, site, root / 'backups')
                            self.assertEqual(file_hashes(site / 'current' / 'docs'), hashes)
                            self.assertEqual((site / 'current' / 'docs' / 'index.html').stat().st_mode & 0o777, 0o644)
                            self.assertTrue(json.loads((artifact / 'deployment-result.json').read_text())['application_preserved'])
                            with self.assertRaisesRegex(ValueError, 'already been deployed'):
                                validate('0'*40, oid, ref, root)



if __name__ == '__main__':
    unittest.main()
