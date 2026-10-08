package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"mime"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/grok-agent/backend/internal/config"
	"github.com/grok-agent/backend/internal/sandbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FileHandler struct {
	db  *pgxpool.Pool
	cfg *config.Config
}

func NewFileHandler(db *pgxpool.Pool, cfg *config.Config) *FileHandler { return &FileHandler{db, cfg} }
func (h *FileHandler) computer(c *gin.Context) (*sandbox.Manager, string, bool) {
	var id string
	if _, err := uuid.Parse(c.Param("id")); err != nil {
		c.JSON(400, gin.H{"error": "invalid agent"})
		return nil, "", false
	}
	err := h.db.QueryRow(c.Request.Context(), `SELECT COALESCE(sandbox_id,'') FROM agents WHERE id=$1 AND user_id=$2`, c.Param("id"), c.GetString("user_id")).Scan(&id)
	if err != nil {
		c.JSON(404, gin.H{"error": "agent unavailable"})
		return nil, "", false
	}
	if id == "" {
		c.JSON(409, gin.H{"error": "Start a conversation to access this computer's files."})
		return nil, "", false
	}
	mgr, err := sandbox.NewManager(h.cfg.SandboxImage, h.cfg.WorkspacePath)
	if err != nil {
		c.JSON(503, gin.H{"error": "computer unavailable"})
		return nil, "", false
	}
	// File views and downloads are explicit requests to wake the computer.
	key, err := mgr.ComputerKey(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
	if err == nil {
		var info *sandbox.SandboxInfo
		info, err = mgr.GetOrCreate(c.Request.Context(), key)
		if err == nil {
			id = info.ContainerID
			_, err = h.db.Exec(c.Request.Context(), `UPDATE agents SET sandbox_id=$1,novnc_port=$2,vnc_port=$3,status='running' WHERE id=$4 AND user_id=$5`, id, info.NoVNCPort, info.VNCPort, c.Param("id"), c.GetString("user_id"))
			_, _ = h.db.Exec(c.Request.Context(), `UPDATE users SET computer_last_active=NOW() WHERE id=$1`, c.GetString("user_id"))
		}
	}
	if err != nil {
		mgr.Close()
		c.JSON(503, gin.H{"error": "could not wake computer"})
		return nil, "", false
	}
	return mgr, id, true
}

func (h *FileHandler) List(c *gin.Context) {
	mgr, id, ok := h.computer(c)
	if !ok {
		return
	}
	defer mgr.Close()
	result, err := mgr.ExecPython(c.Request.Context(), id, fmt.Sprintf(`import os,json
os.makedirs('/workspace/agents/%s',exist_ok=True)
files=[]
for root,dirs,names in os.walk('/workspace/agents/%s',followlinks=False):
 dirs[:]=[d for d in dirs if not d.startswith('.') and not os.path.islink(os.path.join(root,d))]
 for name in names:
  p=os.path.join(root,name)
  if name.startswith('.') or os.path.islink(p) or not os.path.isfile(p): continue
  size=os.path.getsize(p)
  if size>50*1024*1024: continue
  files.append({'path':os.path.relpath(p,'/workspace'),'name':name,'size':size,'modified':os.path.getmtime(p)})
  if len(files)>=300: break
 if len(files)>=300: break
print(json.dumps(sorted(files,key=lambda f:f['modified'],reverse=True)))`, c.Param("id"), c.Param("id")))
	if err != nil || result.ExitCode != 0 {
		c.JSON(409, gin.H{"error": "Computer is asleep. Send a message to reconnect."})
		return
	}
	var files []json.RawMessage
	if json.Unmarshal([]byte(result.Stdout), &files) != nil {
		c.JSON(500, gin.H{"error": "could not list files"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, files)
}

func (h *FileHandler) Download(c *gin.Context) {
	name, err := sandbox.WorkspacePath(c.Query("path"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	mgr, id, ok := h.computer(c)
	if !ok {
		return
	}
	defer mgr.Close()
	if !strings.HasPrefix(name, "/workspace/agents/"+c.Param("id")+"/") {
		c.JSON(400, gin.H{"error": "file must be in this agent's folder"})
		return
	}
	encoded, _ := json.Marshal(name)
	// Resolve links in the container before asking Docker to copy the file.
	result, err := mgr.ExecPython(c.Request.Context(), id, fmt.Sprintf(`import os,json
p=os.path.realpath(json.loads(%q))
relative=os.path.relpath(p,'/workspace')
assert not any(part.startswith('.') for part in relative.split('/')) and p.startswith('/workspace/agents/%s/'), 'path escapes agent folder'
assert os.path.isfile(p) and os.path.getsize(p)<=50*1024*1024, 'file unavailable or exceeds 50 MB'
print(p)`, string(encoded), c.Param("id")))
	if err != nil || result.ExitCode != 0 {
		c.JSON(404, gin.H{"error": "file unavailable"})
		return
	}
	canonical := strings.TrimSpace(result.Stdout)
	err = mgr.CopyWorkspaceFile(c.Request.Context(), id, canonical, func(reader io.Reader, size int64) error {
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(name)}))
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.DataFromReader(200, size, "application/octet-stream", reader, nil)
		return nil
	})
	if err != nil && !c.Writer.Written() {
		c.JSON(404, gin.H{"error": "could not download file"})
	}
}
