package api

import (
	"io/fs"
	"net/http"

	"github.com/piotr-halas/decodo-workflow/internal/api/handlers"
	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/web"
	"github.com/piotr-halas/decodo-workflow/internal/workflow"
)

func NewRouter(database *db.DB, engine *workflow.Engine) http.Handler {
	mux := http.NewServeMux()

	projectHandler := handlers.NewProjectHandler(database)
	ticketHandler := handlers.NewTicketHandler(database)
	workflowHandler := handlers.NewWorkflowHandler(database, engine)
	taskHandler := handlers.NewTaskHandler(database)
	sseHandler := handlers.NewSSEHandler(database)

	mux.HandleFunc("GET /api/health", handlers.Health)

	mux.HandleFunc("GET /api/projects", projectHandler.List)
	mux.HandleFunc("POST /api/projects", projectHandler.Create)
	mux.HandleFunc("GET /api/projects/{projectId}", projectHandler.Get)
	mux.HandleFunc("PUT /api/projects/{projectId}", projectHandler.Update)

	mux.HandleFunc("GET /api/projects/{projectId}/tickets", ticketHandler.ListByProject)
	mux.HandleFunc("POST /api/projects/{projectId}/tickets", ticketHandler.Create)

	mux.HandleFunc("GET /api/projects/{projectId}/workflows", workflowHandler.ListByProject)
	mux.HandleFunc("POST /api/projects/{projectId}/workflows", workflowHandler.Start)

	mux.HandleFunc("GET /api/tickets", ticketHandler.List)
	mux.HandleFunc("GET /api/tickets/{key}", ticketHandler.GetByKey)

	mux.HandleFunc("GET /api/workflows", workflowHandler.List)
	mux.HandleFunc("GET /api/workflows/{id}", workflowHandler.Get)
	mux.HandleFunc("GET /api/workflows/{id}/tasks", workflowHandler.GetTasks)
	mux.HandleFunc("GET /api/workflows/{id}/chat", workflowHandler.GetChat)
	mux.HandleFunc("POST /api/workflows/{id}/approve", workflowHandler.Approve)
	mux.HandleFunc("POST /api/workflows/{id}/reject", workflowHandler.Reject)

	mux.HandleFunc("GET /api/tasks/{id}/logs/stream", taskHandler.StreamLogs)

	mux.HandleFunc("GET /api/events", sseHandler.Stream)

	staticFS, _ := fs.Sub(web.StaticFiles, "static")
	mux.Handle("GET /", http.FileServer(http.FS(staticFS)))

	return mux
}
