"""Run installed hook subprocesses with the real semantic reviewer in a fixture.

Usage: python3 probe_hook.py /absolute/arclint /absolute/new-evidence-directory
Uses model access. Does not activate/trust native host hooks or alter existing installs.
The harness proposes each repair through PreToolUse before mutating fixture files.
"""
import hashlib
import json
import pathlib
import shlex
import subprocess
import sys
import tempfile
import time

binary = pathlib.Path(sys.argv[1]).resolve()
out = pathlib.Path(sys.argv[2]).resolve()
out.mkdir(parents=True, exist_ok=False)
root = pathlib.Path(tempfile.mkdtemp(prefix='arclint-live-hook-'))
(root / 'src').mkdir()
recording = 'version: 1\nproject: rooms\ncontexts:\n  booking:\n    definition: A Room is a named bookable space.\n'
good = 'package booking\n\ntype Room struct { Name string }\n'
bad = 'package booking\n\n// This is the Room value.\ntype Room struct { Name string }\n'
(root / 'domain.arclint.yaml').write_text(recording)
(root / 'src/room.go').write_text(bad)
install = subprocess.run([str(binary), 'agents', 'hooks', '--host', 'codex', '--domain-source', 'src/room.go', '--format', 'json'], cwd=root, capture_output=True, text=True)
(out / 'install.stdout').write_text(install.stdout)
(out / 'install.stderr').write_text(install.stderr)
if install.returncode:
    raise RuntimeError('installation failed')
hooks = json.loads((root / '.codex/hooks.json').read_text())['hooks']
history = []
session = 'arclint-live-repair-' + str(time.time_ns())

def invoke(event, **payload):
    data = {'hook_event_name': event, 'session_id': session, 'cwd': str(root), **payload}
    command = shlex.split(hooks[event][0]['hooks'][0]['command'])
    start = time.time()
    result = subprocess.run(command, cwd=root, input=json.dumps(data), capture_output=True, text=True, timeout=85)
    item = {'event': data, 'command': command, 'exit_code': result.returncode,
            'stdout': result.stdout, 'stderr': result.stderr, 'seconds': round(time.time()-start, 2)}
    cache = root / '.arclint/cache/codex-domain-guard'
    states = list(cache.rglob('*.json'))
    item['states'] = {str(p.relative_to(cache)): json.loads(p.read_text()) for p in states}
    history.append(item)
    (out / 'activity.json').write_text(json.dumps(history, indent=2)+'\n')
    if result.returncode:
        raise RuntimeError('hook process failed: ' + result.stderr)
    response = json.loads(result.stdout)
    print(event, response.get('decision', response.get('hookSpecificOutput', {}).get('permissionDecision', 'returned')), flush=True)
    return response

def allow(response):
    if response.get('continue') is False or response.get('decision') == 'block' or response.get('hookSpecificOutput', {}).get('permissionDecision') == 'deny':
        raise RuntimeError('repair or fresh review was rejected: '+json.dumps(response))
    event = history[-1]['event']
    key = hashlib.sha256(json.dumps(session, sort_keys=True).encode()).hexdigest() + '.json'
    matches = [value for path, value in history[-1]['states'].items() if pathlib.Path(path).name == key]
    state = matches[0] if len(matches) == 1 else None
    if event['hook_event_name'] == 'Stop' and (not state or state.get('verdict', {}).get('status') != 'passed'):
        raise RuntimeError('completion returned without a fresh passing verdict')

def blocked(response):
    if response.get('continue') is not False and response.get('decision') != 'block' and response.get('hookSpecificOutput', {}).get('permissionDecision') != 'deny':
        raise RuntimeError('expected protection was not observed')

try:
    invoke('SessionStart')
    invoke('UserPromptSubmit', prompt='Preserve the existing Room meaning and Go type. Remove the explanatory source comment to comply with the configured domain-source comment policy. If src/room.go becomes missing, restore exactly that Room implementation without the comment. Do not change rules, configuration, scope or other files.')
    blocked(invoke('Stop', last_assistant_message='The Room implementation still contains its explanatory source comment.'))
    patch = '*** Begin Patch\n*** Update File: src/room.go\n@@\n-// This is the Room value.\n type Room struct { Name string }\n*** End Patch\n'
    allow(invoke('PreToolUse', tool_name='apply_patch', tool_input={'input': patch}))
    (root / 'src/room.go').write_text(good)
    allow(invoke('PostToolUse', tool_name='apply_patch', tool_input={'input': patch}, tool_response='Removed only the explanatory comment.'))
    allow(invoke('Stop', last_assistant_message='Removed the explanatory comment. The existing Room value and its name are unchanged. No functional or architecture checks are claimed.'))
    (root / 'src/room.go').unlink()
    blocked(invoke('Stop', last_assistant_message='src/room.go is missing and requires restoration.'))
    addition = '*** Begin Patch\n*** Add File: src/room.go\n' + ''.join('+'+line+'\n' for line in good.splitlines()) + '*** End Patch\n'
    allow(invoke('PreToolUse', tool_name='apply_patch', tool_input={'input': addition}))
    (root / 'src/room.go').write_text(good)
    allow(invoke('PostToolUse', tool_name='apply_patch', tool_input={'input': addition}, tool_response='Restored the scoped Room file.'))
    allow(invoke('Stop', last_assistant_message='Restored src/room.go with the same Room type and Name field, without the explanatory comment. No functional or architecture checks are claimed.'))
    (root / 'rules.arclint.yaml').write_text('version: 1\n')
    blocked(invoke('PreToolUse', tool_name='apply_patch', tool_input={'input': addition}))
    allow(invoke('PreToolUse', tool_name='get_goal', tool_input={}))
    blocked(invoke('Stop', last_assistant_message='The rules file changed; review approval is invalid.'))
    (root / 'rules.arclint.yaml').unlink()
    session += '-fresh'
    invoke('SessionStart')
    invoke('UserPromptSubmit', prompt='Review the existing Room recording and implementation. The original rules bytes have been restored. Do not edit anything or claim unrun checks.')
    allow(invoke('Stop', last_assistant_message='The Room recording and implementation are unchanged; no unrun checks are claimed.'))
    result = 'passed'
except Exception as error:
    result = 'failed: ' + str(error)
finally:
    manifest = {'result': result, 'fixture': str(root), 'binary': str(binary),
                'source_commit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip(),
                'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
                'guard_sha256': hashlib.sha256((root / '.codex/hooks/arclint-domain-guard/guard.py').read_bytes()).hexdigest(),
                'codex_version': subprocess.check_output(['codex', '--version'], text=True).strip(),
                'evidence_kind': 'Installed hook subprocess protocol using production semantic reviewer; mutations orchestrated by harness. Not native outer-host activation or autonomous agent execution.'}
    (out / 'manifest.json').write_text(json.dumps(manifest, indent=2)+'\n')
    print(json.dumps(manifest, indent=2), flush=True)
if result != 'passed':
    sys.exit(1)
