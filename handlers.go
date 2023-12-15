package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/hako/durafmt"
	"golang.org/x/exp/slog"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// AlertmanagerPayload represents the payload received from Alertmanager.
type AlertmanagerPayload struct {
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"`
	Alerts            []Alert           `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
}

// Alert represents a single alert.
type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     string            `json:"startsAt"`
	EndsAt       string            `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// MattermostMessage represents a message to be sent to Mattermost.
type MattermostMessage struct {
	Text        string       `json:"text"`
	Username    string       `json:"username"`
	IconEmoji   string       `json:"icon_emoji"`
	Attachments []Attachment `json:"attachments"`
	Channel     string       `json:"channel"`
}

// Field represents a single field in an attachment.
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Attachment represents a message attachment.
type Attachment struct {
	Color     string  `json:"color"`
	Text      string  `json:"text"`
	Title     string  `json:"title"`
	TitleLink string  `json:"title_link"`
	Fields    []Field `json:"fields"`
}

const (
	colorFiring   = "#FF0000"
	colorResolved = "#008000"
	colorExpired  = "#F0F8FF"
)

var (
	serverAddr    string
	mattermostURL string
	verbose       bool
	logger        *slog.Logger
)

// prepareMessage converts AlertmanagerPayload to MattermostMessage.
func prepareMessage(payload AlertmanagerPayload, channel string) MattermostMessage {
	attachments := make([]Attachment, 0, len(payload.Alerts))
	for _, alert := range payload.Alerts {
		attachment := Attachment{
			Color:  setColor(alert.Status),
			Fields: convertAlertToFields(alert, payload.ExternalURL, payload.Receiver),
		}
		attachments = append(attachments, attachment)
	}
	return MattermostMessage{Attachments: attachments, Username: "alertmatter", IconEmoji: ":bell:", Channel: channel}
}

// sendToMattermost sends a MattermostMessage to the Mattermost server.
func sendToMattermost(mmMessage MattermostMessage, url string) error {
	jsonData, err := json.Marshal(mmMessage)
	if err != nil {
		logger.Error("Error marshalling JSON", "err", err)
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Error sending request to Mattermost", "err", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response from Mattermost: %s", resp.Status)
	}

	return nil
}

// handleAlert processes an incoming alert and sends it to Mattermost.
func handleAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	channel := r.URL.Query().Get("channel")
	if channel == "" {
		http.Error(w, "channel query parameter is required", http.StatusBadRequest)
		return
	}

	var payload AlertmanagerPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logger.Info("Received alert notification", "channel", channel)

	mmMessage := prepareMessage(payload, channel)
	if err := sendToMattermost(mmMessage, mattermostURL); err != nil {
		logger.Error("Failed to send to Mattermost", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func setColor(status string) string {
	switch status {
	case "firing":
		return colorFiring
	case "resolved":
		return colorResolved
	default:
		return colorExpired
	}
}

// convertAlertToFields converts an Alert to a slice of Fields.
// Modified version of https://github.com/cpanato/mattermost-plugin-alertmanager/blob/main/server/webhook.go#L79
func convertAlertToFields(alert Alert, externalURL, receiver string) []Field {
	var fields []Field

	statusMsg := strings.ToUpper(alert.Status)
	if alert.Status == "firing" {
		statusMsg = fmt.Sprintf(":fire: %s :fire:", strings.ToUpper(alert.Status))
	}

	// Annotations, Start/End, Source
	var msg string
	annotations := make([]string, 0, len(alert.Annotations))
	for k := range alert.Annotations {
		annotations = append(annotations, k)
	}
	sort.Strings(annotations)
	for _, k := range annotations {
		msg = fmt.Sprintf("%s**%s:** %s\n", msg, cases.Title(language.Und, cases.NoLower).String(k), alert.Annotations[k])
	}
	msg = fmt.Sprintf("%s**Started at:** %s (%s ago)\n", msg,
		alert.StartsAt,
		durafmt.Parse(time.Since(time.Now())).LimitFirstN(2).String(),
	)
	if alert.Status == "resolved" {
		msg = fmt.Sprintf("%s**Ended at:** %s (%s ago)\n", msg,
			alert.EndsAt,
			durafmt.Parse(time.Since(time.Now())).LimitFirstN(2).String(),
		)
	}
	msg = fmt.Sprintf("%sGenerated by a [Prometheus Alert](%s) and sent to the [Alertmanager](%s) '%s' receiver.", msg, alert.GeneratorURL, externalURL, receiver)
	fields = append(fields, Field{
		Title: statusMsg,
		Value: msg,
		Short: true,
	})

	// Labels
	msg = ""
	for k, v := range alert.Labels {
		msg = fmt.Sprintf("%s**%s:** %s\n", msg, cases.Title(language.Und, cases.NoLower).String(k), v)
	}
	fields = append(fields, Field{
		Title: "",
		Value: msg,
		Short: true,
	})

	return fields
}
