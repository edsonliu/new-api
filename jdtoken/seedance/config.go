// Package seedance provides JDOTOKEN's optional material and task-management
// extension. It deliberately does not implement generation or billing.
package seedance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const ConfigKey = "JDOTOKENSeedance"
const APIVersion = "2024-01-01"

type ModelCapabilities struct {
	UpstreamModel  string   `json:"upstream_model,omitempty"`
	ReferenceTypes []string `json:"reference_types"`
	Cancel         bool     `json:"cancel"`
	Draft          bool     `json:"draft"`
	DraftUpgrade   bool     `json:"draft_upgrade"`
	MOV            bool     `json:"mov"`
	Frames         bool     `json:"frames"`
}
type Binding struct {
	Name                   string                       `json:"name"`
	ChannelID              int                          `json:"channel_id"`
	Provider               string                       `json:"provider"`
	Models                 map[string]ModelCapabilities `json:"models"`
	StoreTypes             []string                     `json:"store_types"`
	RealPersonVerification bool                         `json:"real_person_verification,omitempty"`
}
type Config struct {
	Enabled  bool      `json:"enabled"`
	Bindings []Binding `json:"bindings"`
}

var bindingName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func LoadConfig() (Config, error) {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[ConfigKey]
	common.OptionMapRWMutex.RUnlock()
	if raw == "" {
		raw = os.Getenv("JDOTOKEN_SEEDANCE_CONFIG")
	}
	if raw == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := common.UnmarshalJsonStr(raw, &cfg); err != nil {
		return cfg, errors.New("invalid seedance configuration")
	}
	seen := map[string]bool{}
	channels := map[int]bool{}
	for _, b := range cfg.Bindings {
		if !bindingName.MatchString(b.Name) || seen[b.Name] || channels[b.ChannelID] || b.ChannelID <= 0 || !slices.Contains([]string{"lingxi", "mobilecloud", "sdgo", "shujubao"}, b.Provider) {
			return Config{}, errors.New("invalid seedance binding")
		}
		if b.RealPersonVerification && b.Provider != "shujubao" {
			return Config{}, errors.New("real-person verification requires a verified provider")
		}
		seen[b.Name] = true
		channels[b.ChannelID] = true
		for name, caps := range b.Models {
			if name == "" || len(name) > 256 || len(caps.UpstreamModel) > 256 {
				return Config{}, errors.New("invalid model capability")
			}
			for _, kind := range caps.ReferenceTypes {
				if !slices.Contains([]string{"Image", "Video", "Audio"}, kind) {
					return Config{}, errors.New("invalid reference type")
				}
			}
		}
		for _, t := range b.StoreTypes {
			if !slices.Contains([]string{"Image", "Video", "Audio"}, t) {
				return Config{}, errors.New("invalid material type")
			}
		}
		if b.Provider == "sdgo" && len(b.StoreTypes) > 0 {
			return Config{}, errors.New("sdgo material management is not verified")
		}
	}
	return cfg, nil
}

func AccountHash(ch *model.Channel) string {
	digest := sha256.Sum256([]byte(strings.TrimRight(ch.GetBaseURL(), "/") + "\x00" + strings.TrimSpace(ch.Key)))
	return hex.EncodeToString(digest[:])
}

// Eligible enforces token limits and group permissions even for pinned requests:
// the upstream distributor intentionally exempts administrative/origin pins.
func Eligible(c *gin.Context, ch *model.Channel, name string) bool {
	if ch.Status != common.ChannelStatusEnabled || ch.ChannelInfo.IsMultiKey || strings.ContainsAny(strings.TrimSpace(ch.Key), "\r\n") || ch.Key == "" {
		return false
	}
	// Header overrides can change the generation account independently of Key.
	// Material bindings require a single, explicit credential source.
	for name := range ch.GetHeaderOverride() {
		if strings.EqualFold(name, "Authorization") || strings.EqualFold(name, "X-Api-Key") || strings.EqualFold(name, "Cookie") {
			return false
		}
	}
	if common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		limits, _ := c.Get("token_model_limit")
		allowed, _ := limits.(map[string]bool)
		if !allowed[name] && !allowed[ratio_setting.FormatMatchingModelName(name)] && !allowed[ratio_setting.RoutingMatchModelName(name)] {
			return false
		}
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if group == "auto" {
		for _, g := range service.GetRequestAutoGroups(c, common.GetContextKeyString(c, constant.ContextKeyUserGroup)) {
			if model.IsChannelEnabledForGroupModel(g, name, ch.Id) {
				return true
			}
		}
		return false
	}
	return model.IsChannelEnabledForGroupModel(group, name, ch.Id)
}

func modelMappingMatches(ch *model.Channel, name string, caps ModelCapabilities) bool {
	expected := caps.UpstreamModel
	if expected == "" {
		expected = name
	}
	mapping := map[string]string{}
	if ch.ModelMapping != nil && *ch.ModelMapping != "" {
		if common.UnmarshalJsonStr(*ch.ModelMapping, &mapping) != nil {
			return false
		}
	}
	actual := name
	for range 16 {
		next, ok := mapping[actual]
		if !ok || next == actual || next == "" {
			return actual == expected
		}
		actual = next
	}
	return false
}

func ResolveBinding(c *gin.Context, cfg Config, binding, name string) (Binding, *model.Channel, error) {
	for _, b := range cfg.Bindings {
		if b.Name != binding {
			continue
		}
		ch, err := model.GetChannelById(b.ChannelID, true)
		if err != nil {
			break
		}
		if name != "" {
			if caps, ok := b.Models[name]; !ok || !modelMappingMatches(ch, name, caps) || !Eligible(c, ch, name) {
				break
			}
		} else {
			permitted := false
			for m := range b.Models {
				if modelMappingMatches(ch, m, b.Models[m]) && Eligible(c, ch, m) {
					permitted = true
					break
				}
			}
			if !permitted {
				break
			}
		}
		u, err := url.Parse(ch.GetBaseURL())
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
			break
		}
		if ch.Type != constant.ChannelTypeDoubaoVideo && ch.Type != constant.ChannelTypeVolcEngine && !ch.GetSetting().BindsTaskPlugin("doubao") {
			break
		}
		return b, ch, nil
	}
	return Binding{}, nil, apiError(403, "binding_unavailable", "Service or model is not available for this token")
}
