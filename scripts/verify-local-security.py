#!/usr/bin/env python3
"""Only loopback requests; no proxy, production URL, credential or business data."""
import http.client
import json
import socket
from pathlib import Path

rows = []

def request(port, path, headers=None):
    connection = http.client.HTTPConnection('127.0.0.1', port, timeout=30)
    connection.request('GET', path, headers=headers or {})
    response = connection.getresponse()
    status, response_headers = response.status, dict((k.lower(), v) for k, v in response.getheaders())
    response.read()
    connection.close()
    return status, response_headers

with socket.create_connection(('127.0.0.1', 16379), 3) as conn:
    conn.sendall(b'*1\r\n$4\r\nPING\r\n')
    result = conn.recv(4096).decode().strip()
rows.append(dict(label='redis_anonymous_ping', result=result, passed='NOAUTH' in result))
with socket.create_connection(('127.0.0.1', 4222), 3) as conn:
    conn.settimeout(4)
    info = conn.recv(8192).decode()
    conn.sendall(b'CONNECT {"verbose":false}\r\nPING\r\n')
    result = conn.recv(4096).decode().strip()
rows.append(dict(label='nats_anonymous_connect', auth_required='"auth_required":true' in info, result=result, passed='Authorization' in result or 'Authentication' in result))
for port, path in [(8000, '/share/v1/app/web/info'), (5173, '/login'), (3010, '/auth/login')]:
    status, headers = request(port, path)
    safe_headers = {key: headers.get(key) for key in ['content-security-policy', 'x-content-type-options', 'x-frame-options', 'referrer-policy', 'x-powered-by']}
    rows.append(dict(label=f'{port}_headers', status=status, headers=safe_headers, passed=status == 200 and safe_headers['x-content-type-options'] == 'nosniff' and bool(safe_headers['content-security-policy']) and safe_headers['x-powered-by'] is None))
    status, _ = request(port, path, {'Host': 'audit.invalid'})
    rows.append(dict(label=f'{port}_invalid_host', status=status, passed=status == 421))
for origin, wanted in [('https://audit.invalid', 403), ('http://localhost:5173', 200)]:
    status, headers = request(8000, '/share/v1/app/web/info', {'Origin': origin})
    allowed = headers.get('access-control-allow-origin')
    rows.append(dict(label='api_origin', origin=origin, status=status, allow_origin=allowed, passed=status == wanted and (allowed == origin if wanted == 200 else allowed is None)))
status, _ = request(8222, '/varz')
rows.append(dict(label='nats_monitor_local_boundary', status=status, passed=status == 200, note='监控仍无 HTTP 认证；仅绑定回环地址，不声称端点已增加认证。'))
output = Path(__file__).resolve().parents[1] / 'docs/security-review-20261022/修复后边界HTTP验证.json'
output.write_text(json.dumps(rows, ensure_ascii=False, indent=2) + '\n')
print(f'{sum(row["passed"] for row in rows)}/{len(rows)} local boundary checks passed')
assert all(row['passed'] for row in rows)
