package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
)

type SavedFile struct {
	Path     string  `json:"path"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Modified float64 `json:"modified"`
}

// Only public working files are compared; completed response snapshots are immutable.
const artifactWalk = `
def working_files():
 files={}
 for root,dirs,names in os.walk('/workspace',followlinks=False):
  dirs[:]=[d for d in dirs if not d.startswith('.') and not os.path.islink(os.path.join(root,d))]
  if root=='/workspace/agents': dirs[:]=[d for d in dirs if d==args['agent']]
  if root=='/workspace/agents/'+args['agent']: dirs[:]=[d for d in dirs if d!='responses']
  for name in names:
   p=os.path.join(root,name)
   if name.startswith('.') or os.path.islink(p) or not os.path.isfile(p):continue
   stat=os.stat(p)
   if stat.st_size>50*1024*1024:continue
   files[os.path.relpath(p,'/workspace')]=str(stat.st_size)+':'+str(stat.st_mtime_ns)
   if len(files)>=300:return files
 return files
`

func (m *Manager) WorkingFiles(ctx context.Context, id, agentID string) (map[string]string, error) {
	args, _ := json.Marshal(map[string]string{"agent": agentID})
	result, err := m.ExecPython(ctx, id, fmt.Sprintf("import os,json\nargs=json.loads(%q)\nos.makedirs('/workspace/agents/'+args['agent'],exist_ok=True)\n", string(args))+artifactWalk+"\nprint(json.dumps(working_files()))")
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("inspect files: %s", result.Stderr)
	}
	var files map[string]string
	err = json.Unmarshal([]byte(result.Stdout), &files)
	return files, err
}

func (m *Manager) SaveResponseFiles(ctx context.Context, id, agentID, turnID string, before map[string]string) ([]SavedFile, error) {
	args, _ := json.Marshal(map[string]interface{}{"agent": agentID, "turn": turnID, "before": before})
	code := fmt.Sprintf("import os,json,shutil\nargs=json.loads(%q)\n", string(args)) + artifactWalk + `
files=[]
total=0
prefix='agents/'+args['agent']+'/'
for relative,version in working_files().items():
 if args['before'].get(relative)==version:continue
 source=os.path.join('/workspace',relative)
 tail=relative[len(prefix):] if relative.startswith(prefix) else 'shared-output/'+relative
 destination='agents/'+args['agent']+'/responses/'+args['turn']+'/'+tail
 target=os.path.join('/workspace',destination)
 size=os.path.getsize(source)
 if total+size>200*1024*1024:continue
 os.makedirs(os.path.dirname(target),exist_ok=True)
 shutil.copy2(source,target)
 total+=size
 files.append({'path':destination,'name':os.path.basename(target),'size':size,'modified':os.path.getmtime(target)})
print(json.dumps(files))
`
	result, err := m.ExecPython(ctx, id, code)
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("save response files: %s", result.Stderr)
	}
	files := []SavedFile{}
	err = json.Unmarshal([]byte(result.Stdout), &files)
	return files, err
}
