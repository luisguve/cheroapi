package server

import(
	"log"
	"errors"

	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
)

// Post upvote on thread, comment or subcomment
func (s *Server) Upvote(req *pbApi.UpvoteRequest, stream pbApi.CrudCheropatilla_UpvoteServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		submitter = req.UserId
		notifyUsers []*pbApi.NotifyUsers
		err error
		sendErr error
	)
	// call a different upvote method, depending upon the context where the
	// upvote is being submitted.
	switch ctx := req.ContentContext.(type) {
	case *pbApi.UpvoteRequest_ThreadCtx: // THREAD
		notifyUser, err := s.dbHandler.UpvoteThread(submitter, ctx.ThreadCtx)
		if err == nil {
			notifyUsers = append(notifyUsers, notifyUser)
		}
	case *pbApi.UpvoteRequest_CommentCtx: // COMMENT
		notifyUsers, err = s.dbHandler.UpvoteComment(submitter, ctx.CommentCtx)
	case *pbApi.UpvoteRequest_SubcommentCtx: // SUBCOMMENT
		notifyUsers, err = s.dbHandler.UpvoteSubcomment(submitter, ctx.SubcommentCtx)
	}
	if err != nil {
		log.Printf("Could not submit upvote: %v\n", err)
		if errors.Is(err, ErrThreadNotFound) {
			return status.Errorf(codes.NotFound, "Content %s not found", req.ContentContext)
		}
		return status.Error(codes.Internal, "Proto marshal/unmarshal error")
	}
	for _, notifyUser := range notifyUsers {
		if sendErr = stream.Send(notifyUser); sendErr != nil {
			log.Printf("Could not send NotifyUser: %v\n", sendErr)
			return status.Error(codes.Internal,	sendErr.Error())
		}
	}
	return nil
}

// Undo an upvote on a thread, comment or subcomment
func (s *Server) UndoUpvote(ctx context.Context, req *pbApi.UndoUpvoteRequest) (*pbApi.UndoUpvoteResponse, error) {

}

// Post comment on a thread or in a comment
func (s *Server) Comment(req *pbApi.CommentRequest, stream pbApi.CrudCheropatilla_CommentServer) error {
	
}