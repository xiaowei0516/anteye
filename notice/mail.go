package notice

import (
	"crypto/tls"
	"github.com/niean/anteye/g"
	"github.com/niean/anteye/modle"
	"gopkg.in/gomail.v2"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type MailNoticeServer struct {
	mailChan chan *MailMessage
	stopChan chan bool
}

var NoticeServer = &MailNoticeServer{}

//GetMailDialer 获取邮箱服务器代理
func (e *MailNoticeServer) GetMailDialer() *gomail.Dialer {
	cfg := g.Config().Mail
	d := gomail.NewDialer(cfg.MailServer, cfg.MailPort, cfg.MailUser, cfg.MailPassword)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: cfg.MailPort == 465}
	return d
}

type MailMessage struct {
	message  *gomail.Message
	errCount int
	alert    *modle.Alarm
}

func (e *MailNoticeServer) GetMessage(body string, subject string, receiver ...string) *MailMessage {
	m := gomail.NewMessage()
	m.SetHeader("From", g.Config().Mail.MailFrom)

	m.SetHeader("To", receiver...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)
	return &MailMessage{
		message:  m,
		errCount: 0,
	}
}

//创建邮件内容
func (e *MailNoticeServer) GetBody(alert *modle.Alarm) string {
	path, _ := filepath.Abs("views/mail.html")
	buffer, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln("get mail tmeplate error")
	}
	mail := string(buffer)
	mail = strings.Replace(mail, "[TITLE]", string(alert.ObjName+alert.AlarmType), -1)
	//mail = strings.Replace(mail, "[URL]", string(), -1)
	mail = strings.Replace(mail, "[DESCRIPTION]", string(alert.String()), -1)
	return mail
}

func (e *MailNoticeServer) GetMessageByAlert(alert *modle.Alarm) (messages []*MailMessage) {
	receivers := g.Config().Mail.MailReceivers
	hn, _ := os.Hostname()
	toaddr := strings.Split(receivers, ",")
	m := e.GetMessage(e.GetBody(alert), "Opsultra-eye from "+hn+strconv.Itoa(alert.AlarmCnt), toaddr...)
	messages = append(messages, m)
	return
}

func (e *MailNoticeServer) SendAlert(alert *modle.Alarm) error {
	messages := e.GetMessageByAlert(alert)
	if len(messages) > 0 {
		for _, m := range messages {
			e.mailChan <- m
		}
	}
	return nil
}
