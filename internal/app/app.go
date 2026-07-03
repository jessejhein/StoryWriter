package app

// app.go wires the production adapters into the Storywork HTTP application.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"storywork/internal/action"
	"storywork/internal/agent"
	"storywork/internal/api"
	"storywork/internal/branch"
	"storywork/internal/contextpack"
	"storywork/internal/extract"
	"storywork/internal/gitstore"
	"storywork/internal/importer"
	"storywork/internal/index"
	"storywork/internal/modelchat"
	"storywork/internal/mutation"
	"storywork/internal/project"
	"storywork/internal/projectcheck"
	"storywork/internal/provider"
	"storywork/internal/story"
	"storywork/internal/storyfile"
	"storywork/internal/workspace"
)

var errInvalidProviderConfigPath = errors.New("invalid provider config path")

type providerDependencies struct {
	service   *provider.Service
	configErr error
}

func newProviderDependencies(override string, userConfigDir func() (string, error)) *providerDependencies {
	configDir, err := provider.ResolveConfigDir(override, userConfigDir)
	if err != nil {
		return &providerDependencies{configErr: fmt.Errorf("%w: %w", errInvalidProviderConfigPath, err)}
	}
	return &providerDependencies{service: provider.NewService(
		provider.NewStore(filepath.Join(configDir, "providers.yaml")),
		provider.EnvironmentBroker{LookupEnv: os.LookupEnv},
	)}
}

func (p *providerDependencies) List(ctx context.Context) ([]provider.Profile, *string, error) {
	if p.configErr != nil {
		return nil, nil, p.configErr
	}
	return p.service.List(ctx)
}

func (p *providerDependencies) Save(ctx context.Context, profiles []provider.Profile, expectedRevision *string) ([]provider.Profile, *string, error) {
	if p.configErr != nil {
		return nil, nil, p.configErr
	}
	return p.service.Save(ctx, profiles, expectedRevision)
}

func (p *providerDependencies) Resolve(ctx context.Context, profileID string) (provider.ResolvedProfile, bool, error) {
	if p.configErr != nil {
		return provider.ResolvedProfile{}, false, p.configErr
	}
	return p.service.Resolve(ctx, profileID)
}

func (p *providerDependencies) ProviderProfiles(ctx context.Context) ([]provider.Profile, *string, error) {
	return p.List(ctx)
}

func (p *providerDependencies) SaveProviderProfiles(ctx context.Context, profiles []provider.Profile, expectedRevision *string) ([]provider.Profile, *string, error) {
	return p.Save(ctx, profiles, expectedRevision)
}

type importHandlerStore struct {
	service *importer.Service
}

func (s *importHandlerStore) ImportDirectory(ctx context.Context, sourceDirectory string) (importer.ImportResponse, error) {
	return s.service.ImportDirectory(ctx, sourceDirectory)
}

func (s *importHandlerStore) ListImports(ctx context.Context) ([]importer.ImportSummary, error) {
	return s.service.ListImports(ctx)
}

func (s *importHandlerStore) LoadImport(ctx context.Context, importID string) (importer.ImportResponse, error) {
	return s.service.LoadImport(ctx, importID)
}

func (s *importHandlerStore) ListImportChunks(ctx context.Context, importID string) ([]importer.Chunk, error) {
	return s.service.ListChunks(ctx, importID)
}

func (s *importHandlerStore) ExtractImport(ctx context.Context, request importer.ExtractRequest) (importer.ExtractResponse, error) {
	return s.service.Extract(ctx, request)
}

func (s *importHandlerStore) ListImportCandidates(ctx context.Context, status *importer.CandidateStatus, kind *importer.CandidateKind) ([]importer.Candidate, error) {
	return s.service.ListCandidatesFiltered(ctx, status, kind)
}

func (s *importHandlerStore) LoadImportCandidate(ctx context.Context, candidateID string) (importer.Candidate, error) {
	return s.service.LoadCandidate(ctx, candidateID)
}

func (s *importHandlerStore) UpdateImportCandidate(ctx context.Context, candidateID, expectedRevision string, proposal importer.CandidateProposal) (importer.Candidate, error) {
	return s.service.UpdateCandidate(ctx, candidateID, expectedRevision, proposal)
}

func (s *importHandlerStore) MergeImportCandidates(ctx context.Context, candidateID string, request importer.MergeRequest) (importer.Candidate, []string, error) {
	return s.service.MergeCandidates(ctx, candidateID, request)
}

func (s *importHandlerStore) DiscardImportCandidate(ctx context.Context, candidateID, expectedRevision string) (importer.Candidate, error) {
	return s.service.DiscardCandidate(ctx, candidateID, expectedRevision)
}

func (s *importHandlerStore) AcceptImportCandidate(ctx context.Context, candidateID, expectedRevision string) (importer.Candidate, []importer.CanonicalRef, error) {
	return s.service.AcceptCandidate(ctx, candidateID, expectedRevision)
}

// NewHandler creates the production HTTP application for the supplied version string.
func NewHandler(version string) http.Handler {
	git := gitstore.New("git")
	disposableIndex := index.New()
	session := workspace.NewSession()
	projects := project.NewService(git, disposableIndex, time.Now)
	files := storyfile.New()
	mutations := mutation.NewCoordinator()
	stories := story.NewService(session, files, git, disposableIndex, story.NewRandomIDGenerator()).WithMutationCoordinator(mutations)
	providerService := newProviderDependencies(os.Getenv("STORYWORK_CONFIG_DIR"), os.UserConfigDir)
	importService := importer.NewService(session, git, disposableIndex, importer.NewSourceStore(), importer.NewRandomIDGenerator(), time.Now).
		WithMutationCoordinator(mutations).
		WithExtractor(extract.NewRemoteExtractor(providerService, nil)).
		WithStoryMutator(stories)
	branchRepo := &branch.GitRepository{Store: git}
	branchAnalyzer := &branch.ModelchatAnalyzer{
		Resolver: func(ctx context.Context, profileID string) (modelchat.Request, error) {
			resolved, ok, err := providerService.Resolve(ctx, profileID)
			if err != nil {
				return modelchat.Request{}, err
			}
			if !ok {
				return modelchat.Request{}, branch.ErrProviderUnavailable
			}
			return modelchat.Request{Profile: resolved}, nil
		},
		Completer: modelchat.NewHTTPClient(),
	}
	branchService := branch.NewService(
		branchRepo,
		disposableIndex,
		mutations,
		branch.SessionAdapter{PathFn: func() (string, bool) {
			current, ok := session.Current()
			if !ok {
				return "", false
			}
			return current.Path, true
		}},
		projectcheck.New(),
		branchAnalyzer,
		branch.NewRandomIDGenerator(),
	)
	actions := action.NewService(
		session,
		agent.NewLoader(),
		stories,
		stories,
		agent.NewDispatcher(providerService, nil),
		providerService,
		action.NewRunStore(),
		action.NewRandomIDGenerator(),
	).WithMaterialSource(stories).WithContextBuilder(contextpack.NewBuilder()).
		WithBodyAcceptor(stories).
		WithInvitationStore(action.NewInvitationStore(1000)).
		WithInvitationIDGenerator(action.NewRandomInvitationIDGenerator())
	return api.NewHandler(api.HandlerDependencies{
		Projects:  projects,
		Session:   session,
		Stories:   stories,
		Actions:   actions,
		Providers: providerService,
		Imports:   &importHandlerStore{service: importService},
		Branches:  branchService,
		Version:   version,
	})
}
