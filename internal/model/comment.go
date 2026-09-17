package model

// Comment es una nota de seguimiento asociada a una tarea.
// No tiene autor: todos los comentarios los escribe el usuario.
type Comment struct {
	ID        int64  `json:"id"`
	TaskID    int64  `json:"task_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// CommentResponse es la respuesta JSON de un solo comentario.
type CommentResponse struct {
	OK      bool    `json:"ok"`
	Comment Comment `json:"comment"`
}

// CommentListResponse es la respuesta JSON de listado de comentarios.
type CommentListResponse struct {
	Comments []Comment `json:"comments"`
}
