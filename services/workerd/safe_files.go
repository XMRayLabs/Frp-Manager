package workerd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
)

var workerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func ValidateWorker(worker *pb.Worker) error {
	if worker == nil || !workerIDPattern.MatchString(worker.GetWorkerId()) {
		return fmt.Errorf("invalid worker ID")
	}
	entry := worker.GetCodeEntry()
	if entry == "" || !filepath.IsLocal(entry) || strings.ContainsAny(entry, `\:`) {
		return fmt.Errorf("worker entry must be a relative path inside its code directory")
	}
	for _, part := range strings.Split(entry, "/") {
		if part == ".." || part == "." || part == "" {
			return fmt.Errorf("invalid worker entry path")
		}
	}
	return nil
}

// os.Root prevents both path traversal and symlink/junction escape during writes.
func writeWorkerFile(worker *pb.Worker, dir, name, content string) error {
	if err := ValidateWorker(worker); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	path := filepath.Join(defs.WorkerInfoPath, worker.GetWorkerId(), name)
	if err := root.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return root.WriteFile(path, []byte(content), 0600)
}
