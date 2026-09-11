package handler

import "net/http"

func NewRouter(h *TodoHandler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", h.Index)
	mux.HandleFunc("POST /add", h.Add)
	mux.HandleFunc("GET /edit/{id}", h.EditForm)
	mux.HandleFunc("POST /edit/{id}", h.Update)
	mux.HandleFunc("POST /delete/{id}", h.Delete)
	mux.HandleFunc("POST /toggle/{id}", h.Toggle)
	return mux
}
