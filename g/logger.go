package g

import (
	"fmt"
	"os"

	"github.com/toolkits/pkg/logger"
)

type LoggerSection struct {
	Dir       string `json:"dir"`
	Level     string `json:"level"`
	KeepHours uint   `json:"keepHours"`
}

// stdoutBackend mirrors logger's unexported stdBackend so docker logs can see app output.
type stdoutBackend struct{}

func (b *stdoutBackend) Log(_ logger.Severity, msg []byte) {
	_, _ = os.Stdout.Write(msg)
}

func (b *stdoutBackend) Close() {}

func InitLog(l LoggerSection) {
	lb, err := logger.NewFileBackend(l.Dir)
	if err != nil {
		fmt.Println("cannot init logger:", err)
		os.Exit(1)
	}
	lb.SetRotateByHour(true)
	lb.SetKeepHours(l.KeepHours)

	mb, err := logger.NewMultiBackend(lb, &stdoutBackend{})
	if err != nil {
		fmt.Println("cannot init logger multi backend:", err)
		os.Exit(1)
	}

	logger.SetLogging(l.Level, mb)
}
