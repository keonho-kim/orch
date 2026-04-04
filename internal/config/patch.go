package config

import (
	"strings"

	"github.com/keonho-kim/orch/domain"
)

type ProviderPatch struct {
	Endpoint  *string
	Model     *string
	APIKey    *string
	Reasoning *string
}

type DocumentPatch struct {
	Provider          *string
	ApprovalPolicy    *string
	SelfDrivingMode   *bool
	ReactRalphIter    *int
	PlanRalphIter     *int
	CompactThresholdK *int
	Providers         map[domain.Provider]ProviderPatch
}

func (p DocumentPatch) IsEmpty() bool {
	return p.Provider == nil &&
		p.ApprovalPolicy == nil &&
		p.SelfDrivingMode == nil &&
		p.ReactRalphIter == nil &&
		p.PlanRalphIter == nil &&
		p.CompactThresholdK == nil &&
		len(p.Providers) == 0
}

func (p *DocumentPatch) SetProviderPatch(provider domain.Provider, update ProviderPatch) {
	if provider == "" {
		return
	}
	if p.Providers == nil {
		p.Providers = make(map[domain.Provider]ProviderPatch)
	}
	current := p.Providers[provider]
	if update.Endpoint != nil {
		current.Endpoint = update.Endpoint
	}
	if update.Model != nil {
		current.Model = update.Model
	}
	if update.APIKey != nil {
		current.APIKey = update.APIKey
	}
	if update.Reasoning != nil {
		current.Reasoning = update.Reasoning
	}
	p.Providers[provider] = current
}

func ApplyDocumentPatch(document *Document, patch DocumentPatch) error {
	document.Normalize()

	if patch.Provider != nil {
		provider, err := domain.ParseProvider(*patch.Provider)
		if err != nil {
			return err
		}
		document.Provider = provider.String()
	}
	if patch.ApprovalPolicy != nil {
		document.ApprovalPolicy = strings.TrimSpace(*patch.ApprovalPolicy)
	}
	if patch.SelfDrivingMode != nil {
		document.SelfDrivingMode = *patch.SelfDrivingMode
	}
	if patch.ReactRalphIter != nil {
		document.ReactRalphIter = *patch.ReactRalphIter
	}
	if patch.PlanRalphIter != nil {
		document.PlanRalphIter = *patch.PlanRalphIter
	}
	if patch.CompactThresholdK != nil {
		document.CompactThresholdK = *patch.CompactThresholdK
	}
	for provider, providerPatch := range patch.Providers {
		applyProviderPatch(document.Providers.Provider(provider), providerPatch)
	}
	return nil
}

func applyProviderPatch(target *ProviderDocument, patch ProviderPatch) {
	if patch.Endpoint != nil {
		target.Endpoint = strings.TrimSpace(*patch.Endpoint)
	}
	if patch.Model != nil {
		target.Model = strings.TrimSpace(*patch.Model)
	}
	if patch.APIKey != nil {
		target.APIKey = strings.TrimSpace(*patch.APIKey)
	}
	if patch.Reasoning != nil {
		normalized, err := domain.ParseReasoningValue(*patch.Reasoning)
		if err == nil {
			target.Reasoning = normalized
		} else {
			target.Reasoning = strings.TrimSpace(*patch.Reasoning)
		}
	}
}
