package modle

import (
	"fmt"
	ntime "github.com/niean/gotools/time"
)

type Alarm struct {
	ObjName   string
	AlarmType string
	AlarmCnt  int
	Ts        int64
}

func (a *Alarm) String() string {
	return fmt.Sprintf("[%s][%s][%d][%s]", ntime.FormatTs(a.Ts), a.AlarmType, a.AlarmCnt, a.ObjName)
}
