package monitor

import (
	//"bytes"
	//"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	//"os"
	"strings"
	"sync"
	"time"

	ncron "github.com/niean/cron"
	pfc "github.com/niean/goperfcounter"
	nmap "github.com/niean/gotools/container/nmap"
	nhttpclient "github.com/niean/gotools/http/httpclient"
	ntime "github.com/niean/gotools/time"

	"github.com/niean/anteye/g"
	. "github.com/niean/anteye/modle"
	"github.com/niean/anteye/notice"
)

var (
	// cron
	monitorCron = ncron.New()
	cronSpec    = "30 * * * * ?"
	// cache
	statusCache = nmap.NewSafeMap()
	alarmCache  = nmap.NewSafeMap()
)

func GetMonitorOrPortcheck() map[string]string {
	checkCache := make(map[string]string)
	if len(statusCache.Keys()) == 0 {
		return checkCache
	}

	for _, key := range statusCache.Keys() {
		master := strings.Split(key, ",")[0]
		s, found := statusCache.Get(key)
		if !found {
			checkCache[master] = "err"
			continue
		}
		ss := s.(*Status)
		checkCache[master] = ss.Status
	}
	return checkCache
}

func Start() {
	notice.NoticeServer.StartWork()

	monitorCron.AddFuncCC(cronSpec, func() { monitor() }, 1)
	monitorCron.Start()
	go alarmJudge()
	log.Println("monitor.Start ok")
}

// alarm judge
func alarmJudge() {
	interval := time.Duration(10) * time.Second
	for {
		time.Sleep(interval)

		keys := alarmCache.Keys()
		if len(keys) == 0 {
			continue
		}

		for _, key := range keys {
			aitem, found := alarmCache.GetAndRemove(key)
			if !found {
				continue
			}
			notice.NoticeServer.SendAlert(aitem.(*Alarm))
		}

		/*
			for _, key := range keys {
				aitem, found := alarmCache.GetAndRemove(key)
				if !found {
					continue
				}
				content.WriteString(aitem.(*Alarm).String() + "\n")
			}

			if content.Len() < 6 {
				return
			}

			cfg := g.Config()
			// mail
			if cfg.Mail.Enable {
				hn, _ := os.Hostname()
				mailContent := formAlarmMailContent(cfg.Mail.Receivers, "Opsultra-anteye.From.["+hn+"]",
					content.String(), "AntEye")
				err := sendMail(cfg.Mail.Url, mailContent)
				if err != nil {
					log.Println("alarm send mail error, mail:", mailContent, "", err)
				} else {
					// statistics
					pfc.Meter("MonitorAlarmMail", 1)
				}
			}*/
	}
}

//func formAlarmMailContent(tos string, subject string, content string, from string) string {
//	return fmt.Sprintf("tos=%s&subject=%s&content=%s&user=%s", tos, subject, content, from)
//}
//
//func sendMail(mailUrl string, content string) error {
//	client := nhttpclient.GetHttpClient("monitor.mail", 5*time.Second, 10*time.Second)
//	// send by http-post
//	req, err := http.NewRequest("POST", mailUrl, bytes.NewBufferString(content))
//	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
//	req.Header.Set("Connection", "close")
//	postResp, err := client.Do(req)
//	if err != nil {
//		return err
//	}
//	defer postResp.Body.Close()
//
//	if postResp.StatusCode/100 != 2 {
//		return fmt.Errorf("Http-Post Error, Code %d", postResp.StatusCode)
//	}
//	return nil
//}
//
//func formAlarmSmsContent(tos string, content string, from string) string {
//	return fmt.Sprintf("tos=%s&content=%s&from=%s", tos, content, from)
//}
//
//func sendSms(smsUrl string, content string) error {
//	client := nhttpclient.GetHttpClient("monitor.sms", 5*time.Second, 10*time.Second)
//	// send by http-post
//	req, err := http.NewRequest("POST", smsUrl, bytes.NewBufferString(content))
//	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
//	req.Header.Set("Connection", "close")
//	postResp, err := client.Do(req)
//	if err != nil {
//		return err
//	}
//	defer postResp.Body.Close()
//
//	if postResp.StatusCode/100 != 2 {
//		return fmt.Errorf("Http-Post Error, Code %d", postResp.StatusCode)
//	}
//	return nil
//}
//
//func alarmCallback(caUrl string, content string) error {
//	client := nhttpclient.GetHttpClient("monitor.callback", 5*time.Second, 10*time.Second)
//	// send by http-post
//	req, err := http.NewRequest("POST", caUrl, bytes.NewBufferString(content))
//	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
//	req.Header.Set("Connection", "close")
//	postResp, err := client.Do(req)
//	if err != nil {
//		return err
//	}
//	defer postResp.Body.Close()
//
//	if postResp.StatusCode/100 != 2 {
//		return fmt.Errorf("Http-Post Error, Code %d", postResp.StatusCode)
//	}
//	return nil
//}

// status calc
func monitor() {
	startTs := time.Now().Unix()
	_monitor()
	endTs := time.Now().Unix()
	log.Printf("monitor, startTs %s, time-consuming %d sec\n", ntime.FormatTs(startTs), endTs-startTs)

	// statistics
	pfc.Meter("MonitorCronCnt", 1)
	pfc.Gauge("MonitorCronTs", endTs-startTs)
}

func checkReach(proto, addr string) bool {
	c, err := net.DialTimeout(proto, addr, time.Duration(10)*time.Second)
	if err == nil {
		c.Close()
		return true
	} else {
		return false
	}
}
func _monitor() {
	client := nhttpclient.GetHttpClient("monitor.get", 5*time.Second, 10*time.Second)
	for _, host := range g.Config().Monitor.Cluster {
		hostInfo := strings.Split(host, ",") // "module,hostname:port/health/monitor/url"
		if len(hostInfo) != 2 {
			continue
		}
		//hostType := hostInfo[0]
		hostUrl := hostInfo[1]
		if !strings.Contains(hostUrl, "http://") {
			hostUrl = "http://" + hostUrl
		}

		req, _ := http.NewRequest("GET", hostUrl, nil)
		req.Header.Set("Connection", "close")
		getResp, err := client.Do(req)
		if err != nil {
			log.Printf(host+", monitor error,", err)
			onMonitorErr(host)
			continue
		}
		defer getResp.Body.Close()

		body, err := ioutil.ReadAll(getResp.Body)                        // body=['o','k',...]
		if !(err == nil && len(body) >= 2 && string(body[:2]) == "ok") { // err
			log.Println(host, ", error,", err)
			onMonitorErr(host)
		} else { // get "ok"
			onMonitorOk(host)
		}
	}

	for _, portcheck := range g.Config().Monitor.PortCheck {
		portinfo := strings.Split(portcheck, ",")
		if len(portinfo) != 3 {
			continue
		}
		porturl := portinfo[1]
		if !strings.Contains(porturl, ":") {
			porturl = porturl + ":80"
		}
		proto := portinfo[2]
		if proto != "tcp" && proto != "udp" {
			continue
		}

		istcpConn := checkReach(proto, porturl)
		if istcpConn {
			onMonitorOk(portcheck)
		} else {
			onMonitorErr(portcheck)
		}
	}
}

func onMonitorErr(host string) {
	// change status
	s, found := statusCache.Get(host)
	if !found {
		s = NewStatus()
		statusCache.Put(host, s)
	}
	ss := s.(*Status)
	ss.OnErr()

	// alarm
	errCnt := ss.GetErrCnt()
	if errCnt >= 4 && errCnt <= 16 {
		for i := 4; i <= errCnt; i *= 2 {
			if errCnt == i {
				a := NewAlarm(host, "err", ss.GetErrCnt())
				alarmCache.Put(host, a)
				break
			}
		}
	}
}

func onMonitorOk(host string) {
	// change status
	s, found := statusCache.Get(host)
	if !found {
		s = NewStatus()
		statusCache.Put(host, s)
	}
	ss := s.(*Status)
	errCnt := ss.GetErrCnt()
	ss.OnOk()

	if ss.IsTurnToOk() {
		if errCnt >= 4 { //有过alarm, 才能turnOk
			// alarm
			a := NewAlarm(host, "ok", ss.GetErrCnt())
			alarmCache.Put(host, a)
		}
	}
}

// Status Struct
type Status struct {
	sync.RWMutex
	Status     string
	LastStatus string
	ErrCnt     int
	OkCnt      int
}

func NewStatus() *Status {
	return &Status{Status: "ok", LastStatus: "ok", ErrCnt: 0, OkCnt: 0}
}

func (s *Status) GetErrCnt() int {
	s.RLock()
	cnt := s.ErrCnt
	s.RUnlock()
	return cnt
}

func (s *Status) OnErr() {
	s.Lock()
	s.LastStatus = s.Status
	s.Status = "err"
	s.OkCnt = 0
	s.ErrCnt += 1
	s.Unlock()
}

func (s *Status) OnOk() {
	s.Lock()
	s.LastStatus = s.Status
	s.Status = "ok"
	s.OkCnt += 1
	s.ErrCnt = 0
	s.Unlock()
}

func (s *Status) IsTurnToOk() bool {
	s.RLock()
	ret := false
	if s.LastStatus == "err" && s.Status == "ok" {
		ret = true
	}
	s.RUnlock()
	return ret
}

func NewAlarm(obj string, atype string, cnt int) *Alarm {
	return &Alarm{AlarmType: atype, ObjName: obj, AlarmCnt: cnt, Ts: time.Now().Unix()}
}
