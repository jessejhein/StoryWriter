package provider

import (
	"context"
)

type profileStore interface {
	Load(ctx context.Context) ([]Profile, *string, error)
	Save(ctx context.Context, profiles []Profile, expectedRevision *string) ([]Profile, *string, error)
}

type Service struct {
	store  profileStore
	broker Broker
}

type ResolvedProfile struct {
	Profile
	Credential Credential
}

func NewService(store profileStore, broker Broker) *Service {
	return &Service{store: store, broker: broker}
}

func (s *Service) List(ctx context.Context) ([]Profile, *string, error) {
	profiles, revision, err := s.store.Load(ctx)
	if err != nil {
		return nil, nil, err
	}
	withReadiness, err := s.attachReadiness(ctx, profiles)
	if err != nil {
		return nil, nil, err
	}
	return withReadiness, revision, nil
}

func (s *Service) Save(ctx context.Context, profiles []Profile, expectedRevision *string) ([]Profile, *string, error) {
	saved, revision, err := s.store.Save(ctx, profiles, expectedRevision)
	if err != nil {
		return nil, nil, err
	}
	withReadiness, err := s.attachReadiness(ctx, saved)
	if err != nil {
		return nil, nil, err
	}
	return withReadiness, revision, nil
}

func (s *Service) Resolve(ctx context.Context, profileID string) (ResolvedProfile, bool, error) {
	profiles, _, err := s.store.Load(ctx)
	if err != nil {
		return ResolvedProfile{}, false, err
	}
	for _, profile := range profiles {
		if profile.ID != profileID {
			continue
		}
		resolved := ResolvedProfile{Profile: profile}
		if profile.Auth.Type == AuthTypeBearerEnv {
			credential, readiness, err := s.broker.Resolve(ctx, profile.Auth.CredentialEnv)
			if err != nil {
				return ResolvedProfile{}, false, err
			}
			resolved.Credential = credential
			resolved.Readiness = readiness
			return resolved, true, nil
		}
		resolved.Readiness = ReadinessReady
		return resolved, true, nil
	}
	return ResolvedProfile{}, false, nil
}

func (s *Service) attachReadiness(ctx context.Context, profiles []Profile) ([]Profile, error) {
	next := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		updated := profile
		if profile.Auth.Type == AuthTypeBearerEnv {
			_, readiness, err := s.broker.Resolve(ctx, profile.Auth.CredentialEnv)
			if err != nil {
				return nil, err
			}
			updated.Readiness = readiness
		} else {
			updated.Readiness = ReadinessReady
		}
		next = append(next, updated)
	}
	return next, nil
}
