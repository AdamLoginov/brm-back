import json
import pathlib
import sqlite3
import tarfile
import tempfile
import urllib.request
import urllib.error

base = 'http://127.0.0.1:8080/api'
with tempfile.TemporaryDirectory() as directory:
    original = pathlib.Path(directory) / 'original.db'
    with tarfile.open('/root/brm-data-20261113.tar.gz') as archive:
        original.write_bytes(archive.extractfile('test.db').read())
    with sqlite3.connect(original) as database:
        login, password = database.execute('SELECT login,password FROM users WHERE deleted_at IS NULL LIMIT 1').fetchone()
    request = urllib.request.Request(base+'/login', data=json.dumps({'login':login, 'password':password}).encode(), headers={'Content-Type':'application/json'})
    with urllib.request.urlopen(request) as response:
        token = json.load(response)['token']
    print('Existing user login: OK')
    request = urllib.request.Request(base+'/agreements', headers={'Authorization':'Bearer '+token})
    with urllib.request.urlopen(request) as response:
        json.load(response)
        print('Authenticated agreements API:', response.status)
    try:
        urllib.request.urlopen(base+'/attachment/1')
        raise RuntimeError('Attachment is accessible without authentication')
    except urllib.error.HTTPError as error:
        assert error.code == 401, error.code
        print('Unauthenticated attachments: blocked')
with sqlite3.connect('/var/lib/brm/test.db') as database:
    assert database.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
    print('SQLite integrity: OK')
    print('Users:', database.execute('SELECT count(*) FROM users').fetchone()[0])
print('Upload files:', sum(path.is_file() for path in pathlib.Path('/var/lib/brm/uploads').rglob('*')))
