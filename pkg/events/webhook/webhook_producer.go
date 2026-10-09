package webhook_producer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	producer_interfaces "github.com/EvolutionAPI/evolution-go/pkg/events/interfaces"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
)

type webhookProducer struct {
	url           string
	secret        string // global HMAC secret (WEBHOOK_HMAC_SECRET); empty = unsigned
	loggerWrapper *logger_wrapper.LoggerManager
}

// SignedProducer is implemented by the webhook producer: it sends the payload
// signed with a per-instance secret (falling back to the global one).
type SignedProducer interface {
	ProduceWithSecret(queueName string, payload []byte, webhookUrl string, userID string, instanceSecret string) error
}

// NewWebhookProducer creates the webhook producer. url is the global webhook
// (WEBHOOK_URL) and secret the global HMAC secret (WEBHOOK_HMAC_SECRET); when a
// secret applies, every POST carries X-Evo-Signature / X-Webhook-Signature
// ("sha256=<hex HMAC-SHA256 of the raw body>") and X-Evo-Timestamp.
func NewWebhookProducer(
	url string,
	secret string,
	loggerWrapper *logger_wrapper.LoggerManager,
) producer_interfaces.Producer {
	return &webhookProducer{
		url:           url,
		secret:        secret,
		loggerWrapper: loggerWrapper,
	}
}

func (p *webhookProducer) Produce(
	queueName string,
	payload []byte,
	webhookUrl string,
	userID string,
) error {
	return p.ProduceWithSecret(queueName, payload, webhookUrl, userID, "")
}

// ProduceWithSecret sends payload to the global webhook (signed with the global
// secret) and to webhookUrl (signed with instanceSecret, or the global secret
// when instanceSecret is empty).
func (p *webhookProducer) ProduceWithSecret(
	queueName string,
	payload []byte,
	webhookUrl string,
	userID string,
	instanceSecret string,
) error {
	splitQueue := strings.Split(queueName, ".")

	if len(splitQueue) < 2 {
		return nil
	}

	if p.url != "" {
		go p.sendWebhookWithRetry(p.url, payload, p.secret, 5, 30*time.Second, userID)
	}
	if webhookUrl != "" {
		go p.sendWebhookWithRetry(webhookUrl, payload, p.secretFor(instanceSecret), 5, 30*time.Second, userID)
	}

	return nil
}

func (p *webhookProducer) secretFor(instanceSecret string) string {
	if instanceSecret != "" {
		return instanceSecret
	}
	return p.secret
}

func (p *webhookProducer) sendWebhookWithRetry(url string, body []byte, secret string, maxRetries int, retryInterval time.Duration, userID string) {
	for i := 0; i < maxRetries; i++ {
		err, responseBody, statusCode := p.sendWebhook(url, body, secret, userID)
		if err == nil {
			p.loggerWrapper.GetLogger(userID).LogInfo("[%s] webhook sent successfully - url: %s, status: %d, response: %s", userID, url, statusCode, string(responseBody))
			return
		}
		p.loggerWrapper.GetLogger(userID).LogWarn("[%s] webhook failed - url: %s, attempt: %d, error: %v", userID, url, i+1, err)

		time.Sleep(retryInterval)
	}
	p.loggerWrapper.GetLogger(userID).LogError("[%s] webhook failed after maximum retries - url: %s", userID, url)
}

func (p *webhookProducer) sendWebhook(url string, body []byte, secret string, userID string) (error, []byte, int) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err, nil, 0
	}

	req.Header.Set("Content-Type", "application/json")
	applySignature(req, secret, body, time.Now())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err, nil, 0
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("erro ao ler resposta: %v", err), nil, 0
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("received non-2xx response: " + resp.Status), responseBody, resp.StatusCode
	}

	return nil, responseBody, resp.StatusCode
}

// CreateGlobalQueues não faz nada para webhook producer
func (p *webhookProducer) CreateGlobalQueues() error {
	return nil
}
