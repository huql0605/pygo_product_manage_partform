package product

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	writeJson(w, http.StatusOK, h.store.List())
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var p Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if p.Name == "" {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": "name is require"})
		return
	}
	writeJson(w, http.StatusCreated, h.store.Create(p))
}

func writeJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)

}
