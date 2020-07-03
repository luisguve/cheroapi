package server

import(
	"log"
	"errors"

	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
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
		if (err == nil) && (notifyUser != nil) {
			notifyUsers = append(notifyUsers, notifyUser)
		}
	case *pbApi.UpvoteRequest_CommentCtx: // COMMENT
		notifyUsers, err = s.dbHandler.UpvoteComment(submitter, ctx.CommentCtx)
	case *pbApi.UpvoteRequest_SubcommentCtx: // SUBCOMMENT
		notifyUsers, err = s.dbHandler.UpvoteSubcomment(submitter, ctx.SubcommentCtx)
	}
	if err != nil {
		if (errors.Is(err, dbmodel.ErrSectionNotFound)) || 
		(errors.Is(err, dbmodel.ErrThreadNotFound)) || 
		(errors.Is(err, dbmodel.ErrCommentNotFound)) || 
		(errors.Is(err, dbmodel.ErrSubcommentNotFound)) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
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
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		submitter = req.UserId
		err error
	)
	// call a different undo upvote method, depending upon the context where the
	// upvote undoing is being submitted.
	switch ctx := req.ContentContext.(type) {
	case *pbApi.UndoUpvoteRequest_ThreadCtx: // THREAD
		err = s.dbHandler.UndoUpvoteThread(submitter, ctx.ThreadCtx)
	case *pbApi.UndoUpvoteRequest_CommentCtx: // COMMENT
		err = s.dbHandler.UndoUpvoteComment(submitter, ctx.CommentCtx)
	case *pbApi.UndoUpvoteRequest_SubcommentCtx: // SUBCOMMENT
		err = s.dbHandler.UndoUpvoteSubcomment(submitter, ctx.SubcommentCtx)
	}
	if err != nil {
		if errors.Is(err, dbmodel.ErrNotUpvoted) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if (errors.Is(err, dbmodel.ErrSectionNotFound)) || 
		(errors.Is(err, dbmodel.ErrThreadNotFound)) || 
		(errors.Is(err, dbmodel.ErrCommentNotFound)) || 
		(errors.Is(err, dbmodel.ErrSubcommentNotFound)) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.UndoUpvoteResponse{}, nil
}

// Post comment on a thread or in a comment
func (s *Server) Comment(req *pbApi.CommentRequest, stream pbApi.CrudCheropatilla_CommentServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		submitter = req.UserId
		notifyUsers []*pbApi.NotifyUsers
		err error
		sendErr error
	)
	reply := dbmodel.Reply{
		Content: req.Content,
		FtFile: req.FtFile,
		Submitter: req.UserId,
		PublishDate: req.PublishDate,
	}
	// call a different comment method, depending upon the context where the
	// comment is being submitted.
	switch ctx := req.ContentContext.(type) {
	case *pbApi.CommentRequest_ThreadCtx: // THREAD
		notifyUser, err := s.dbHandler.ReplyThread(ctx.ThreadCtx, reply)
		if (err == nil) && (notifyUser != nil) {
			notifyUsers = append(notifyUsers, notifyUser)
		}
	case *pbApi.CommentRequest_CommentCtx: // COMMENT
		notifyUsers, err = s.dbHandler.ReplyComment(ctx.CommentCtx, reply)
	}
	if err != nil {
		if (errors.Is(err, dbmodel.ErrSectionNotFound)) || 
		(errors.Is(err, dbmodel.ErrThreadNotFound)) || 
		(errors.Is(err, dbmodel.ErrCommentNotFound)) || 
		(errors.Is(err, dbmodel.ErrSubcommentNotFound)) {
			return status.Error(codes.NotFound, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	for _, notifyUser := range notifyUsers {
		if sendErr = stream.Send(notifyUser); sendErr != nil {
			log.Printf("Could not send NotifyUser: %v\n", sendErr)
			return status.Error(codes.Internal,	sendErr.Error())
		}
	}
	return nil
}
