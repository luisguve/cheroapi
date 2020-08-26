package contents

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Get a single thread
func (s *Server) GetThread(ctx context.Context, req *pbApi.GetThreadRequest) (*pbApi.ContentData, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	contentRule, err := s.dbHandler.GetThread(req.Thread)
	if err != nil {
		if errors.Is(err, dbmodel.ErrSectionNotFound) ||
			errors.Is(err, dbmodel.ErrThreadNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return contentRule.Data, nil
}

// Get a comment's comments
func (s *Server) GetSubcomments(req *pbApi.GetSubcommentsRequest, stream pbApi.CrudCheropatilla_GetSubcommentsServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		err          error
		sendErr      error
		ctx          = req.CommentCtx
		offset       = int(req.Offset)
		contentRules []*pbApi.ContentRule
	)
	contentRules, err = s.dbHandler.GetSubcomments(ctx, offset)
	if err != nil {
		if errors.Is(err, dbmodel.ErrSectionNotFound) ||
			errors.Is(err, dbmodel.ErrThreadNotFound) ||
			errors.Is(err, dbmodel.ErrCommentNotFound) {
			return status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, dbmodel.ErrOffsetOutOfRange) {
			return status.Error(codes.OutOfRange, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	for _, contentRule := range contentRules {
		if sendErr = stream.Send(contentRule); err != nil {
			log.Printf("Could not send Content Rule: %v\n", sendErr)
			return status.Error(codes.Internal, sendErr.Error())
		}
	}
	return nil
}

// Get metadata of contents along with their context.
func (s *Server) GetActivityOverview(req *pbDataFormat.Activity, stream pbApi.CrudCheropatilla_GetActivityOverviewServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		errs     []error
		m        sync.Mutex
		wg       sync.WaitGroup
		contents []*pbApi.ContentOverview
	)
	// Set threads from the recent activity.
	for _, ctx := range req.ThreadsCreated {
		wg.Add(1)
		go func(ctx *pbContext.Thread) {
			defer wg.Done()
			pbContent, err := s.dbHandler.GetThreadContent(ctx)
			m.Lock()
			defer m.Unlock()
			if err != nil {
				// failed to get thread metadata; append err to errs
				// and return
				errs = append(errs, err)
				return
			}
			overview := &pbApi.ContentOverview{
				Metadata: pbContent.Metadata,
				Context:  &pbContext.Context{
					Ctx: &pbContext.Context_ThreadCtx{
						ThreadCtx: ctx,
					},
					SectionId: pbContent.SectionId,
				},
			}
			contents = append(contents, overview)
		}(ctx)
	}
	// Set comments from the recent activity.
	for _, ctx := range req.Comments {
		wg.Add(1)
		go func(ctx *pbContext.Comment) {
			defer wg.Done()
			pbContent, err := s.dbHandler.GetCommentContent(ctx)
			m.Lock()
			defer m.Unlock()
			if err != nil {
				// failed to get comment metadata; append err to errs
				// and return
				errs = append(errs, err)
				return
			}
			overview := &pbApi.ContentOverview{
				Metadata: pbContent.Metadata,
				Context:  &pbContext.Context{
					Ctx: &pbContext.Context_CommentCtx{
						CommentCtx: ctx,
					},
					SectionId: pbContent.SectionId,
				},
			}
			contents = append(contents, overview)
		}(ctx)
	}
	// set subcomments from the recent activity of this user
	for _, ctx := range req.Subcomments {
		wg.Add(1)
		go func(ctx *pbContext.Subcomment) {
			defer wg.Done()
			pbContent, err := s.dbHandler.GetSubcommentContent(ctx)
			m.Lock()
			defer m.Unlock()
			if err != nil {
				// failed to get subcomment metadata; append err to
				// errs and return
				errs = append(errs, err)
				return
			}
			overview := &pbApi.ContentOverview{
				Metadata: pbContent.Metadata,
				Context:  &pbContext.Context{
					Ctx: &pbContext.Context_SubcommentCtx{
						SubcommentCtx: ctx,
					},
					SectionId: pbContent.SectionId,
				},
			}
			contents = append(contents, overview)
		}(ctx)
	}
	wg.Wait()

	// Check for errors.
	if len(errs) != 0 {
		// Set the first error.
		errMsg := fmt.Sprintf("Error 1: %v\n", errs[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(errs); i++ {
			errMsg += fmt.Sprintf("Error %d: %v\n", i+1, errs[i].Error())
		}
		return status.Error(codes.Internal, errMsg)
	}

	for _, overview := range contents {
		if sendErr := stream.Send(overview); sendErr != nil {
			log.Printf("Could not send Content Overview: %v\n", sendErr)
			return status.Error(codes.Internal, sendErr.Error())
		}
	}
	return nil
}

func (s *Server) GetContentsByContext(req *pbApi.GetContentsByContextRequest, stream pbApi.CrudCheropatilla_GetContentsByContextServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		contentRules = make([]*pbApi.ContentRule, len(req.Contents))
		errs         []error
		m            sync.Mutex
		wg           sync.WaitGroup
	)

	for idx, content := range req.Contents {
		wg.Add(1)
		go func(idx int, context *pbContext.Context, status string) {
			defer wg.Done()
			var (
				contentRule *pbApi.ContentRule
				err         error
			)
			switch ctx := context.Ctx.(type) {
			case *pbContext.Context_ThreadCtx:
				contentRule, err = s.dbHandler.GetThread(ctx.ThreadCtx)
				if err != nil {
					contentRule = &pbApi.ContentRule{}
					log.Printf("Could not get thread: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
				}
			case *pbContext.Context_CommentCtx:
				contentRule, err = s.dbHandler.GetComment(ctx.CommentCtx)
				if err != nil {
					contentRule = &pbApi.ContentRule{}
					log.Printf("Could not get comment: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
				}
			case *pbContext.Context_SubcommentCtx:
				contentRule, err = s.dbHandler.GetSubcomment(ctx.SubcommentCtx)
				if err != nil {
					contentRule = &pbApi.ContentRule{}
					log.Printf("Could not get subcomment: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
				}
			}
			contentRule.Status = status
			contentRules[idx] = contentRule
		}(idx, content.Key, content.Status)
	}
	wg.Wait()
	for _, contentRule := range contentRules {
		if sendErr := stream.Send(contentRule); sendErr != nil {
			log.Printf("Could not send Content Rule: %v\n", sendErr)
			return status.Error(codes.Internal, sendErr.Error())
		}
	}
	if len(errs) != 0 {
		// Set the first error.
		errMsg := fmt.Sprintf("Error 1: %v\n", errs[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(errs); i++ {
			errMsg += fmt.Sprintf("Error %d: %v\n", i+1, errs[i].Error())
		}
		return status.Error(codes.Internal, errMsg)
	}
	return nil
}
