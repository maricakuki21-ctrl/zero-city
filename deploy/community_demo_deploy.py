#!/usr/bin/python3
"""Root-owned forced SSH command. Updates only the isolated community demo."""
import fcntl
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
from urllib.request import Request, urlopen

ROOT = Path('/data/zero-city-community/demo')
REPO = 'maricakuki21-ctrl/zero-city'
SERVICE = 'zero-city-community.service'


def api(path):
    request = Request('https://api.github.com/repos/' + REPO + path,
                      headers={'User-Agent': 'zero-city-test-deployer'})
    with urlopen(request, timeout=30) as response:
        return json.load(response)


def unpack(archive, destination):
    total = 0
    seen = set()
    with tarfile.open(archive, 'r:gz') as bundle:
        for item in bundle:
            path = PurePosixPath(item.name)
            if '..' in path.parts or path.is_absolute() or not (item.isfile() or item.isdir()):
                raise ValueError('Unsafe archive entry')
            name = str(path)
            if name == '.':
                continue
            if name not in {'zero-city', 'release.json', 'frontend'} and not name.startswith('frontend/'):
                raise ValueError('Unexpected release file')
            if name in seen:
                raise ValueError('Duplicate release file')
            seen.add(name)
            total += item.size
            if total > 1500 * 1024 * 1024 or len(seen) > 15000:
                raise ValueError('Release too large')
            target = destination.joinpath(*path.parts)
            if item.isdir():
                target.mkdir(parents=True, exist_ok=True)
            else:
                target.parent.mkdir(parents=True, exist_ok=True)
                with bundle.extractfile(item) as source, target.open('xb') as output:
                    shutil.copyfileobj(source, output)
                target.chmod(0o644)
    for name in ['zero-city', 'frontend/index.html', 'release.json']:
        if not (destination / name).is_file():
            raise ValueError('Incomplete release: ' + name)


def verify_release(meta):
    sha = meta.get('sha', '')
    run_id = meta.get('run_id')
    if not re.fullmatch('[a-f0-9]{40}', sha) or not isinstance(run_id, int) or isinstance(run_id, bool):
        raise ValueError('Invalid release identity')
    run = api('/actions/runs/' + str(run_id))
    if not (run['conclusion'] == 'success' and run['head_sha'] == sha and
            run['head_branch'] == 'main' and run['event'] in {'push', 'workflow_dispatch'} and
            run['path'] == '.github/workflows/checks.yml' and
            run['repository']['full_name'] == REPO):
        raise ValueError('Build did not pass trusted main checks')
    if api('/git/ref/heads/main')['object']['sha'] != sha:
        raise ValueError('Main advanced; refusing stale deployment')
    return sha


def switch(target):
    temporary = ROOT / 'current.next'
    temporary.unlink(missing_ok=True)
    temporary.symlink_to(target, target_is_directory=True)
    temporary.replace(ROOT / 'current')


def healthy():
    for _ in range(45):
        try:
            with urlopen('http://127.0.0.1:28090/health', timeout=3) as response:
                if json.load(response).get('status') == 'ok':
                    with urlopen('http://127.0.0.1:28090/api/v1/settings/public', timeout=5) as settings:
                        if json.load(settings).get('code') == 0:
                            return True
        except Exception:
            pass
        time.sleep(2)
    return False


def main():
    ROOT.mkdir(exist_ok=True)
    with (ROOT / 'deployment.lock').open('w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if shutil.disk_usage(ROOT).free < 2 * 1024 ** 3:
            raise RuntimeError('Insufficient staging space; active service unchanged')
        releases = ROOT / 'releases'
        releases.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(prefix='stage-', dir=releases) as temporary:
            stage = Path(temporary)
            archive = stage / 'payload.tgz'
            size = 0
            with archive.open('wb') as output:
                while chunk := sys.stdin.buffer.read(1024 * 1024):
                    size += len(chunk)
                    if size > 900 * 1024 * 1024:
                        raise ValueError('Payload too large')
                    output.write(chunk)
            extracted = stage / 'release'
            extracted.mkdir()
            unpack(archive, extracted)
            meta = json.loads((extracted / 'release.json').read_text())
            sha = verify_release(meta)
            (extracted / 'zero-city').chmod(0o755)
            meta['binary_sha256'] = hashlib.sha256((extracted / 'zero-city').read_bytes()).hexdigest()
            (extracted / 'frontend/community-build.json').write_text(json.dumps(meta))
            previous = (ROOT / 'current').resolve(strict=True)
            destination = releases / sha
            if destination.exists():
                if previous == destination and healthy():
                    print('Already deployed ' + sha)
                    return
                raise RuntimeError('Release already exists but is not active; inspect before retry')
            extracted.rename(destination)
            backup = ROOT / 'before-update.dump'
            with backup.open('wb') as output:
                backup.chmod(0o600)
                subprocess.run(['docker', 'exec', 'zero-city-community-pg', 'pg_dump', '-U', 'city',
                                '-d', 'city_community_demo', '-Fc'], stdout=output, check=True)
            # Files and launcher remain root-owned; app can write only its own data directory.
            try:
                switch(destination)
                subprocess.run(['systemctl', 'restart', SERVICE], check=True)
                if not healthy():
                    raise RuntimeError('New service health check failed')
            except Exception:
                subprocess.run(['systemctl', 'stop', SERVICE], check=True)
                with backup.open('rb') as restore:
                    subprocess.run(['docker', 'exec', '-i', 'zero-city-community-pg', 'pg_restore',
                                    '-U', 'city', '-d', 'city_community_demo', '--clean', '--if-exists',
                                    '--single-transaction'], stdin=restore, check=True)
                switch(previous)
                subprocess.run(['systemctl', 'restart', SERVICE], check=True)
                if not healthy():
                    raise RuntimeError('Deployment failed and prior service failed health check')
                raise RuntimeError('Deployment failed; prior executable and demo database restored')
            report = {'sha': sha, 'run_id': meta['run_id'], 'previous': str(previous), 'deployed_at': int(time.time())}
            (ROOT / 'deployment.json').write_text(json.dumps(report))
            for old in releases.iterdir():
                if (re.fullmatch('[a-f0-9]{40}', old.name) and not old.is_symlink() and old.is_dir()
                        and old.resolve().parent == releases.resolve() and old not in {destination, previous}):
                    shutil.rmtree(old)
            print(json.dumps(report))


if __name__ == '__main__':
    main()
