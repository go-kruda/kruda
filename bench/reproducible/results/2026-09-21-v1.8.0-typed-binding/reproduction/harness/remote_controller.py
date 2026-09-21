import contextlib,fcntl,json,os,pathlib,secrets,signal,subprocess,sys,time
ROOT=pathlib.Path('/home/tiger/kruda180-balanced-20260921')
STATE=ROOT/'service-state.json'
UNITS=['actions.runner.tiinno-learn-platform.dev-server.service','forgejo-runner.service','caddy.service','cloudflared.service']
TIMER='kruda180-balanced-restore-20260921'
TOKEN='KRUDA180_CONTROLLER_TOKEN'

def run(args,check=True,timeout=180):
 return subprocess.run(args,text=True,capture_output=True,check=check,timeout=timeout)
def save(value):
 p=STATE.with_suffix('.tmp');p.write_text(json.dumps(value,indent=2)+'\n');p.replace(STATE)
def emit(phase,**values):
 print(json.dumps(dict(phase=phase,utc=time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),**values)),flush=True)
def containers():
 ids=run(['docker','ps','-q']).stdout.split()
 if not ids:return []
 data=json.loads(run(['docker','inspect',*ids]).stdout)
 return [dict(id=c['Id'],name=c['Name'].lstrip('/'),status=c['State']['Status'],health=c['State'].get('Health',{}).get('Status')) for c in data]
def workers():
 return [p.name for p in pathlib.Path('/proc').iterdir() if p.name.isdigit() and read_comm(p)=='Runner.Worker']
def read_comm(p):
 try:return (p/'comm').read_text().strip()
 except (FileNotFoundError,ProcessLookupError,PermissionError):return ''
def process_info(pid):
 try:
  fields=pathlib.Path('/proc',str(pid),'stat').read_text().rsplit(')',1)[1].split()
 except (FileNotFoundError,ProcessLookupError):return None
 return dict(pid=pid,state=fields[0],pgid=int(fields[2]),sid=int(fields[3]),start_ticks=int(fields[19]))
def measurement_members(measurement):
 pgid=measurement['pgid']
 if pgid<=1 or measurement['pid']!=pgid:raise RuntimeError('Invalid measurement process group')
 leader=process_info(measurement['pid'])
 if leader and leader['start_ticks']!=measurement['start_ticks']:
  raise RuntimeError('Measurement process identity changed; refusing group signal')
 members=[]
 for proc in pathlib.Path('/proc').iterdir():
  if not proc.name.isdigit():continue
  info=process_info(int(proc.name))
  if not info or info['pgid']!=pgid or info['state'] in ('Z','X'):continue
  try:environment=(proc/'environ').read_bytes().split(b'\0')
  except (FileNotFoundError,ProcessLookupError):continue
  if info['sid']!=pgid or (TOKEN+'='+measurement['token']).encode() not in environment:
   raise RuntimeError('Unverified measurement group member '+proc.name)
  members.append(info['pid'])
 return members
def stop_measurement(state):
 measurement=state.get('measurement')
 if not measurement:return
 for signum,seconds in ((signal.SIGTERM,15),(signal.SIGKILL,5)):
  if not measurement_members(measurement):return
  try:os.killpg(measurement['pgid'],signum)
  except ProcessLookupError:pass
  deadline=time.monotonic()+seconds
  while measurement_members(measurement):
   if time.monotonic()>=deadline:break
   time.sleep(.1)
  else:return
 if measurement_members(measurement):raise RuntimeError('Owned measurement processes survived cleanup')
@contextlib.contextmanager
def isolated_state():
 with (ROOT/'restore.lock').open('a+') as f:
  fcntl.flock(f,fcntl.LOCK_EX)
  state=json.loads(STATE.read_text())
  if state.get('restoration_started') or state.get('restored'):
   raise RuntimeError('Restoration has fenced the controller; isolation cannot resume')
  yield state
def measure(log):
 with isolated_state() as state:
  token=secrets.token_hex(24)
  process=subprocess.Popen(['/usr/bin/python3',str(ROOT/'harness/remote_controller.py'),'measure'],
   cwd=ROOT,stdin=subprocess.PIPE,stdout=log,stderr=subprocess.STDOUT,start_new_session=True,
   env=dict(os.environ,**{TOKEN:token}))
  try:
   identity=process_info(process.pid)
   if not identity or identity['pgid']!=process.pid or identity['sid']!=process.pid:
    raise RuntimeError('Measurement did not acquire its own session')
   state['measurement']=dict(pid=process.pid,pgid=identity['pgid'],start_ticks=identity['start_ticks'],token=token)
   save(state)
   # The driver cannot spawn workloads until fallback cleanup knows its identity.
   process.stdin.write(b'1');process.stdin.close()
  except BaseException:
   process.stdin.close()
   try:process.wait(timeout=3)
   except subprocess.TimeoutExpired:
    process.kill();process.wait(timeout=3)
   raise
 try:return process.wait(timeout=2400)
 except BaseException:
  with (ROOT/'restore.lock').open('a+') as f:
   fcntl.flock(f,fcntl.LOCK_EX)
   stop_measurement(json.loads(STATE.read_text()))
   process.wait(timeout=3)
  raise
def restore():
 with (ROOT/'restore.lock').open('a+') as f:
  fcntl.flock(f,fcntl.LOCK_EX)
  if not STATE.exists():return
  state=json.loads(STATE.read_text())
  if state.get('restored'):return
  state['restoration_started']=True;save(state)
  errors={}
  try:stop_measurement(state)
  except Exception as error:
   state['restore']={'errors':{'measurement':str(error)}}
   save(state);emit('restore_failed',**state['restore'])
   raise RuntimeError('Measurement cleanup failed; services remain paused') from error
  ids=[c['id'] for c in state.get('containers',[])]
  if ids:
   try:
    result=run(['docker','start',*ids],check=False)
    (ROOT/'restore-containers.log').write_text(result.stdout+result.stderr)
    if result.returncode:errors['containers_start']=result.stderr or 'exit '+str(result.returncode)
   except Exception as error:errors['containers_start']=str(error)
  deadline=time.monotonic()+150
  bad=[c['name'] for c in state.get('containers',[])]
  while ids:
   try:current={c['id']:c for c in containers()}
   except Exception as error:
    errors['containers_status']=str(error);break
   bad=[c['name'] for c in state.get('containers',[]) if c['id'] not in current or (c['health']=='healthy' and current[c['id']]['health']!='healthy')]
   if not bad or time.monotonic()>=deadline:break
   time.sleep(3)
  unit_results={}
  for unit in state.get('units',[]):
   try:
    result=run(['sudo','-n','systemctl','start',unit],check=False)
    if result.returncode:errors[unit+'_start']=result.stderr or 'exit '+str(result.returncode)
   except Exception as error:errors[unit+'_start']=str(error)
   try:
    result=run(['systemctl','is-active',unit],check=False)
    unit_results[unit]=result.stdout.strip()
    if result.returncode:errors[unit+'_status']=result.stderr or 'exit '+str(result.returncode)
   except Exception as error:
    unit_results[unit]='unknown';errors[unit+'_status']=str(error)
  state['restore']={'containers_missing_or_not_healthy':bad,'units':unit_results,'errors':errors,'completed_utc':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime())}
  state['restored']=not errors and not bad and all(v=='active' for v in unit_results.values())
  save(state);emit('restored',**state['restore'])
  if not state['restored']:raise RuntimeError('Restoration needs attention: '+json.dumps(state['restore']))

def main():
 ROOT.mkdir(exist_ok=True)
 with open('/home/tiger/kruda-typed-comparison-20260912/benchmark.lock','r+') as lock:
  fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
  fcntl.flock(lock,fcntl.LOCK_UN)
  busy=workers()
  if busy:raise RuntimeError('CI job still active; nothing stopped: '+str(busy))
  with (ROOT/'restore.lock').open('a+') as f:
   fcntl.flock(f,fcntl.LOCK_EX)
   if STATE.exists() and not json.loads(STATE.read_text()).get('restored'):
    raise RuntimeError('Previous isolation is not restored; preserving its service snapshot')
   if run(['systemctl','is-active',TIMER+'.timer'],check=False).returncode==0:
    raise RuntimeError('Previous restoration timer is still active; nothing changed')
   state={'host':os.uname().nodename,'pid':os.getpid(),'units':[u for u in UNITS if run(['systemctl','is-active',u],check=False).returncode==0],'containers':[],'restored':False}
   save(state)
  try:
   run(['sudo','-n','systemd-run','--unit='+TIMER,'--on-active=55m','--timer-property=AccuracySec=1s','--uid=tiger','/usr/bin/python3',str(ROOT/'harness/remote_controller.py'),'restore'])
   with isolated_state() as state:
    for unit in state['units']:run(['sudo','-n','systemctl','stop',unit])
   emit('runners_paused',units=state['units'])
   deadline=time.monotonic()+480
   while not (ROOT/'ready.json').exists():
    with isolated_state():pass
    if time.monotonic()>deadline:raise TimeoutError('Build/verification readiness exceeded8min')
    time.sleep(1)
   ready=json.loads((ROOT/'ready.json').read_text())
   with isolated_state() as state:
    state['ready']=ready
    state['containers']=containers();save(state)
    if state['containers']:run(['docker','stop','--time','20',*[c['id'] for c in state['containers']]])
   emit('containers_paused',count=len(state['containers']))
   if containers():raise RuntimeError('New or remaining containers prevent isolation')
   time.sleep(12)
   (ROOT/'environment.txt').write_text(run(['uname','-a']).stdout+run(['lscpu']).stdout+run(['free','-m']).stdout+run(['uptime']).stdout)
   emit('measurement_start')
   with (ROOT/'native-progress.jsonl').open('w') as log:
    returncode=measure(log)
   if returncode:raise RuntimeError('Native driver failed; inspect native-progress.jsonl')
   emit('measurement_complete')
  finally:
   signal.signal(signal.SIGTERM,signal.SIG_IGN)
   signal.signal(signal.SIGINT,signal.SIG_IGN)
   restore()
   run(['sudo','-n','systemctl','stop',TIMER+'.timer'],check=False)

if __name__=='__main__':
 if len(sys.argv)>1 and sys.argv[1]=='restore':restore()
 elif len(sys.argv)>1 and sys.argv[1]=='measure':
  if sys.stdin.buffer.read(1)!=b'1':raise RuntimeError('Measurement launch was not authorized')
  os.execl('/usr/bin/python3','/usr/bin/python3',str(ROOT/'harness/run_full.py'))
 else:
  def interrupted(signum,frame):raise RuntimeError('Controller interrupted by signal '+str(signum))
  signal.signal(signal.SIGTERM,interrupted)
  signal.signal(signal.SIGINT,interrupted)
  main()
