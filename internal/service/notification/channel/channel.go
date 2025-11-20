package channel

import (
	"context"
	"fmt"
	"time"

	"myapp/internal/pkg/logger"
	"myapp/internal/service/notification/config"
	"myapp/internal/service/notification/model"
	"myapp/internal/service/notification/repository"

	expo "github.com/oliveroneill/exponent-server-sdk-golang/sdk"

	"go.uber.org/zap"
)

// ChannelResult represents the result of sending a notification
type ChannelResult struct {
	Success   bool
	Retryable bool
	Error     error
}

// Channel is the interface for notification channels
type Channel interface {
	Send(ctx context.Context, target *model.NotificationTarget, payload model.NotificationPayload) *ChannelResult
	Name() string
}

// ExpoChannel implements Expo push notification channel
type ExpoChannel struct {
	config *config.ExpoConfig
	client *expo.PushClient
	logger *logger.Logger
	repo   *repository.NotificationRepository
}

// NewExpoChannel creates a new Expo channel
func NewExpoChannel(config *config.ExpoConfig, log *logger.Logger, repo *repository.NotificationRepository) *ExpoChannel {
	// Create Expo PushClient with optional HTTP client configuration
	// If AccessToken is provided, it will be used for authentication
	var client *expo.PushClient
	if config.AccessToken != "" {
		// Note: The SDK may need custom HTTP client setup for access token
		// For now, we'll use the default client
		client = expo.NewPushClient(nil)
	} else {
		client = expo.NewPushClient(nil)
	}

	return &ExpoChannel{
		config: config,
		client: client,
		logger: log,
		repo:   repo,
	}
}

// Name returns the channel name
func (c *ExpoChannel) Name() string {
	return "expo"
}

// Send sends a notification via Expo
func (c *ExpoChannel) Send(ctx context.Context, target *model.NotificationTarget, payload model.NotificationPayload) *ChannelResult {
	if !c.config.Enabled {
		return &ChannelResult{
			Success:   false,
			Retryable: false,
			Error:     fmt.Errorf("expo channel is disabled"),
		}
	}

	// Get Expo push tokens from device_tokens table
	tokens, err := c.repo.GetDeviceTokensByUserID(target.UserID)
	if err != nil {
		return &ChannelResult{
			Success:   false,
			Retryable: false,
			Error:     fmt.Errorf("failed to get device tokens: %w", err),
		}
	}

	// Filter for Expo tokens only
	var expoTokens []string
	for _, token := range tokens {
		if token != nil && token.Type == "expo" && token.PushToken != "" {
			expoTokens = append(expoTokens, token.PushToken)
		}
	}

	if len(expoTokens) == 0 {
		return &ChannelResult{
			Success:   false,
			Retryable: false,
			Error:     fmt.Errorf("no expo push tokens found for user_id: %s", target.UserID),
		}
	}

	// Extract title and body from payload
	title := ""
	if t, ok := payload.Data["title"].(string); ok {
		title = t
	}
	body := ""
	if b, ok := payload.Data["body"].(string); ok {
		body = b
	}

	// Convert payload.Data to map[string]string for Expo SDK
	// Expo SDK requires Data to be map[string]string
	dataMap := make(map[string]string)
	for k, v := range payload.Data {
		if strVal, ok := v.(string); ok {
			dataMap[k] = strVal
		} else {
			// Convert non-string values to string
			dataMap[k] = fmt.Sprintf("%v", v)
		}
	}

	// Build Expo messages for all tokens using SDK
	var expoMessages []expo.PushMessage
	for _, tokenStr := range expoTokens {
		// Convert string token to ExponentPushToken
		token, err := expo.NewExponentPushToken(tokenStr)
		if err != nil {
			c.logger.Warn("Invalid Expo push token",
				zap.String("token", tokenStr),
				zap.Error(err),
			)
			continue
		}

		message := expo.PushMessage{
			To:    []expo.ExponentPushToken{token},
			Title: title,
			Body:  body,
			Data:  dataMap,
		}

		// Add sound if present
		if sound, ok := payload.Data["sound"].(string); ok && sound != "" {
			message.Sound = sound
		} else {
			message.Sound = "default"
		}

		// Add badge if present
		if badge, ok := payload.Data["badge"].(float64); ok {
			message.Badge = int(badge)
		}

		// Set priority
		message.Priority = expo.DefaultPriority
		if payload.Priority > 0 {
			message.Priority = "high"
		}

		// Add channel ID if present (for Android)
		if channelID, ok := payload.Data["channelId"].(string); ok && channelID != "" {
			message.ChannelID = channelID
		}

		expoMessages = append(expoMessages, message)
	}

	if len(expoMessages) == 0 {
		return &ChannelResult{
			Success:   false,
			Retryable: false,
			Error:     fmt.Errorf("no valid expo push tokens found for user_id: %s", target.UserID),
		}
	}

	// Send messages with retry
	var lastErr error
	for attempt := 0; attempt < c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-ctx.Done():
				return &ChannelResult{
					Success:   false,
					Retryable: true,
					Error:     ctx.Err(),
				}
			case <-time.After(backoff):
			}
		}

		// Send messages using SDK
		// Use PublishMultiple to send all messages in one batch
		responses, err := c.client.PublishMultiple(expoMessages)
		if err != nil {
			lastErr = err
			c.logger.Warn("Expo send attempt failed",
				zap.Int("attempt", attempt+1),
				zap.Int("message_count", len(expoMessages)),
				zap.Error(err),
			)
			// Check if error is retryable
			if attempt < c.config.MaxRetries-1 {
				continue
			}
			break
		}

		// Check if all messages were sent successfully
		if len(responses) == len(expoMessages) {
			allSuccess := true
			hasError := false
			for i, response := range responses {
				if response.Status != expo.SuccessStatus {
					allSuccess = false
					hasError = true
					lastErr = fmt.Errorf("expo response error: %s - %s", response.Status, response.Message)
					c.logger.Warn("Expo response error",
						zap.Int("message_index", i),
						zap.String("status", response.Status),
						zap.String("message", response.Message),
						zap.String("id", response.ID),
					)
				}
			}

			if allSuccess {
				c.logger.Info("Expo notification sent successfully",
					zap.Int64("target_id", target.ID),
					zap.String("user_id", target.UserID),
					zap.Int("token_count", len(expoTokens)),
				)
				return &ChannelResult{Success: true}
			}

			// If there are errors, check if retryable
			if hasError && lastErr != nil {
				// Check if error is retryable (rate limits, etc.)
				// DeviceNotRegistered errors are not retryable
				if attempt < c.config.MaxRetries-1 {
					// Check if it's a non-retryable error
					retryable := true
					for _, response := range responses {
						if response.Status != expo.SuccessStatus {
							// DeviceNotRegistered is not retryable
							if response.Message != "" &&
								(response.Status == expo.ErrorDeviceNotRegistered ||
									response.Status == expo.ErrorMessageTooBig) {
								retryable = false
								break
							}
						}
					}

					if retryable {
						c.logger.Warn("Expo send failed, retrying",
							zap.Int("attempt", attempt+1),
							zap.Error(lastErr),
						)
						continue
					} else {
						// Non-retryable error, return immediately
						return &ChannelResult{
							Success:   false,
							Retryable: false,
							Error:     lastErr,
						}
					}
				}
			}
		} else {
			// Unexpected: number of responses doesn't match messages
			lastErr = fmt.Errorf("expo response count mismatch: expected %d, got %d", len(expoMessages), len(responses))
			if attempt < c.config.MaxRetries-1 {
				c.logger.Warn("Expo send incomplete, retrying",
					zap.Int("attempt", attempt+1),
					zap.Int("expected", len(expoMessages)),
					zap.Int("received", len(responses)),
					zap.Error(lastErr),
				)
				continue
			}
		}
	}

	return &ChannelResult{
		Success:   false,
		Retryable: true,
		Error:     fmt.Errorf("expo send failed after %d attempts: %w", c.config.MaxRetries, lastErr),
	}
}

// FCMChannel implements Firebase Cloud Messaging channel (placeholder)
type FCMChannel struct {
	config *config.FCMConfig
	logger *logger.Logger
}

// NewFCMChannel creates a new FCM channel
func NewFCMChannel(config *config.FCMConfig, log *logger.Logger) *FCMChannel {
	return &FCMChannel{
		config: config,
		logger: log,
	}
}

// Name returns the channel name
func (c *FCMChannel) Name() string {
	return "fcm"
}

// Send sends a notification via FCM (placeholder implementation)
func (c *FCMChannel) Send(ctx context.Context, target *model.NotificationTarget, payload model.NotificationPayload) *ChannelResult {
	if !c.config.Enabled {
		return &ChannelResult{
			Success:   false,
			Retryable: false,
			Error:     fmt.Errorf("fcm channel is disabled"),
		}
	}

	// TODO: Implement FCM channel
	return &ChannelResult{
		Success:   false,
		Retryable: false,
		Error:     fmt.Errorf("fcm channel not implemented"),
	}
}

// APNSChannel implements Apple Push Notification Service channel (placeholder)
type APNSChannel struct {
	config *config.APNSConfig
	logger *logger.Logger
}

// NewAPNSChannel creates a new APNS channel
func NewAPNSChannel(config *config.APNSConfig, log *logger.Logger) *APNSChannel {
	return &APNSChannel{
		config: config,
		logger: log,
	}
}

// Name returns the channel name
func (c *APNSChannel) Name() string {
	return "apns"
}

// Send sends a notification via APNS (placeholder implementation)
func (c *APNSChannel) Send(ctx context.Context, target *model.NotificationTarget, payload model.NotificationPayload) *ChannelResult {
	if !c.config.Enabled {
		return &ChannelResult{
			Success:   false,
			Retryable: false,
			Error:     fmt.Errorf("apns channel is disabled"),
		}
	}

	// TODO: Implement APNS channel
	return &ChannelResult{
		Success:   false,
		Retryable: false,
		Error:     fmt.Errorf("apns channel not implemented"),
	}
}

// EmailChannel implements email channel (placeholder)
type EmailChannel struct {
	config *config.EmailConfig
	logger *logger.Logger
}

// NewEmailChannel creates a new email channel
func NewEmailChannel(config *config.EmailConfig, log *logger.Logger) *EmailChannel {
	return &EmailChannel{
		config: config,
		logger: log,
	}
}

// Name returns the channel name
func (c *EmailChannel) Name() string {
	return "email"
}

// Send sends a notification via email (placeholder implementation)
func (c *EmailChannel) Send(ctx context.Context, target *model.NotificationTarget, payload model.NotificationPayload) *ChannelResult {
	if !c.config.Enabled {
		return &ChannelResult{
			Success:   false,
			Retryable: false,
			Error:     fmt.Errorf("email channel is disabled"),
		}
	}

	// TODO: Implement email channel
	return &ChannelResult{
		Success:   false,
		Retryable: false,
		Error:     fmt.Errorf("email channel not implemented"),
	}
}

// ChannelRegistry manages available channels
type ChannelRegistry struct {
	channels map[string]Channel
	logger   *logger.Logger
}

// NewChannelRegistry creates a new channel registry
func NewChannelRegistry(config *config.ServiceConfig, log *logger.Logger, repo *repository.NotificationRepository) *ChannelRegistry {
	registry := &ChannelRegistry{
		channels: make(map[string]Channel),
		logger:   log,
	}

	// Register channels
	if config.Notification.Senders.Expo.Enabled {
		registry.channels["expo"] = NewExpoChannel(&config.Notification.Senders.Expo, log, repo)
	}
	if config.Notification.Senders.FCM.Enabled {
		registry.channels["fcm"] = NewFCMChannel(&config.Notification.Senders.FCM, log)
	}
	if config.Notification.Senders.APNS.Enabled {
		registry.channels["apns"] = NewAPNSChannel(&config.Notification.Senders.APNS, log)
	}
	if config.Notification.Senders.Email.Enabled {
		registry.channels["email"] = NewEmailChannel(&config.Notification.Senders.Email, log)
	}

	return registry
}

// GetChannel returns a channel by name
func (r *ChannelRegistry) GetChannel(name string) (Channel, bool) {
	channel, ok := r.channels[name]
	return channel, ok
}

// GetAllChannels returns all registered channels
func (r *ChannelRegistry) GetAllChannels() map[string]Channel {
	return r.channels
}

