package cheroapi

import(
	"net"
	"time"
	"fmt"

	"google.golang.org/grpc"
	"github.com/go-co-op/gocron"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
)

type Server interface {
	pbApi.CrudCheropatillaServer
	QA()
}

func New(s Server) *App {
	return &App{
		srv: s,
	}
}

type App struct {
	srv Server
}

func (a *App) Run() error {
	lis, err := net.Listen("tcp", "0.0.0.0:50051")
	if err != nil {
		return fmt.Errorf("Failed to listen: %v\n", err)
	}
	s := grpc.NewServer()

	pbApi.RegisterCrudCheropatillaServer(s, a.srv.(pbApi.CrudCheropatillaServer))
	
	// Run the Quality Assurance on the databases every day.
	QAscheduler := gocron.NewScheduler(time.UTC)
	QAscheduler.Every(1).Day().Do(a.srv.QA)
	QAscheduler.StartAsync()

	return s.Serve(lis)
}
