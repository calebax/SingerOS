package auth

import (
	"context"
	"fmt"

	"github.com/insmtx/SingerOS/backend/internal/api/dto"
)

// AccountResolver 负责按 user + provider 解析运行时可用账户。
type AccountResolver struct {
	store Store
}

// NewAccountResolver 创建一个新的账户解析器。
func NewAccountResolver(store Store) *AccountResolver {
	return &AccountResolver{store: store}
}

// Resolve 解析运行时应使用的账户与凭证。
func (r *AccountResolver) Resolve(ctx context.Context, req *dto.ResolveAccountRequest) (*dto.ResolvedAccount, error) {
	if req == nil {
		return nil, fmt.Errorf("resolve request is required")
	}

	selector := mergeSelector(req)
	if selector.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}

	explicitProfileID := selector.ExplicitProfileID
	if explicitProfileID == "" {
		explicitProfileID = req.AccountID
	}
	if explicitProfileID == "" && selector.ExternalRefs != nil {
		explicitProfileID = selector.ExternalRefs["account_id"]
	}

	if explicitProfileID != "" {
		account, err := r.store.GetAuthorizedAccount(ctx, explicitProfileID)
		if err != nil {
			return nil, err
		}
		if account.Provider != selector.Provider {
			return nil, fmt.Errorf("account does not belong to the requested provider")
		}
		if selector.SubjectID != "" && selector.SubjectType == dto.SubjectTypeUser && account.UserID != selector.SubjectID {
			return nil, fmt.Errorf("account does not belong to the requested user")
		}
		if account.Status != dto.AccountStatusActive {
			return nil, fmt.Errorf("account is not active")
		}

		credential, err := r.store.GetCredential(ctx, account.ID)
		if err != nil {
			return nil, err
		}

		return &dto.ResolvedAccount{
			Account:    account,
			Credential: credential,
			ResolvedBy: "explicit_profile_id",
		}, nil
	}

	if selector.SubjectID != "" {
		account, err := r.store.GetDefaultAccount(ctx, selector.SubjectID, selector.Provider)
		if err == nil && account != nil && account.Status == dto.AccountStatusActive {
			credential, credErr := r.store.GetCredential(ctx, account.ID)
			if credErr == nil {
				return &dto.ResolvedAccount{
					Account:    account,
					Credential: credential,
					ResolvedBy: "subject_default",
				}, nil
			}
		}

		accounts, err := r.store.ListUserAccounts(ctx, selector.SubjectID, selector.Provider)
		if err != nil {
			return nil, err
		}

		for _, candidate := range accounts {
			if candidate.Status != dto.AccountStatusActive {
				continue
			}

			credential, credErr := r.store.GetCredential(ctx, candidate.ID)
			if credErr != nil {
				continue
			}

			return &dto.ResolvedAccount{
				Account:    candidate,
				Credential: credential,
				ResolvedBy: "first_available",
			}, nil
		}
	}

	if selector.SubjectID == "" {
		return nil, fmt.Errorf("no authorization profile matched for provider %s", selector.Provider)
	}
	return nil, fmt.Errorf("no authorized account found for provider %s", selector.Provider)
}

// ResolveAuthorization resolves a stored account profile as a generic runtime authorization.
func (r *AccountResolver) ResolveAuthorization(ctx context.Context, req *dto.ResolveAuthorizationRequest) (*dto.ResolvedAuthorization, bool, error) {
	if req == nil {
		return nil, true, fmt.Errorf("resolve authorization request is required")
	}

	resolved, err := r.Resolve(ctx, &dto.ResolveAccountRequest{
		Selector:  req.Selector,
		UserID:    req.UserID,
		Provider:  req.Provider,
		AccountID: req.AccountID,
	})
	if err != nil {
		return nil, true, err
	}

	provider := ""
	profileID := ""
	if resolved.Account != nil {
		provider = resolved.Account.Provider
		profileID = resolved.Account.ID
	}

	return &dto.ResolvedAuthorization{
		Provider:   provider,
		ProfileID:  profileID,
		ResolvedBy: resolved.ResolvedBy,
		Account:    resolved.Account,
		Credential: resolved.Credential,
	}, true, nil
}

func mergeSelector(req *dto.ResolveAccountRequest) *dto.AuthSelector {
	selector := &dto.AuthSelector{}
	if req != nil && req.Selector != nil {
		selector = cloneSelector(req.Selector)
	}
	if req == nil {
		return selector
	}
	if selector.Provider == "" {
		selector.Provider = req.Provider
	}
	if selector.ExplicitProfileID == "" {
		selector.ExplicitProfileID = req.AccountID
	}
	if selector.SubjectID == "" && req.UserID != "" {
		selector.SubjectID = req.UserID
	}
	if selector.SubjectType == "" && selector.SubjectID != "" {
		selector.SubjectType = dto.SubjectTypeUser
	}
	return selector
}

func cloneSelector(selector *dto.AuthSelector) *dto.AuthSelector {
	if selector == nil {
		return &dto.AuthSelector{}
	}
	cloned := *selector
	if selector.ExternalRefs != nil {
		cloned.ExternalRefs = make(map[string]string, len(selector.ExternalRefs))
		for key, value := range selector.ExternalRefs {
			cloned.ExternalRefs[key] = value
		}
	}
	return &cloned
}
