package model

// Comment is a follow-up note attached to a task.
// It has no author: every comment is written by the user.
type Comment struct {
	ID        int64  `json:"id"`
	TaskID    int64  `json:"task_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// CommentResponse is the JSON response for a single comment.
type CommentResponse struct {
	OK      bool    `json:"ok"`
	Comment Comment `json:"comment"`
}

// CommentListResponse is the JSON response for a comment list.
type CommentListResponse struct {
	Comments []Comment `json:"comments"`
}
