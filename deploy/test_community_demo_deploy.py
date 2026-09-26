import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import community_demo_deploy as deploy


def payload(files=None, link=None):
    output = io.BytesIO()
    entries = files or {'zero-city': b'binary', 'frontend/index.html': b'<html>demo</html>',
                        'release.json': json.dumps({'sha': 'a' * 40, 'run_id': 42}).encode()}
    with tarfile.open(fileobj=output, mode='w:gz') as archive:
        for name, content in entries.items():
            item = tarfile.TarInfo(name)
            item.size = len(content)
            archive.addfile(item, io.BytesIO(content))
        if link:
            item = tarfile.TarInfo('frontend/link')
            item.type = tarfile.SYMTYPE
            item.linkname = link
            archive.addfile(item)
    return output.getvalue()


class DeployTests(unittest.TestCase):
    def test_api_health_alone_cannot_pass_deployment(self):
        def response(url, timeout):
            if url.endswith('/health'):
                return io.BytesIO(b'{"status":"ok"}')
            if url.endswith('/settings/public'):
                return io.BytesIO(b'{"code":0}')
            return io.BytesIO(b'404 page not found')
        with patch.object(deploy, 'urlopen', side_effect=response), patch.object(deploy.time, 'sleep'):
            self.assertFalse(deploy.healthy('a' * 40))

    def test_unpack_complete_release(self):
        with tempfile.TemporaryDirectory() as name:
            root = Path(name)
            archive = root / 'input.tgz'
            archive.write_bytes(payload())
            deploy.unpack(archive, root)
            self.assertEqual((root / 'zero-city').read_bytes(), b'binary')

    def test_reject_traversal_links_and_unexpected_files(self):
        for content in [payload({'../escape': b'x'}), payload({'/escape': b'x'}),
                        payload({'config.env': b'x'}), payload(link='/etc/passwd')]:
            with self.subTest(), tempfile.TemporaryDirectory() as name:
                archive = Path(name) / 'input.tgz'
                archive.write_bytes(content)
                with self.assertRaises(ValueError):
                    deploy.unpack(archive, Path(name))

    def test_reject_unverified_and_stale_builds(self):
        good = {'conclusion': 'success', 'head_sha': 'a' * 40, 'head_branch': 'main',
                'event': 'push', 'path': '.github/workflows/checks.yml',
                'repository': {'full_name': deploy.REPO}}
        for change in [{'conclusion': 'failure'}, {'event': 'pull_request'},
                       {'head_branch': 'other'}, {'path': 'untrusted.yml'}]:
            with self.subTest(change=change), patch.object(deploy, 'api', return_value={**good, **change}):
                with self.assertRaises(ValueError):
                    deploy.verify_release({'sha': 'a' * 40, 'run_id': 42})
        with patch.object(deploy, 'api', side_effect=[good, {'object': {'sha': 'b' * 40}}]):
            with self.assertRaises(ValueError):
                deploy.verify_release({'sha': 'a' * 40, 'run_id': 42})

    def test_failed_health_restores_executable_and_demo_database(self):
        with tempfile.TemporaryDirectory() as name:
            root = Path(name)
            previous = root / 'releases/bootstrap'
            previous.mkdir(parents=True)
            (root / 'current').symlink_to(previous, target_is_directory=True)
            stdin = type('Input', (), {'buffer': io.BytesIO(payload())})()
            with patch.object(deploy, 'ROOT', root), patch.object(deploy.sys, 'stdin', stdin), \
                    patch.object(deploy, 'verify_release', return_value='a' * 40), \
                    patch.object(deploy, 'healthy', side_effect=[False, True]), \
                    patch.object(deploy.subprocess, 'run') as run:
                with self.assertRaisesRegex(RuntimeError, 'prior executable and demo database restored'):
                    deploy.main()
                self.assertEqual((root / 'current').resolve(), previous)
                commands = [call.args[0] for call in run.call_args_list]
                self.assertTrue(any('pg_dump' in args for args in commands))
                self.assertTrue(any('pg_restore' in args and 'city_community_demo' in args for args in commands))
                self.assertFalse((root / 'deployment.json').exists())


if __name__ == '__main__':
    unittest.main()
