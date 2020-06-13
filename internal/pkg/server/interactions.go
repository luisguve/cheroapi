package server

import(
	
)

// Post upvote on thread, comment or subcomment
func (s *Server) Upvote(req *pbApi.UpvoteRequest, 
	stream pbApi.CrudCheropatilla_UpvoteServer) error {
	
}

// Undo an upvote on a thread, comment or subcomment
func (s *Server) UndoUpvote(ctx context.Context,
	req *pbApi.UndoUpvoteRequest) (*pbApi.UndoUpvoteResponse, error) {

}

// Post comment on a thread or in a comment
func (s *Server) Comment(req *pbApi.CommentRequest,
	stream pbApi.CrudCheropatilla_CommentServer) error {
	
}