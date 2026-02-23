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
	mux.HandleFunc("DELETE /api/projects/{projectId}", projectHandler.Delete)

	mux.HandleFunc("GET /api/projects/{projectId}/tickets", ticketHandler.ListByProject)
	mux.HandleFunc("POST /api/projects/{projectId}/tickets", ticketHandler.Create)

	mux.HandleFunc("GET /api/projects/{projectId}/workflows", workflowHandler.ListByProject)
	mux.HandleFunc("POST /api/projects/{projectId}/workflows", workflowHandler.Start)

	mux.HandleFunc("GET /api/tickets", ticketHandler.List)
	mux.HandleFunc("GET /api/tickets/{id}", ticketHandler.GetByID)
	mux.HandleFunc("PUT /api/tickets/{id}", ticketHandler.Update)
	mux.HandleFunc("POST /api/tickets/{id}/done", ticketHandler.MarkDone)
	mux.HandleFunc("GET /api/tickets/{id}/workflows", ticketHandler.GetWorkflows)

	mux.HandleFunc("GET /api/workflows", workflowHandler.List)
	mux.HandleFunc("GET /api/workflows/{id}", workflowHandler.Get)
	mux.HandleFunc("GET /api/workflows/{id}/tasks", workflowHandler.GetTasks)
	mux.HandleFunc("GET /api/workflows/{id}/chat", workflowHandler.GetChat)
	mux.HandleFunc("POST /api/workflows/{id}/approve", workflowHandler.Approve)
	mux.HandleFunc("POST /api/workflows/{id}/reject", workflowHandler.Reject)

	mux.HandleFunc("POST /api/tasks/{id}/retry", taskHandler.Retry)
	mux.HandleFunc("GET /api/tasks/{id}/logs/stream", taskHandler.StreamLogs)

	mux.HandleFunc("GET /api/events", sseHandler.Stream)

	staticFS, _ := fs.Sub(web.StaticFiles, "static")
	fileServer := http.FileServer(http.FS(staticFS))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			indexContent, err := fs.ReadFile(staticFS, "index.html")
			if err != nil {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(indexContent)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	return mux
}
