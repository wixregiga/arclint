package mapserve

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/wixregiga/arclint/internal/application"
)

func (s Server) features(w http.ResponseWriter, _ *http.Request) {
	if s.Features == nil {
		writeJSON(w, 200, []byte(`{"features":[]}`))
		return
	}
	result, err := (application.ListFeatureDrafts{Store: s.Features}).Execute()
	featureResponse(w, map[string]any{"features": result}, err)
}

func (s Server) feature(w http.ResponseWriter, r *http.Request) {
	if s.Features == nil {
		featureResponse(w, nil, application.ErrFeatureMissing)
		return
	}
	id := r.URL.Query().Get("id")
	switch r.Method {
	case http.MethodGet:
		d, err := (application.GetFeatureDraft{Store: s.Features}).Execute(id)
		featureResponse(w, map[string]any{"feature": d}, err)
	case http.MethodPost:
		// Required pointers distinguish omitted text from deliberately empty prose.
		var body struct {
			ID         string                         `json:"id"`
			Revision   string                         `json:"revision"`
			Title      *string                        `json:"title"`
			Body       *string                        `json:"body"`
			References []application.FeatureReference `json:"references"`
		}
		if !readJSON(w, r, &body) {
			return
		}
		if body.Title == nil || body.Body == nil || body.References == nil {
			featureResponse(w, nil, application.ErrFeatureInput)
			return
		}
		d, err := (application.SaveFeatureDraft{Store: s.Features}).Execute(application.FeatureDraft{ID: body.ID, Revision: body.Revision, Title: *body.Title, Body: *body.Body, References: body.References})
		featureResponse(w, map[string]any{"feature": d}, err)
	case http.MethodDelete:
		err := (application.DeleteFeatureDraft{Store: s.Features}).Execute(id, r.Header.Get("If-Match"))
		featureResponse(w, map[string]string{"removed": id}, err)
	}
}

func featureResponse(w http.ResponseWriter, value any, err error) {
	if err != nil {
		status := 422
		switch {
		case errors.Is(err, application.ErrFeatureInput):
			status = 400
		case errors.Is(err, application.ErrFeatureMissing):
			status = 404
		case errors.Is(err, application.ErrFeatureConflict):
			status = 409
		}
		writeError(w, status, err)
		return
	}
	b, err := json.Marshal(value)
	if err != nil {
		writeError(w, 422, err)
		return
	}
	writeJSON(w, 200, b)
}
