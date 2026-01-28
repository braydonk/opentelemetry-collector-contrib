package semconvtest_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestWeaver(t *testing.T) {
	weaver, err := semconvtest.NewWeaverContext(t.Context(), semconvtest.NewDefaultWeaverOptions())
	require.NoError(t, err)

	logs := plog.NewLogs()
	res := logs.ResourceLogs().AppendEmpty()
	res.Resource().Attributes().PutStr("something", "value")
	scope := res.ScopeLogs().AppendEmpty()
	record := scope.LogRecords().AppendEmpty()
	record.Body().SetStr("hi I am a log")

	time.Sleep(10 * time.Second)

	wLogs, _ := weaver.ContainerLogs()
	for _, line := range wLogs {
		fmt.Println(line)
	}

	err = weaver.TestLogs(logs)
	require.NoError(t, err)

	time.Sleep(8 * time.Second)

	wLogs, _ = weaver.ContainerLogs()
	for _, line := range wLogs {
		fmt.Println(line)
	}

	// err = weaver.Shutdown()
	// require.NoError(t, err)
}
