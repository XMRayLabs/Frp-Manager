package workerd

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/Sakurame1/frp-manager/pb"
	"github.com/sourcegraph/conc"
)

func TestRunWorker(t *testing.T) {
	workerdBinPath, err := exec.LookPath("workerd")
	if err != nil {
		t.Skip("workerd binary is not installed")
	}

	workerdCWD := t.TempDir()
	workerID := "test"

	c := context.Background()
	defaultWorker := &pb.Worker{WorkerId: &workerID}
	FillWorkerValue(defaultWorker, 1)

	if err := GenCapnpConfig(c, workerdCWD, &pb.WorkerList{Workers: []*pb.Worker{defaultWorker}}); err != nil {
		t.Fatal(err)
	}

	var wg conc.WaitGroup

	wg.Go(func() {
		time.Sleep(10 * time.Second)
	})

	if err := WriteWorkerCodeToFile(c, defaultWorker, workerdCWD); err != nil {
		t.Fatal(err)
	}

	runner := NewExecManager(workerdBinPath,
		[]string{"serve", "--watch", "--verbose"})
	runner.RunCmd(workerID, WorkerCWDPath(c, defaultWorker, workerdCWD),
		[]string{ConfigFilePath(c, defaultWorker, workerdCWD)})

	defer runner.ExitAllCmd()

	wg.Wait()
}
