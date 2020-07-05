package app

import(
	"net"

	"google.golang.org/grpc"
	"github.com/go-co-op/gocron"
	"github.com/luisguve/cheroapi/internal/pkg/server"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
)

func New(s server.Server) *App {
	return &App{
		srv: s,
	}
}

type App struct {
	srv server.Server
}

func (a *App) Run() error {
	lis, err := net.Listen("tcp", "0.0.0.0:50051")
	if err != nil {
		return fmt.Errorf("Failed to listen: %v\n", err)
	}
	s := grpc.NewServer()

	pbApi.RegisterCrudCheropatillaServer(s, a.srv)
	QAscheduler := gocron.NewScheduler(time.UTC)
	// Quality Assurance the databases every day.
	QAscheduler.Every(1).Day().Do(s.srv.QA())
	QAscheduler.StartAsync()
	return s.Serve(lis)
}
