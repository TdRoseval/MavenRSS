package tags

import (
	"encoding/json"
	"net/http"

	"MrRSS/internal/handlers/core"
	"MrRSS/internal/models"
)

// HandleTags handles GET and POST requests for tags.
// GET: Returns all tags
// POST: Creates a new tag
func HandleTags(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// GET: Return all tags
		tags, err := h.DB.GetTags()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(tags)
		return
	}

	if r.Method == http.MethodPost {
		// POST: Create new tag
		var req struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Validate input
		if req.Name == "" {
			http.Error(w, "Tag name is required", http.StatusBadRequest)
			return
		}
		if req.Color == "" {
			req.Color = "#3B82F6" // Default blue color
		}

		tag := &models.Tag{
			Name:  req.Name,
			Color: req.Color,
		}

		id, err := h.DB.AddTag(tag)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tag.ID = id
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(tag)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// HandleTagUpdate updates an existing tag.
func HandleTagUpdate(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Color    string `json:"color"`
		Position int    `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Name == "" {
		http.Error(w, "Tag name is required", http.StatusBadRequest)
		return
	}
	if req.Color == "" {
		http.Error(w, "Tag color is required", http.StatusBadRequest)
		return
	}

	err := h.DB.UpdateTag(req.ID, req.Name, req.Color, req.Position)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the updated tag
	tag := &models.Tag{
		ID:       req.ID,
		Name:     req.Name,
		Color:    req.Color,
		Position: req.Position,
	}
	json.NewEncoder(w).Encode(tag)
}

// HandleTagDelete deletes a tag by ID.
func HandleTagDelete(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.DB.DeleteTag(req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// HandleTagReorder changes the position of a tag.
func HandleTagReorder(h *core.Handler, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID          int64 `json:"id"`
		NewPosition int   `json:"new_position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.DB.ReorderTag(req.ID, req.NewPosition)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated list of tags
	tags, err := h.DB.GetTags()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(tags)
}

// RegisterTagRoutes registers all tag-related routes.
func RegisterTagRoutes(h *core.Handler, path string) {
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		HandleTags(h, w, r)
	})
	http.HandleFunc(path+"/update", func(w http.ResponseWriter, r *http.Request) {
		HandleTagUpdate(h, w, r)
	})
	http.HandleFunc(path+"/delete", func(w http.ResponseWriter, r *http.Request) {
		HandleTagDelete(h, w, r)
	})
	http.HandleFunc(path+"/reorder", func(w http.ResponseWriter, r *http.Request) {
		HandleTagReorder(h, w, r)
	})
}
