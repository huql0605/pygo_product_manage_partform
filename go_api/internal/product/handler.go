package product

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	item, err := h.svc.List()
	if err != nil {
		writeJson(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJson(w, http.StatusOK, item)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var p Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.svc.Create(&p); err != nil {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": "name is require"})
		return
	}
	writeJson(w, http.StatusCreated, p)
}

func writeJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)

}
