package notice

import (
	"github.com/niean/anteye/g"
	"gopkg.in/gomail.v2"
	"log"
	"time"
)

func (e *MailNoticeServer) StartWork() (err error) {
	log.Println("mail  server init start")
	defer log.Println("mail server init over")
	cfg := g.Config().Mail
	mailCount := cfg.MailCount
	mailReCount := cfg.MailReCount
	if e.mailChan == nil {
		e.mailChan = make(chan *MailMessage, mailCount)
	}
	if e.stopChan == nil {
		e.stopChan = make(chan bool)
	}
	go func() {
		d := e.GetMailDialer()
		var s gomail.SendCloser
		var err error
		open := false
		for {
			select {
			case m, ok := <-e.mailChan:
				if !ok {
					return
				}
				if !open {
					if s, err = d.Dial(); err != nil {
						log.Println("Get mail dial error." + err.Error())
					}
					open = true
				}
				if err := gomail.Send(s, m.message); err != nil {
					log.Println("send mail message error." + err.Error())
					m.errCount++
					if m.errCount < mailReCount {
						//5秒后重试
						log.Printf("mail errCount:", m.errCount)
						go func(m *MailMessage) {
							time.Sleep(time.Second * 5)
							e.mailChan <- m
						}(m)
					}
				}
			case stop := <-e.stopChan:
				if stop {
					goto exit
				}
			// Close the connection to the SMTP server if no email was sent in
			// the last 30 seconds.
			case <-time.After(30 * time.Second):
				if open {
					if err := s.Close(); err != nil {
						panic(err)
					}
					open = false
				}
			}
		}
	exit:
		log.Println("mail work stop success")
	}()
	log.Println("mail notice server start success")

	return
}

func (e *MailNoticeServer) StopWork() error {
	if e.stopChan != nil {
		e.stopChan <- true
		close(e.stopChan)
	}
	if e.mailChan != nil {
		close(e.mailChan)
	}
	return nil
}
