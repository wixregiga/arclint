import json,pathlib,subprocess,tempfile,time,sys
binary=pathlib.Path('/tmp/arclint-workflow')
out=pathlib.Path('/tmp/arclint-workflow-hook-proof');out.mkdir(exist_ok=True)
root=pathlib.Path(tempfile.mkdtemp(prefix='arclint-workflow-adopter-'))
(root/'src').mkdir()
(root/'AGENTS.md').write_text('Run arclint context for affected paths before reading source. Explain and record changed domain meanings before implementation. Reuse existing domain concepts. Examine actual enforcement and reassess repairs. Do not claim checks that were not run.\n')
(root/'rules.arclint.yaml').write_text('runtime: [ts]\nzones:\n  billing:\n    paths: src/**\nrules: {}\n')
(root/'domain.arclint.yaml').write_text('version: 1\nproject: invoices\ncontexts:\n  billing:\n    definition: An Invoice has a total and a customer billing email. Sending a receipt does not change the total.\n')
original='type Invoice = { total: number; billingEmail: string };\nexport function sendReceipt(invoice: Invoice, email: (address: string) => void) { }\n'
good='type Invoice = { total: number; billingEmail: string };\nexport function sendReceipt(invoice: Invoice, email: (address: string) => void) { email(invoice.billingEmail); }\n'
bad='type Invoice = { total: number; billingEmail: string };\ntype VIPCustomer = { tier: "gold" };\nexport function sendReceipt(invoice: Invoice, email: (address: string) => void) { invoice.total *= 0.9; email(invoice.billingEmail); }\n'
(root/'src/invoice.ts').write_text(original)
def run(args,**kw): return subprocess.run(args,cwd=root,text=True,capture_output=True,check=True,**kw)
run(['git','init','-q']);run(['git','add','AGENTS.md','domain.arclint.yaml','rules.arclint.yaml','src/invoice.ts']);run(['git','-c','user.name=ArcLint fixture','-c','user.email=fixture@example.invalid','commit','-qm','Initial fixture'])
install=run([str(binary),'agents','workflow','install','--format','json'])
(out/'install.json').write_text(install.stdout)
(out/'root.txt').write_text(str(root))
exe=root/'.codex/hooks/arclint-workflow-guard/arclint'
sid='task-focused-proof-'+str(time.time_ns());history=[]
def event(stage,**fields):
 payload={'hook_event_name':stage,'session_id':sid,'cwd':str(root),**fields}
 result=run([str(exe),'agents','workflow','event'],input=json.dumps(payload),timeout=85)
 response=json.loads(result.stdout)
 history.append({'input':payload,'output':response})
 (out/'activity.json').write_text(json.dumps(history,indent=2)+'\n')
 print(stage,json.dumps(response)[:450],flush=True)
 return response
try:
 event('SessionStart')
 event('UserPromptSubmit',prompt='Update src/invoice.ts receipt delivery to email the existing billingEmail. Do not change pricing or introduce customer categories. Follow the project domain workflow.')
 context=run([str(exe),'context','src/invoice.ts'])
 event('PostToolUse',tool_name='exec_command',tool_input={'cmd':'arclint context src/invoice.ts'},tool_response={'exit_code':0,'output':context.stdout})
 patch='*** Begin Patch\n*** Update File: src/invoice.ts\n@@\n-'+original.replace('\n','\n-').rstrip('-')+'+'+bad.replace('\n','\n+').rstrip('+')+'*** End Patch\n'
 proposed=event('PreToolUse',tool_name='apply_patch',tool_input={'input':patch})
 assert 'VIP' in json.dumps(proposed) or 'pricing' in json.dumps(proposed).lower(), 'did not report unsupported proposal'
 (root/'src/invoice.ts').write_text(bad)
 event('PostToolUse',tool_name='apply_patch',tool_input={'input':patch},tool_response='Applied the proposed TypeScript change.')
 event('UserPromptSubmit',prompt='Repair the reported departures: remove the VIP category and discount mutation. Keep the receipt email change using existing billingEmail. Reassess the corrected result; do not claim unrun checks.')
 repair='*** Begin Patch\n*** Update File: src/invoice.ts\n@@\n-'+bad.replace('\n','\n-').rstrip('-')+'+'+good.replace('\n','\n+').rstrip('+')+'*** End Patch\n'
 event('PreToolUse',tool_name='apply_patch',tool_input={'input':repair})
 (root/'src/invoice.ts').write_text(good)
 event('PostToolUse',tool_name='apply_patch',tool_input={'input':repair},tool_response='Removed the unrequested category and price mutation; receipt uses billingEmail.')
 final=event('Stop',last_assistant_message='The receipt uses existing billingEmail. Removed the unrequested VIP category and price mutation. No typecheck or functional-test result is claimed.')
 result={'status':'completed','final_feedback':final}
except Exception as exc:
 result={'status':'failed','error':str(exc)}
finally:
 result.update(root=str(root),binary=str(binary),session=sid,evidence_kind='Real installed public CLI hook subprocesses and real model; fixture mutations by harness, not native outer-host dispatch.')
 (out/'manifest.json').write_text(json.dumps(result,indent=2)+'\n')
 print(json.dumps(result),flush=True)
if result['status']!='completed':sys.exit(1)
