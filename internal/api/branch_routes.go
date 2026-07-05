package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"storywork/internal/branch"
)

// BranchStore serves branch lifecycle, comparison, promotion, and analysis.
type BranchStore interface {
	Status(ctx context.Context) (branch.RepositoryStatus, error)
	ListExperiments(ctx context.Context) ([]branch.ExperimentRef, error)
	CreateExperiment(ctx context.Context, name string) (branch.RepositoryStatus, error)
	SwitchTarget(ctx context.Context, target string, expectedHead *branch.CommitID) (branch.RepositoryStatus, error)
	LoadComparison(ctx context.Context, experimentID string) (branch.Comparison, error)
	LoadFileComparison(ctx context.Context, experimentID, path string) (branch.FileComparison, error)
	AnalyzeRamifications(ctx context.Context, experimentID string, request branch.AnalysisRequest) (branch.AnalysisResult, error)
	PromoteSelectedFiles(ctx context.Context, request branch.PromotionRequest) (branch.PromotionResult, error)
	DiscardExperiment(ctx context.Context, experimentID string, expectedHead branch.CommitID) (branch.RepositoryStatus, error)
}

type branchRouteDeps struct {
	branches BranchStore
}

type branchStatusResponse struct {
	ActiveBranch       string               `json:"active_branch"`
	ActiveKind         string               `json:"active_kind"`
	MainHead           branch.CommitID      `json:"main_head"`
	ExperimentHead     *branch.CommitID     `json:"experiment_head"`
	ActiveExperimentID *branch.ExperimentID `json:"active_experiment_id"`
	WorktreeClean      bool                 `json:"worktree_clean"`
}

func publicBranchStatus(status branch.RepositoryStatus) branchStatusResponse {
	response := branchStatusResponse{
		ActiveBranch:  status.ActiveBranch,
		ActiveKind:    "canon",
		MainHead:      status.MainHead,
		WorktreeClean: status.IsClean,
	}
	if status.IsManaged {
		response.ActiveKind = "experiment"
		response.ExperimentHead = &status.ExperimentHead
		response.ActiveExperimentID = &status.ExperimentID
	}
	return response
}

type experimentResponse struct {
	ExperimentID branch.ExperimentID `json:"experiment_id"`
	BranchName   branch.BranchRef    `json:"branch_name"`
	Head         branch.CommitID     `json:"head"`
	DisplayName  string              `json:"display_name"`
}

func publicExperiments(experiments []branch.ExperimentRef) []experimentResponse {
	result := make([]experimentResponse, 0, len(experiments))
	for _, experiment := range experiments {
		_, slug, _ := branch.ParseManagedExperimentRef(string(experiment.BranchName))
		result = append(result, experimentResponse{
			ExperimentID: experiment.ID, BranchName: experiment.BranchName,
			Head: experiment.Head, DisplayName: strings.TrimSpace(slug),
		})
	}
	return result
}

func registerBranchRoutes(mux *http.ServeMux, deps branchRouteDeps) {
	branches := deps.branches
	mux.HandleFunc("GET /api/branches/status", func(writer http.ResponseWriter, request *http.Request) {
		if err := validateExactQuery(request); err != nil {
			writeInvalidBranchRequest(writer)
			return
		}
		status, err := branches.Status(request.Context())
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, publicBranchStatus(status))
	})
	mux.HandleFunc("GET /api/branches", func(writer http.ResponseWriter, request *http.Request) {
		if err := validateExactQuery(request); err != nil {
			writeInvalidBranchRequest(writer)
			return
		}
		experiments, err := branches.ListExperiments(request.Context())
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		if experiments == nil {
			experiments = []branch.ExperimentRef{}
		}
		writeJSON(writer, http.StatusOK, map[string]any{"experiments": publicExperiments(experiments)})
	})
	mux.HandleFunc("POST /api/branches", func(writer http.ResponseWriter, request *http.Request) {
		if err := validateExactQuery(request); err != nil {
			writeInvalidBranchRequest(writer)
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &body, 1<<20, requiredJSONField{name: "name"}); err != nil {
			writeBranchBodyError(writer, err)
			return
		}
		status, err := branches.CreateExperiment(request.Context(), body.Name)
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		writeJSON(writer, http.StatusCreated, publicBranchStatus(status))
	})
	mux.HandleFunc("POST /api/branches/switch", func(writer http.ResponseWriter, request *http.Request) {
		if err := validateExactQuery(request); err != nil {
			writeInvalidBranchRequest(writer)
			return
		}
		var body struct {
			Target       string  `json:"target"`
			ExpectedHead *string `json:"expected_head"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &body, 1<<20, requiredJSONField{name: "target"}); err != nil {
			writeBranchBodyError(writer, err)
			return
		}
		var expected *branch.CommitID
		if body.ExpectedHead != nil {
			head, err := branch.ValidateCommitID(*body.ExpectedHead)
			if err != nil {
				writeBranchError(writer, err)
				return
			}
			expected = &head
		}
		if (body.Target == branch.CanonBranchName && expected != nil) || (body.Target != branch.CanonBranchName && expected == nil) {
			writeInvalidBranchRequest(writer)
			return
		}
		status, err := branches.SwitchTarget(request.Context(), body.Target, expected)
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, publicBranchStatus(status))
	})
	mux.HandleFunc("GET /api/branches/{experiment_id}/comparison", func(writer http.ResponseWriter, request *http.Request) {
		if _, err := branch.ValidateExperimentID(request.PathValue("experiment_id")); err != nil {
			writeBranchError(writer, err)
			return
		}
		if err := validateExactQuery(request); err != nil {
			writeInvalidBranchRequest(writer)
			return
		}
		comparison, err := branches.LoadComparison(request.Context(), request.PathValue("experiment_id"))
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, comparison)
	})
	mux.HandleFunc("GET /api/branches/{experiment_id}/comparison/file", func(writer http.ResponseWriter, request *http.Request) {
		if _, err := branch.ValidateExperimentID(request.PathValue("experiment_id")); err != nil {
			writeBranchError(writer, err)
			return
		}
		if err := validateExactQuery(request, "path"); err != nil {
			writeInvalidBranchRequest(writer)
			return
		}
		path := request.URL.Query().Get("path")
		if path == "" {
			writeBranchError(writer, branch.ErrInvalidProjectPath)
			return
		}
		comparison, err := branches.LoadFileComparison(request.Context(), request.PathValue("experiment_id"), path)
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, comparison)
	})
	mux.HandleFunc("POST /api/branches/{experiment_id}/ramifications", func(writer http.ResponseWriter, request *http.Request) {
		if _, err := branch.ValidateExperimentID(request.PathValue("experiment_id")); err != nil {
			writeBranchError(writer, err)
			return
		}
		if err := validateExactQuery(request); err != nil {
			writeInvalidBranchRequest(writer)
			return
		}
		var body struct {
			Goal                   string `json:"goal"`
			ProfileID              string `json:"profile_id"`
			Model                  string `json:"model"`
			ExpectedMainHead       string `json:"expected_main_head"`
			ExpectedExperimentHead string `json:"expected_experiment_head"`
			ComparisonFingerprint  string `json:"comparison_fingerprint"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &body, 1<<20,
			requiredJSONField{name: "goal"},
			requiredJSONField{name: "profile_id"},
			requiredJSONField{name: "model"},
			requiredJSONField{name: "expected_main_head"},
			requiredJSONField{name: "expected_experiment_head"},
			requiredJSONField{name: "comparison_fingerprint"},
		); err != nil {
			writeBranchBodyError(writer, err)
			return
		}
		mainHead, err := branch.ValidateCommitID(body.ExpectedMainHead)
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		experimentHead, err := branch.ValidateCommitID(body.ExpectedExperimentHead)
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		if err := branch.ValidateFingerprint(body.ComparisonFingerprint); err != nil {
			writeBranchError(writer, err)
			return
		}
		body.ProfileID = strings.TrimSpace(body.ProfileID)
		body.Model = strings.TrimSpace(body.Model)
		if body.ProfileID == "" || body.Model == "" {
			writeBranchError(writer, branch.ErrInvalidAnalysis)
			return
		}
		result, err := branches.AnalyzeRamifications(request.Context(), request.PathValue("experiment_id"), branch.AnalysisRequest{
			Goal:                   body.Goal,
			ProfileID:              body.ProfileID,
			Model:                  body.Model,
			ExpectedMainHead:       mainHead,
			ExpectedExperimentHead: experimentHead,
			ExpectedFingerprint:    body.ComparisonFingerprint,
		})
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, result)
	})
	mux.HandleFunc("POST /api/branches/{experiment_id}/promote", func(writer http.ResponseWriter, request *http.Request) {
		experimentID, err := branch.ValidateExperimentID(request.PathValue("experiment_id"))
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		if err := validateExactQuery(request); err != nil {
			writeInvalidBranchRequest(writer)
			return
		}
		var body struct {
			Paths                  []string `json:"paths"`
			ExpectedMainHead       string   `json:"expected_main_head"`
			ExpectedExperimentHead string   `json:"expected_experiment_head"`
			ComparisonFingerprint  string   `json:"comparison_fingerprint"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &body, 1<<20,
			requiredJSONField{name: "paths"},
			requiredJSONField{name: "expected_main_head"},
			requiredJSONField{name: "expected_experiment_head"},
			requiredJSONField{name: "comparison_fingerprint"},
		); err != nil {
			writeBranchBodyError(writer, err)
			return
		}
		mainHead, err := branch.ValidateCommitID(body.ExpectedMainHead)
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		experimentHead, err := branch.ValidateCommitID(body.ExpectedExperimentHead)
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		if err := branch.ValidateFingerprint(body.ComparisonFingerprint); err != nil {
			writeBranchError(writer, err)
			return
		}
		paths, err := branch.ValidateSelectedPaths(body.Paths)
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		result, err := branches.PromoteSelectedFiles(request.Context(), branch.PromotionRequest{
			ExperimentID:           experimentID,
			Paths:                  paths,
			ExpectedMainHead:       mainHead,
			ExpectedExperimentHead: experimentHead,
			ExpectedFingerprint:    body.ComparisonFingerprint,
		})
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, result)
	})
	mux.HandleFunc("POST /api/branches/{experiment_id}/discard", func(writer http.ResponseWriter, request *http.Request) {
		if _, err := branch.ValidateExperimentID(request.PathValue("experiment_id")); err != nil {
			writeBranchError(writer, err)
			return
		}
		if err := validateExactQuery(request); err != nil {
			writeInvalidBranchRequest(writer)
			return
		}
		var body struct {
			ExpectedExperimentHead string `json:"expected_experiment_head"`
		}
		if err := decodeJSONWithRequiredFields(writer, request, &body, 1<<20, requiredJSONField{name: "expected_experiment_head"}); err != nil {
			writeBranchBodyError(writer, err)
			return
		}
		head, err := branch.ValidateCommitID(body.ExpectedExperimentHead)
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		status, err := branches.DiscardExperiment(request.Context(), request.PathValue("experiment_id"), head)
		if err != nil {
			writeBranchError(writer, err)
			return
		}
		writeJSON(writer, http.StatusOK, publicBranchStatus(status))
	})
	mux.HandleFunc("/api/branches", methodNotAllowed("GET, POST"))
	mux.HandleFunc("/api/branches/status", methodNotAllowed("GET"))
	mux.HandleFunc("/api/branches/switch", methodNotAllowed("POST"))
	mux.HandleFunc("/api/branches/{experiment_id}/comparison", methodNotAllowed("GET"))
	mux.HandleFunc("/api/branches/{experiment_id}/comparison/file", methodNotAllowed("GET"))
	mux.HandleFunc("/api/branches/{experiment_id}/ramifications", methodNotAllowed("POST"))
	mux.HandleFunc("/api/branches/{experiment_id}/promote", methodNotAllowed("POST"))
	mux.HandleFunc("/api/branches/{experiment_id}/discard", methodNotAllowed("POST"))
}

func writeInvalidBranchRequest(writer http.ResponseWriter) {
	writeError(writer, http.StatusBadRequest, errors.New("invalid branch request"))
}

func writeBranchBodyError(writer http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(writer, http.StatusRequestEntityTooLarge, errors.New("request body exceeds the 1 MiB limit"))
		return
	}
	writeError(writer, http.StatusBadRequest, errors.New("invalid branch request"))
}

func writeBranchError(writer http.ResponseWriter, err error) {
	writeError(writer, statusForBranchError(err), sanitizeBranchError(err))
}

func statusForBranchError(err error) int {
	switch {
	case errors.Is(err, branch.ErrNoActiveProject), errors.Is(err, branch.ErrDirtyWorktree), errors.Is(err, branch.ErrStaleRef), errors.Is(err, branch.ErrStaleFingerprint), errors.Is(err, branch.ErrPromotionConflict), errors.Is(err, branch.ErrDetachedHEAD), errors.Is(err, branch.ErrUnmanagedBranch):
		return http.StatusConflict
	case errors.Is(err, branch.ErrInvalidExperimentID), errors.Is(err, branch.ErrInvalidExperimentName), errors.Is(err, branch.ErrInvalidBranchRef), errors.Is(err, branch.ErrInvalidCommitID), errors.Is(err, branch.ErrInvalidProjectPath), errors.Is(err, branch.ErrInvalidFingerprint), errors.Is(err, branch.ErrInvalidPromotion), errors.Is(err, branch.ErrInvalidAnalysis), errors.Is(err, branch.ErrAnalysisBudget), errors.Is(err, branch.ErrTooManyChangedPaths):
		return http.StatusBadRequest
	case errors.Is(err, branch.ErrExperimentNotFound), errors.Is(err, branch.ErrPathNotInComparison):
		return http.StatusNotFound
	case errors.Is(err, branch.ErrFileTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, branch.ErrProviderRejected), errors.Is(err, branch.ErrInvalidAnalysisOutput):
		return http.StatusBadGateway
	case errors.Is(err, branch.ErrProviderUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func sanitizeBranchError(err error) error {
	switch {
	case errors.Is(err, branch.ErrNoActiveProject), errors.Is(err, branch.ErrDirtyWorktree), errors.Is(err, branch.ErrStaleRef), errors.Is(err, branch.ErrStaleFingerprint), errors.Is(err, branch.ErrDetachedHEAD):
		return errors.New("branch operation conflicts with current project state")
	case errors.Is(err, branch.ErrPromotionConflict):
		return err
	case errors.Is(err, branch.ErrExperimentNotFound), errors.Is(err, branch.ErrPathNotInComparison):
		return errors.New("branch resource not found")
	case errors.Is(err, branch.ErrInvalidExperimentID), errors.Is(err, branch.ErrInvalidExperimentName), errors.Is(err, branch.ErrInvalidBranchRef), errors.Is(err, branch.ErrInvalidCommitID), errors.Is(err, branch.ErrInvalidProjectPath), errors.Is(err, branch.ErrInvalidFingerprint), errors.Is(err, branch.ErrInvalidPromotion), errors.Is(err, branch.ErrInvalidAnalysis), errors.Is(err, branch.ErrAnalysisBudget), errors.Is(err, branch.ErrTooManyChangedPaths):
		return errors.New("invalid branch request")
	case errors.Is(err, branch.ErrProviderRejected), errors.Is(err, branch.ErrInvalidAnalysisOutput):
		return errors.New("analysis provider returned an invalid response")
	case errors.Is(err, branch.ErrProviderUnavailable):
		return errors.New("analysis provider is unavailable")
	default:
		return errors.New("branch operation failed")
	}
}
