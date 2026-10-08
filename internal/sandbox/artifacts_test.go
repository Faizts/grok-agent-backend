package sandbox

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestResponseSnapshotsPreserveEarlierFiles(t *testing.T) {
	id := os.Getenv("TEST_SANDBOX_ID")
	if id == "" {
		t.Skip("set TEST_SANDBOX_ID for Docker integration")
	}
	mgr, err := NewManager("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	ctx := context.Background()
	agent := fmt.Sprintf("review-%d", time.Now().UnixNano())
	root := "/workspace/agents/" + agent
	defer mgr.ExecPython(ctx, id, fmt.Sprintf("import shutil;shutil.rmtree(%q,ignore_errors=True)", root))
	before, err := mgr.WorkingFiles(ctx, id, agent)
	if err != nil {
		t.Fatal(err)
	}
	res, err := mgr.ExecPython(ctx, id, fmt.Sprintf("from pathlib import Path;Path(%q).write_text('first version')", root+"/image.txt"))
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("fixture: %v %#v", err, res)
	}
	files, err := mgr.SaveResponseFiles(ctx, id, agent, "first-turn", before)
	if err != nil {
		t.Fatal(err)
	}
	var saved string
	for _, f := range files {
		if f.Name == "image.txt" {
			saved = f.Path
		}
	}
	if saved == "" {
		t.Fatal("changed file was not attached")
	}
	res, err = mgr.ExecPython(ctx, id, fmt.Sprintf("from pathlib import Path;Path(%q).write_text('new version');print(Path(%q).read_text())", root+"/image.txt", "/workspace/"+saved))
	if err != nil || res.ExitCode != 0 || res.Stdout != "first version\n" {
		t.Fatalf("snapshot overwritten: %v %#v", err, res)
	}
	baseline, err := mgr.WorkingFiles(ctx, id, agent)
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := mgr.SaveResponseFiles(ctx, id, agent, "next-turn", baseline)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range unchanged {
		if f.Name == "image.txt" {
			t.Fatal("unchanged file repeated in next response")
		}
	}
}
