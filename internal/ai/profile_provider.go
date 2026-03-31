package ai

import (
	"strconv"

	"MavenRSS/internal/models"
)

// ProfileProvider provides AI profile resolution for different features
type ProfileProvider struct {
	db ProfileDB
}

// ProfileDB interface for database operations needed by ProfileProvider
type ProfileDB interface {
	GetAIProfileForUser(userID, id int64) (*models.AIProfile, error)
	GetDefaultAIProfileForUser(userID int64) (*models.AIProfile, error)
	GetSettingForUser(userID int64, key string) (string, error)
	GetEncryptedSettingForUser(userID int64, key string) (string, error)
	GetAIProfile(id int64) (*models.AIProfile, error)
	GetDefaultAIProfile() (*models.AIProfile, error)
	GetSetting(key string) (string, error)
}

// NewProfileProvider creates a new ProfileProvider
func NewProfileProvider(db ProfileDB) *ProfileProvider {
	return &ProfileProvider{db: db}
}

// FeatureType represents different AI features that can have separate profile configurations
type FeatureType string

const (
	FeatureTranslation    FeatureType = "translation"
	FeatureSummary        FeatureType = "summary"
	FeatureChat           FeatureType = "chat"
	FeatureSearch         FeatureType = "search"
	FeatureFusion         FeatureType = "fusion"
	FeatureRecommendation FeatureType = "recommendation"
)

// GetProfileForFeature returns the AI profile configured for a specific feature
// Falls back to default profile if no specific profile is configured
func (p *ProfileProvider) GetProfileForFeature(feature FeatureType) (*models.AIProfile, error) {
	// Get the setting key for this feature
	settingKey := p.getSettingKeyForFeature(feature)

	// Try to get the configured profile ID
	profileIDStr, err := p.db.GetSetting(settingKey)
	if err == nil && profileIDStr != "" {
		profileID, err := strconv.ParseInt(profileIDStr, 10, 64)
		if err == nil && profileID > 0 {
			profile, err := p.db.GetAIProfile(profileID)
			if err == nil && profile != nil {
				return profile, nil
			}
		}
	}

	// Fallback to default profile
	profile, err := p.db.GetDefaultAIProfile()
	if err != nil {
		// If GetDefaultAIProfile returns an error (like sql.ErrNoRows), just return nil, nil
		// so that the caller can fall back to legacy settings
		return nil, nil
	}
	return profile, nil
}

// GetProfileForFeatureForUser returns the user-scoped AI profile configured for a feature.
// It only resolves user settings and user profiles.
func (p *ProfileProvider) GetProfileForFeatureForUser(userID int64, feature FeatureType) (*models.AIProfile, error) {
	if userID <= 0 {
		return nil, nil
	}

	settingKey := p.getSettingKeyForFeature(feature)

	profileIDStr, err := p.db.GetSettingForUser(userID, settingKey)
	if err == nil && profileIDStr != "" {
		profileID, err := strconv.ParseInt(profileIDStr, 10, 64)
		if err == nil && profileID > 0 {
			profile, err := p.db.GetAIProfileForUser(userID, profileID)
			if err == nil && profile != nil {
				return profile, nil
			}
		}
	}

	profile, err := p.db.GetDefaultAIProfileForUser(userID)
	if err != nil {
		return nil, nil
	}
	return profile, nil
}

// getSettingKeyForFeature returns the settings key for a feature's AI profile
func (p *ProfileProvider) getSettingKeyForFeature(feature FeatureType) string {
	switch feature {
	case FeatureTranslation:
		return "ai_translation_profile_id"
	case FeatureSummary:
		return "ai_summary_profile_id"
	case FeatureChat:
		return "ai_chat_profile_id"
	case FeatureSearch:
		return "ai_search_profile_id"
	case FeatureFusion:
		return "ai_fusion_profile_id"
	case FeatureRecommendation:
		return "ai_recommendation_profile_id"
	default:
		return ""
	}
}

// GetConfigForFeature returns the AI client config for a specific feature
// This is a convenience method that combines profile lookup with config creation
func (p *ProfileProvider) GetConfigForFeature(feature FeatureType) (*ClientConfig, error) {
	profile, err := p.GetProfileForFeature(feature)
	if err != nil || profile == nil {
		// No profile configured or error occurred, return nil so caller can fallback
		return nil, nil
	}

	cfg := &ClientConfig{
		APIKey:        profile.APIKey,
		Endpoint:      profile.Endpoint,
		Model:         profile.Model,
		CustomHeaders: profile.CustomHeaders, // Keep as string, will be parsed by client
	}

	return cfg, nil
}

// GetConfigForFeatureForUser returns a user-scoped AI config for a feature.
// It first resolves the user's selected/default profile, then falls back to legacy user settings.
func (p *ProfileProvider) GetConfigForFeatureForUser(userID int64, feature FeatureType) (*ClientConfig, error) {
	profile, err := p.GetProfileForFeatureForUser(userID, feature)
	if err != nil {
		return nil, err
	}
	if profile != nil {
		return &ClientConfig{
			APIKey:        profile.APIKey,
			Endpoint:      profile.Endpoint,
			Model:         profile.Model,
			CustomHeaders: profile.CustomHeaders,
		}, nil
	}

	return p.getLegacyConfigForUser(userID), nil
}

// UseGlobalProxyForFeature returns whether the profile for a feature should use global proxy
// Returns true by default if no profile is configured
func (p *ProfileProvider) UseGlobalProxyForFeature(feature FeatureType) bool {
	profile, err := p.GetProfileForFeature(feature)
	if err != nil || profile == nil {
		// Default to using global proxy when no profile is configured
		return true
	}
	return profile.UseGlobalProxy
}

// UseGlobalProxyForFeatureForUser returns whether the user-scoped feature config should use global proxy.
// Legacy user settings default to using global proxy.
func (p *ProfileProvider) UseGlobalProxyForFeatureForUser(userID int64, feature FeatureType) bool {
	profile, err := p.GetProfileForFeatureForUser(userID, feature)
	if err != nil || profile == nil {
		return true
	}
	return profile.UseGlobalProxy
}

// HasProfileConfigured checks if a specific profile is configured for a feature
func (p *ProfileProvider) HasProfileConfigured(feature FeatureType) bool {
	settingKey := p.getSettingKeyForFeature(feature)
	profileIDStr, err := p.db.GetSetting(settingKey)
	if err != nil || profileIDStr == "" {
		return false
	}
	profileID, err := strconv.ParseInt(profileIDStr, 10, 64)
	if err != nil || profileID <= 0 {
		return false
	}
	// Verify the profile actually exists
	profile, err := p.db.GetAIProfile(profileID)
	return err == nil && profile != nil
}

// HasProfileConfiguredForUser checks whether a user explicitly configured a valid profile for a feature.
func (p *ProfileProvider) HasProfileConfiguredForUser(userID int64, feature FeatureType) bool {
	if userID <= 0 {
		return false
	}

	settingKey := p.getSettingKeyForFeature(feature)
	profileIDStr, err := p.db.GetSettingForUser(userID, settingKey)
	if err != nil || profileIDStr == "" {
		return false
	}
	profileID, err := strconv.ParseInt(profileIDStr, 10, 64)
	if err != nil || profileID <= 0 {
		return false
	}
	profile, err := p.db.GetAIProfileForUser(userID, profileID)
	return err == nil && profile != nil
}

func (p *ProfileProvider) getLegacyConfigForUser(userID int64) *ClientConfig {
	if userID <= 0 {
		return nil
	}

	apiKey, _ := p.db.GetEncryptedSettingForUser(userID, "ai_api_key")
	endpoint, _ := p.db.GetSettingForUser(userID, "ai_endpoint")
	model, _ := p.db.GetSettingForUser(userID, "ai_model")
	customHeaders, _ := p.db.GetSettingForUser(userID, "ai_custom_headers")

	if apiKey == "" && endpoint == "" && model == "" && customHeaders == "" {
		return nil
	}

	return &ClientConfig{
		APIKey:        apiKey,
		Endpoint:      endpoint,
		Model:         model,
		CustomHeaders: customHeaders,
	}
}
