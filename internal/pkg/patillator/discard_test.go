package patillator_test

import (
	"fmt"
	"time"
	"testing"

	pbTime "github.com/golang/protobuf/ptypes/timestamp"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
)

func TestDiscardActivity(t *testing.T) {
	result := patillator.DiscardActivities(activities, nil)
	if len(result.ThreadsCreated) != len(activities.ThreadsCreated) ||
	len(result.Comments) != len(activities.Comments) ||
	len(result.Subcomments) != len(activities.Subcomments) {
		t.Errorf("result should have the same activities, this is the result: %v\n", printActivities(t, result))
	}
	result = patillator.DiscardActivities(activities, activityIds)
	if len(result.ThreadsCreated) != 0 || len(result.Comments) != 0 ||
	len(result.Subcomments) != 0 {
		t.Errorf("result should have no activities, these were left: %v\n", printActivities(t, result))
	}
}

func TestDiscardContents(t *testing.T) {
	result := patillator.DiscardContents(contents, nil)
	if len(result) != len(contents) {
		t.Errorf("DiscardContents should have returned the same list of contents. This was returned: %v\n", result)
	}
	result = patillator.DiscardContents(contents, ids)
	if len(result) != (len(contents) - len(ids)) {
		t.Errorf("DiscardContents should have returned a result with 5 contents. This was returned: %v\n", result)
	}
	// Check whether the result set have only even posts.
	for i := 0; i < len(result); i++ {
		for {
			key, ok := result[i].Key().(string)
			if !ok {
				t.Errorf("Key should be string, got instead: %v\n", result[i].Key())
				break
			}
			if key == "post-02" || key == "post-04" || key == "post-06" ||
			key == "post-08" || key == "post-10" {
				// Remove content.
				last := len(result) - 1
				result[i] = result[last]
				result = result[:last]
				if len(result) == 0 {
					break
				}
			}
		}
	}
	if len(result) != 0 {
		t.Errorf("result should have a length of 0. These were left: %v\n", printResult(t, result))
	}
}

func printActivities(t *testing.T, result patillator.UserActivity) string {
	var s string
	s += "Threads created:\n"
	for _, tc := range result.ThreadsCreated {
		ta, ok := tc.(patillator.ThreadActivity)
		if !ok {
			t.Errorf("Failed type assertion to thread activitiy: %t\n", tc)
		}
		s += fmt.Sprintf("%+v\n", ta.Thread)
	}
	s += "Comments:\n"
	for _, c := range result.Comments {
		ca, ok := c.(patillator.CommentActivity)
		if !ok {
			t.Errorf("Failed type assertion to thread activitiy: %t\n", c)
		}
		s += fmt.Sprintf("%+v\n", ca.Comment)
	}
	s += "Subcomments:\n"
	for _, sc := range result.Subcomments {
		sca, ok := sc.(patillator.SubcommentActivity)
		if !ok {
			t.Errorf("Failed type assertion to thread activitiy: %t\n", sc)
		}
		s += fmt.Sprintf("%+v\n", sca.Subcomment)
	}
	return s
}

func printResult(t *testing.T, result []patillator.SegregateFinder) string {
	var s string
	for _, r := range result {
		c, ok := r.(patillator.Content)
		if !ok {
			t.Errorf("Failed type assertion to patillator.Content: %t\n", r)
			 continue
		}
		s += c.DataKey + " "
	}
	return s
}

// Ids of contents to be discarded.
var ids = []string{
	"post-01",
	"post-03",
	"post-05",
	"post-07",
	"post-09",
}

var contents = []patillator.SegregateDiscarderFinder{
	patillator.Content(pbMetadata.Content{
		Interactions:  uint32(12),
		LastUpdated:   &pbTime.Timestamp{
			Seconds:   time.Now().Add(-5 * time.Minute).Unix(),
		},
		AvgUpdateTime: (2 * time.Minute).Seconds(),
		DataKey:       "post-01",
	}),
	patillator.Content(pbMetadata.Content{
		Interactions:  uint32(7),
		LastUpdated:   &pbTime.Timestamp{
			Seconds:   time.Now().Add(-8 * time.Minute).Unix(),
		},
		AvgUpdateTime: (14 * time.Minute).Seconds(),
		DataKey:       "post-02",
	}),
	patillator.Content(pbMetadata.Content{
		Interactions:  uint32(25),
		LastUpdated:   &pbTime.Timestamp{
			Seconds:   time.Now().Add(-3 * time.Minute).Unix(),
		},
		AvgUpdateTime: (4 * time.Minute).Seconds(),
		DataKey:       "post-03",
	}),
	patillator.Content(pbMetadata.Content{
		Interactions:  uint32(2),
		LastUpdated:   &pbTime.Timestamp{
			Seconds:   time.Now().Add(-17 * time.Minute).Unix(),
		},
		AvgUpdateTime: (35 * time.Minute).Seconds(),
		DataKey:       "post-04",
	}),
	patillator.Content(pbMetadata.Content{
		Interactions:  uint32(257),
		LastUpdated:   &pbTime.Timestamp{
			Seconds:   time.Now().Add(-16 * time.Minute).Unix(),
		},
		AvgUpdateTime: (55 * time.Minute).Seconds(),
		DataKey:       "post-05",
	}),
	patillator.Content(pbMetadata.Content{
		Interactions:  uint32(1),
		LastUpdated:   &pbTime.Timestamp{
			Seconds:   time.Now().Add(-24 * time.Minute).Unix(),
		},
		AvgUpdateTime: (24 * time.Minute).Seconds(),
		DataKey:       "post-06",
	}),
	patillator.Content(pbMetadata.Content{
		Interactions:  uint32(0),
		LastUpdated:   &pbTime.Timestamp{
			Seconds:   time.Now().Add(-12 * time.Minute).Unix(),
		},
		AvgUpdateTime: (0 * time.Minute).Seconds(),
		DataKey:       "post-07",
	}),
	patillator.Content(pbMetadata.Content{
		Interactions:  uint32(174),
		LastUpdated:   &pbTime.Timestamp{
			Seconds:   time.Now().Add(-6 * time.Minute).Unix(),
		},
		AvgUpdateTime: (4 * time.Minute).Seconds(),
		DataKey:       "post-08",
	}),
	patillator.Content(pbMetadata.Content{
		Interactions:  uint32(73),
		LastUpdated:  &pbTime.Timestamp{
			Seconds:   time.Now().Add(-9 * time.Minute).Unix(),
		},
		AvgUpdateTime: (3 * time.Minute).Seconds(),
		DataKey:       "post-09",
	}),
	patillator.Content(pbMetadata.Content{
		Interactions:  uint32(33),
		LastUpdated:   &pbTime.Timestamp{
			Seconds:   time.Now().Add(-4 * time.Minute).Unix(),
		},
		AvgUpdateTime: (25 * time.Minute).Seconds(),
		DataKey:       "post-10",
	}),
}

// Ids of activities to be discarded.
var activityIds = &pbDataFormat.Activity{
	ThreadsCreated: []*pbContext.Thread{
		&pbContext.Thread{
			Id: "post-01",
			SectionCtx: &pbContext.Section{
				Id: "mylife",
			},
		},
		&pbContext.Thread{
			Id: "post-02",
			SectionCtx: &pbContext.Section{
				Id: "mylife",
			},
		},
		&pbContext.Thread{
			Id: "post-666",
			SectionCtx: &pbContext.Section{
				Id: "mylife",
			},
		},
	},
	Comments: []*pbContext.Comment{
		&pbContext.Comment{
			Id: "comment-01",
			ThreadCtx: &pbContext.Thread{
				Id: "post-01",
				SectionCtx: &pbContext.Section{
					Id: "mylife",
				},
			},
		},
		&pbContext.Comment{
			Id: "comment-02",
			ThreadCtx: &pbContext.Thread{
				Id: "post-01",
				SectionCtx: &pbContext.Section{
					Id: "mylife",
				},
			},
		},
	},
	Subcomments: []*pbContext.Subcomment{
		&pbContext.Subcomment{
			Id: "subcomment-01",
			CommentCtx: &pbContext.Comment{
				Id:        "comment-01",
				ThreadCtx: &pbContext.Thread{
					Id: "post-01",
					SectionCtx: &pbContext.Section{
						Id: "mylife",
					},
				},
			},
		},
		&pbContext.Subcomment{
			Id: "subcomment-02",
			CommentCtx: &pbContext.Comment{
				Id:        "comment-01",
				ThreadCtx: &pbContext.Thread{
					Id: "post-01",
					SectionCtx: &pbContext.Section{
						Id: "mylife",
					},
				},
			},
		},
	},
}

var activities = patillator.UserActivity{
	ThreadsCreated: []patillator.SegregateFinder{
		patillator.ThreadActivity{
			Thread: &pbContext.Thread{
				Id: "post-01",
				SectionCtx: &pbContext.Section{
					Id: "mylife",
				},
			},
			ActivityMetadata: patillator.ActivityMetadata(pbMetadata.Content{
				Interactions:  uint32(12),
				LastUpdated:   &pbTime.Timestamp{
					Seconds:   time.Now().Add(-5 * time.Minute).Unix(),
				},
				AvgUpdateTime: (2 * time.Minute).Seconds(),
				DataKey:       "post-01",
			}),
		},
		patillator.ThreadActivity{
			Thread: &pbContext.Thread{
				Id: "post-02",
				SectionCtx: &pbContext.Section{
					Id: "mylife",
				},
			},
			ActivityMetadata: patillator.ActivityMetadata(pbMetadata.Content{
				Interactions:  uint32(12),
				LastUpdated:   &pbTime.Timestamp{
					Seconds:   time.Now().Add(-5 * time.Minute).Unix(),
				},
				AvgUpdateTime: (2 * time.Minute).Seconds(),
				DataKey:       "post-02",
			}),
		},
	},
	Comments:       []patillator.SegregateFinder{
		patillator.CommentActivity{
			Comment: &pbContext.Comment{
				Id:        "comment-01",
				ThreadCtx: &pbContext.Thread{
					Id: "post-01",
					SectionCtx: &pbContext.Section{
						Id: "mylife",
					},
				},
			},
			ActivityMetadata: patillator.ActivityMetadata(pbMetadata.Content{
				Interactions:  uint32(12),
				LastUpdated:   &pbTime.Timestamp{
					Seconds:   time.Now().Add(-5 * time.Minute).Unix(),
				},
				AvgUpdateTime: (2 * time.Minute).Seconds(),
				DataKey:       "post-01",
			}),
		},
		patillator.CommentActivity{
			Comment: &pbContext.Comment{
				Id:        "comment-02",
				ThreadCtx: &pbContext.Thread{
					Id: "post-01",
					SectionCtx: &pbContext.Section{
						Id: "mylife",
					},
				},
			},
			ActivityMetadata: patillator.ActivityMetadata(pbMetadata.Content{
				Interactions:  uint32(12),
				LastUpdated:   &pbTime.Timestamp{
					Seconds:   time.Now().Add(-5 * time.Minute).Unix(),
				},
				AvgUpdateTime: (2 * time.Minute).Seconds(),
				DataKey:       "post-01",
			}),
		},
	},
	Subcomments:    []patillator.SegregateFinder{
		patillator.SubcommentActivity{
			Subcomment: &pbContext.Subcomment{
				Id: "subcomment-01",
				CommentCtx: &pbContext.Comment{
					Id:        "comment-01",
					ThreadCtx: &pbContext.Thread{
						Id: "post-01",
						SectionCtx: &pbContext.Section{
							Id: "mylife",
						},
					},
				},
			},
			ActivityMetadata: patillator.ActivityMetadata(pbMetadata.Content{
				Interactions:  uint32(12),
				LastUpdated:   &pbTime.Timestamp{
					Seconds:   time.Now().Add(-5 * time.Minute).Unix(),
				},
				AvgUpdateTime: (2 * time.Minute).Seconds(),
				DataKey:       "post-01",
			}),
		},
		patillator.SubcommentActivity{
			Subcomment: &pbContext.Subcomment{
				Id: "subcomment-02",
				CommentCtx: &pbContext.Comment{
					Id:        "comment-01",
					ThreadCtx: &pbContext.Thread{
						Id: "post-01",
						SectionCtx: &pbContext.Section{
							Id: "mylife",
						},
					},
				},
			},
			ActivityMetadata: patillator.ActivityMetadata(pbMetadata.Content{
				Interactions:  uint32(12),
				LastUpdated:   &pbTime.Timestamp{
					Seconds:   time.Now().Add(-5 * time.Minute).Unix(),
				},
				AvgUpdateTime: (2 * time.Minute).Seconds(),
				DataKey:       "post-01",
			}),
		},
	},
}
