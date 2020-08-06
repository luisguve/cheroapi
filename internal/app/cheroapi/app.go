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
			}).Error("Could not open log file. Writing to stderr.")
		}
		summary, err := a.srv.QA()
		if err != nil {
			logger.WithFields(log.Fields{
				"package":  "cheroapi",
				"file":     "app.go",
				"function": "scheduled QA",
			}).Error("QA returned the following error:", err)
			return
		}
		if summary == "" {
			logger.WithFields(log.Fields{
				"package":  "cheroapi",
				"file":     "app.go",
				"function": "scheduled QA",
			}).Error("QA returned the empty summary")
			return
		}
		logger.WithFields(log.Fields{
			"package":  "cheroapi",
			"file":     "app.go",
			"function": "scheduled QA",
		}).Info("Result of QA:", summary)
	})
	QAscheduler.StartAsync()

	_, nextQA := QAscheduler.NextRun()
	now := time.Now()
	diff := nextQA.Sub(now)
	hoursLeft := int(diff.Hours())
	minutesLeft := int(diff.Minutes()) - (hoursLeft * 60)
	secondsLeft := int(diff.Seconds()) - (hoursLeft * 60 * 60) - (minutesLeft * 60)

	defaultLog.Printf("Next QA: %v (in %v hours, %v minutes, %v seconds)",
		nextQA.Format(time.RFC822), hoursLeft, minutesLeft, secondsLeft)
}

func (a *App) Run(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("Failed to listen: %v\n", err)
	}
	s := grpc.NewServer()

	pbApi.RegisterCrudCheropatillaServer(s, a.srv.(pbApi.CrudCheropatillaServer))

	// Uncomment this line to turn on the daily QA on the sections.
	// a.scheduleQA()
	defaultLog.Println("Running")
	return s.Serve(lis)
}
