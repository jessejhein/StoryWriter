// Package api provides the local HTTP API.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"storywork/internal/project"
	"storywork/internal/story"
)

// ProjectStore is the project application boundary used by HTTP handlers.
type ProjectStore interface {
	Create(ctx context.Context, request project.CreateRequest) (project.Project, error)
	Open(ctx context.Context, path string) (project.Project, error)
}

// ActiveProjectSession stores the current project for later outline routes.
type ActiveProjectSession interface {
	Set(project.Project)
}

// StoryStore serves and mutates the active project's outline.
type StoryStore interface {
	Outline(ctx context.Context) (story.Outline, error)
	CreateArc(ctx context.Context, title string) (story.MutationResult, error)
	CreateChapter(ctx context.Context, arcID, title string) (story.MutationResult, error)
	CreateScene(ctx context.Context, chapterID, title string) (story.MutationResult, error)
	Reorder(ctx context.Context, request story.ReorderRequest) (story.MutationResult, error)
	LoadScene(ctx context.Context, sceneID string) (story.SceneDocument, error)
	SaveScene(ctx context.Context, sceneID string, request story.SaveSceneRequest) (story.SceneDocument, error)
}

// NewHandler creates the Milestone 0 API router.
func NewHandler(projects ProjectStore, session ActiveProjectSession, stories StoryStore, version string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok", "version": version})
	})
	mux.HandleFunc("POST /api/projects", func(writer http.ResponseWriter, request *http.Request) {
		var createRequest project.CreateRequest
		if err := decodeJSON(request, &createRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		created, err := projects.Create(request.Context(), createRequest)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		session.Set(created)
		writeJSON(writer, http.StatusCreated, created)
	})
	mux.HandleFunc("POST /api/projects/open", func(writer http.ResponseWriter, request *http.Request) {
		var openRequest struct {
			Path string `json:"path"`
		}
		if err := decodeJSON(request, &openRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		opened, err := projects.Open(request.Context(), openRequest.Path)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		session.Set(opened)
		writeJSON(writer, http.StatusOK, opened)
	})
	mux.HandleFunc("GET /api/outline", func(writer http.ResponseWriter, request *http.Request) {
		outline, err := stories.Outline(request.Context())
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, outline)
	})
	mux.HandleFunc("POST /api/arcs", func(writer http.ResponseWriter, request *http.Request) {
		var createRequest struct {
			Title string `json:"title"`
		}
		if err := decodeJSON(request, &createRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		result, err := stories.CreateArc(request.Context(), createRequest.Title)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, result)
	})
	mux.HandleFunc("POST /api/chapters", func(writer http.ResponseWriter, request *http.Request) {
		var createRequest struct {
			ArcID string `json:"arc_id"`
			Title string `json:"title"`
		}
		if err := decodeJSON(request, &createRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		result, err := stories.CreateChapter(request.Context(), createRequest.ArcID, createRequest.Title)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, result)
	})
	mux.HandleFunc("POST /api/scenes", func(writer http.ResponseWriter, request *http.Request) {
		var createRequest struct {
			ChapterID string `json:"chapter_id"`
			Title     string `json:"title"`
		}
		if err := decodeJSON(request, &createRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		result, err := stories.CreateScene(request.Context(), createRequest.ChapterID, createRequest.Title)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, result)
	})
	mux.HandleFunc("POST /api/outline/reorder", func(writer http.ResponseWriter, request *http.Request) {
		var reorderRequest story.ReorderRequest
		if err := decodeJSON(request, &reorderRequest); err != nil {
			writeError(writer, http.StatusBadRequest, err)
			return
		}
		result, err := stories.Reorder(request.Context(), reorderRequest)
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, result)
	})
	mux.HandleFunc("GET /api/scenes/{scene_id}", func(writer http.ResponseWriter, request *http.Request) {
		sceneDocument, err := stories.LoadScene(request.Context(), request.PathValue("scene_id"))
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, sceneDocument)
	})
	mux.HandleFunc("PUT /api/scenes/{scene_id}", func(writer http.ResponseWriter, request *http.Request) {
		var saveRequest struct {
			Title            string                  `json:"title"`
			FrontMatter      *story.SceneFrontMatter `json:"frontmatter"`
			Markdown         string                  `json:"markdown"`
			ExpectedRevision string                  `json:"expected_revision"`
		}
		if err := decodeJSONWithLimit(writer, request, &saveRequest, 6<<20); err != nil {
			status := http.StatusBadRequest
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				status = http.StatusRequestEntityTooLarge
			}
			writeError(writer, status, err)
			return
		}
		if saveRequest.FrontMatter == nil {
			writeError(writer, http.StatusBadRequest, errors.New("frontmatter is required"))
			return
		}
		sceneDocument, err := stories.SaveScene(request.Context(), request.PathValue("scene_id"), story.SaveSceneRequest{
			Title:            saveRequest.Title,
			FrontMatter:      *saveRequest.FrontMatter,
			Markdown:         saveRequest.Markdown,
			ExpectedRevision: saveRequest.ExpectedRevision,
		})
		if err != nil {
			writeStoryError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, sceneDocument)
	})
	return mux
}

func decodeJSON(request *http.Request, target any) error {
	return decodeJSONWithLimit(nil, request, target, 1<<20)
}

func decodeJSONWithLimit(writer http.ResponseWriter, request *http.Request, target any, limit int64) error {
	defer request.Body.Close()
	reader := io.Reader(request.Body)
	if writer != nil {
		reader = http.MaxBytesReader(writer, request.Body, limit)
	} else {
		reader = io.LimitReader(request.Body, limit)
	}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON request: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return errors.New("invalid JSON request: unexpected trailing JSON value")
		}
		return fmt.Errorf("invalid JSON request: %w", err)
	}
	return nil
}

func writeStoryError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, story.ErrNoActiveProject), errors.Is(err, story.ErrDirtyWorktree):
		status = http.StatusConflict
	case errors.Is(err, story.ErrInvalidTitle), errors.Is(err, story.ErrInvalidID), errors.Is(err, story.ErrInvalidReorder), errors.Is(err, story.ErrInvalidPOV), errors.Is(err, story.ErrInvalidStatus), errors.Is(err, story.ErrInvalidMarkdown), errors.Is(err, story.ErrInvalidRevision), errors.Is(err, story.ErrNoSceneChanges):
		status = http.StatusBadRequest
	case errors.Is(err, story.ErrParentNotFound), errors.Is(err, story.ErrSceneNotFound):
		status = http.StatusNotFound
	case errors.Is(err, story.ErrStaleRevision):
		status = http.StatusConflict
	}
	writeError(writer, status, err)
}

func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, map[string]string{"error": err.Error()})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		_, _ = writer.Write([]byte(strings.TrimSpace(err.Error())))
	}
}
