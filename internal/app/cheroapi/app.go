package cheroapi

import (
	"fmt"
	defaultLog "log"
	"net"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/go-co-op/gocron"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	"google.golang.org/grpc"
)

type Server interface {
	pbApi.CrudCheropatillaServer
	QA() (string, error)
}

func New(s Server, logPath string) *App {
	return &App{
		logFile: filepath.Join(logPath, "QA.log"),
		srv:     s,
	}
}

type App struct {
	logFile string
	srv     Server
}

func (a *App) scheduleQA() {
	// Run the Quality Assurance on the databases every day.
	QAscheduler := gocron.NewScheduler(time.UTC)
	QAscheduler.Every(1).Day().Do(func() {
		defaultLog.Println("Starting QA")

		logger := log.New()
		logFile, err := os.OpenFile(a.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			logger.SetOutput(logFile)
			defer logFile.Close()
		} else {
			logger.WithFields(log.Fields{
				"package":  "cheroapi",
				"file":     "app.go",
				"function": "scheduled QA",
			}).Errorf("Could not open log file: %v. Writing to stderr.\n", err)
		}
		summary, err := a.srv.QA()
		if err != nil {
			logger.WithFields(log.Fields{
				"package":  "cheroapi",
				"file":     "app.go",
				"function": "scheduled QA",
			}).Errorf("QA returned error: %v\n", err)
			return
		}
		if summary == "" {
			logger.WithFields(log.Fields{
				"package":  "cheroapi",
				"file":     "app.go",
				"function": "scheduled QA",
			}).Error("QA returned an empty summary")
			return
		}
		logger.WithFields(log.Fields{
			"package":  "cheroapi",
			"file":     "app.go",
			"function": "scheduled QA",
		}).Infof("Result of QA: %v\n", summary)

		defaultLog.Println("Finished QA")
	})
	// QAscheduler.StartAt(time.Now().Add(10 * time.Second))
	QAscheduler.StartAsync()

	_, nextQA := QAscheduler.NextRun()
	now := time.Now()
	diff := nextQA.Sub(now)
	hoursLeft := int(diff.Hours())
	minutesLeft := int(diff.Minutes()) - (hoursLeft * 60)
	secondsLeft := int(diff.Seconds()) - (hoursLeft * 60 * 60) - (minutesLeft * 60)

	defaultLog.Printf("Next QA: %v (in %v hours, %v minutes, %v seconds)",
		nextQA.Format(time.RubyDate), hoursLeft, minutesLeft, secondsLeft)
}

func (a *App) Run(addr string, doQA bool) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("Failed to listen: %v\n", err)
	}
	s := grpc.NewServer()

	pbApi.RegisterCrudCheropatillaServer(s, a.srv.(pbApi.CrudCheropatillaServer))

	if doQA {
		a.scheduleQA()
	}
	defaultLog.Println("Running")
	return s.Serve(lis)
}
