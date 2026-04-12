package domain

import (
	"time"

	"github.com/google/uuid"
)

type ReceiverType string

const (
	ReceiverTypeWebhook   ReceiverType = "webhook"
	ReceiverTypeEmail     ReceiverType = "email"
	ReceiverTypeSlack     ReceiverType = "slack"
	ReceiverTypeDingTalk  ReceiverType = "dingtalk"
	ReceiverTypePagerDuty ReceiverType = "pagerduty"
)

type Receiver struct {
	ID        uuid.UUID     `json:"id"`
	Name      string        `json:"name"`
	Type      ReceiverType  `json:"type"`
	Config    ReceiverConfig `json:"config"`
	Active    bool          `json:"active"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type ReceiverConfig struct {
	WebhookURL   string            `json:"webhook_url,omitempty"`
	EmailConfig  *EmailConfig      `json:"email_config,omitempty"`
	SlackConfig  *SlackConfig      `json:"slack_config,omitempty"`
	DingTalkConfig *DingTalkConfig `json:"dingtalk_config,omitempty"`
	PagerDutyConfig *PagerDutyConfig `json:"pagerduty_config,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
}

type EmailConfig struct {
	To           string `json:"to"`
	From         string `json:"from"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUser     string `json:"smtp_user,omitempty"`
	SMTPPassword string `json:"smtp_password,omitempty"`
}

type SlackConfig struct {
	Channel     string `json:"channel"`
	WebhookURL  string `json:"webhook_url"`
	Username    string `json:"username,omitempty"`
	IconEmoji   string `json:"icon_emoji,omitempty"`
}

type DingTalkConfig struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret,omitempty"`
	AtMobiles  []string `json:"at_mobiles,omitempty"`
	AtAll      bool   `json:"at_all,omitempty"`
}

type PagerDutyConfig struct {
	ServiceKey  string `json:"service_key"`
	URL         string `json:"url,omitempty"`
	Severity    string `json:"severity,omitempty"`
}

func NewReceiver(name string, receiverType ReceiverType) *Receiver {
	now := time.Now()
	return &Receiver{
		ID:        uuid.New(),
		Name:      name,
		Type:      receiverType,
		Config:    ReceiverConfig{},
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (r *Receiver) SetWebhookConfig(url string, headers map[string]string) {
	r.Config.WebhookURL = url
	r.Config.Headers = headers
}

func (r *Receiver) SetDingTalkConfig(webhookURL, secret string, atMobiles []string, atAll bool) {
	r.Config.DingTalkConfig = &DingTalkConfig{
		WebhookURL: webhookURL,
		Secret:     secret,
		AtMobiles:  atMobiles,
		AtAll:      atAll,
	}
}

func (r *Receiver) SetSlackConfig(channel, webhookURL, username, iconEmoji string) {
	r.Config.SlackConfig = &SlackConfig{
		Channel:    channel,
		WebhookURL: webhookURL,
		Username:   username,
		IconEmoji:  iconEmoji,
	}
}

type ReceiverFilter struct {
	Type     ReceiverType `form:"type"`
	Active   *bool        `form:"active"`
	Page     int          `form:"page"`
	PageSize int          `form:"page_size"`
}
