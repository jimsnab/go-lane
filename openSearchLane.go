package lane

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
)

// TODO change default values
const (
	OslDefaultLogThreshold    = 5
	OslDefaultMaxBufferSize   = 10
	OslDefaultBackoffInterval = 10 * time.Second
	OslDefaultBackOffLimit    = 10 * time.Minute
)

type OslMessage struct {
	AppName      string            `json:"appName"`
	ParentLaneId string            `json:"parentLaneId,omitempty"`
	JourneyID    string            `json:"journeyId,omitempty"`
	LaneID       string            `json:"laneId,omitempty"`
	LogMessage   string            `json:"logMessage,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type OslEmergencyFn func(logBuffer []*OslMessage) (err error)

type OslStats struct {
	MessagesQueued     int `json:"messagesQueued"`
	MessagesSent       int `json:"messagesSent"`
	MessagesSentFailed int `json:"messagesSentFailed"`
}

type OslConfig struct {
	offline             bool
	OpenSearchUrl       string          `json:"openSearchUrl"`
	OpenSearchPort      string          `json:"openSearchPort"`
	OpenSearchUser      string          `json:"openSearchUser"`
	OpenSearchPass      string          `json:"openSearchPass"`
	OpenSearchIndex     string          `json:"openSearchIndex"`
	OpenSearchAppName   string          `json:"openSearchAppName"`
	OpenSearchTransport *http.Transport `json:"openSearchTransport"`
	LogThreshold        int             `json:"logThreshold,omitempty"`
	MaxBufferSize       int             `json:"maxBufferSize,omitempty"`
	BackoffInterval     time.Duration   `json:"backoffInterval,omitempty"`
	BackOffLimit        time.Duration   `json:"backoffLimit,omitempty"`
}

type openSearchLane struct {
	logLane
	openSearchConnection *openSearchConnection
	metadata             map[string]string
	mu                   sync.Mutex
}

type OpenSearchLane interface {
	Lane
	LaneMetadata
	Reconnect(config *OslConfig) (err error)
	SetEmergencyHandler(emergencyFn OslEmergencyFn)
	Stats() (stats OslStats)
}

func NewOpenSearchLane(ctx context.Context, config *OslConfig) (l OpenSearchLane) {
	ll := deriveLogLane(nil, ctx, []Lane{}, "")

	ctx, cancelFn := context.WithCancel(context.Background())

	osl := openSearchLane{
		openSearchConnection: &openSearchConnection{
			logBuffer: make([]*OslMessage, 0),
			stopCh:    make(chan *sync.WaitGroup, 1),
			wakeCh:    make(chan struct{}, 1),
			clientCh:  make(chan clientChParams, 1),
			ctx:       ctx,
			cancelFn:  cancelFn,
		},
	}
	osl.openSearchConnection.attach()

	ll.setFlagsMask(log.Ldate | log.Ltime)
	ll.clone(&osl.logLane)
	osl.logLane.writer = log.New(&osl, "", 0)

	val := reflect.ValueOf(config)
	if val.Kind() != reflect.Struct {
		config = &OslConfig{
			offline: true,
		}
		osl.openSearchConnection.config = config
	}

	go osl.openSearchConnection.processConnection()

	err := osl.openSearchConnection.connect(config)
	if err != nil {
		osl.Error(err)
	}

	l = &osl

	return

}

func (osl *openSearchLane) Reconnect(config *OslConfig) (err error) {
	osl.openSearchConnection.laneMu.Lock()
	defer osl.openSearchConnection.laneMu.Unlock()
	return osl.openSearchConnection.connect(config)
}

func (osl *openSearchLane) Derive() Lane {
	ll := deriveLogLane(&osl.logLane, context.WithValue(osl.Context, parent_lane_id, osl.LaneId()), osl.tees, osl.cr)

	osl2 := finishDerive(osl, ll)

	return osl2
}

func (osl *openSearchLane) DeriveWithCancel() (Lane, context.CancelFunc) {
	childCtx, cancelFn := context.WithCancel(context.WithValue(osl.logLane.Context, parent_lane_id, osl.logLane.LaneId()))
	ll := deriveLogLane(&osl.logLane, childCtx, osl.tees, osl.cr)

	osl2 := finishDerive(osl, ll)

	return osl2, cancelFn
}

func (osl *openSearchLane) DeriveReplaceContext(ctx context.Context) Lane {
	ll := deriveLogLane(&osl.logLane, context.WithValue(ctx, parent_lane_id, osl.LaneId()), osl.tees, osl.cr)

	osl2 := finishDerive(osl, ll)

	return osl2
}

func finishDerive(osl *openSearchLane, ll *logLane) *openSearchLane {
	osl2 := openSearchLane{
		openSearchConnection: osl.openSearchConnection,
	}
	osl.openSearchConnection.attach()

	ll.setFlagsMask(log.Ldate | log.Ltime)
	ll.clone(&osl2.logLane)
	osl2.logLane.writer = log.New(&osl2, "", 0)

	return &osl2
}

func (osl *openSearchLane) SetMetadata(key string, val string) {
	osl.mu.Lock()
	defer osl.mu.Unlock()

	if osl.metadata == nil {
		osl.metadata = make(map[string]string)
	}

	osl.metadata[key] = val
}

func (osl *openSearchLane) Close() {
	osl.openSearchConnection.detach()
}

func (osl *openSearchLane) Write(p []byte) (n int, err error) {
	osl.mu.Lock()
	defer osl.mu.Unlock()

	logEntry := string(p)

	logEntry = strings.ReplaceAll(logEntry, "\n", " ")

	// TODO remove this line
	fmt.Println(logEntry)

	parentLaneId, _ := osl.Context.Value(parent_lane_id).(string)

	mapCopy := make(map[string]string, len(osl.metadata)+1)
	if len(osl.metadata) > 0 {
		for k, v := range osl.metadata {
			mapCopy[k] = v
		}
	}
	mapCopy["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	logData := &OslMessage{
		AppName:      osl.openSearchConnection.config.OpenSearchAppName,
		ParentLaneId: parentLaneId,
		JourneyID:    osl.journeyId,
		LaneID:       osl.LaneId(),
		LogMessage:   logEntry,
		Metadata:     mapCopy,
	}

	osl.openSearchConnection.logBuffer = append(osl.openSearchConnection.logBuffer, logData)

	osl.openSearchConnection.messagesQueued++

	if ((1 + osl.openSearchConnection.messagesSent - osl.openSearchConnection.messagesQueued) % osl.openSearchConnection.config.LogThreshold) == 0 {
		osl.openSearchConnection.wakeCh <- struct{}{}
	}

	return len(p), nil
}

func (osl *openSearchLane) Stats() (stats OslStats) {
	osl.mu.Lock()
	defer osl.mu.Unlock()

	stats.MessagesQueued = osl.openSearchConnection.messagesQueued
	stats.MessagesSent = osl.openSearchConnection.messagesSent
	stats.MessagesSentFailed = osl.openSearchConnection.messagesSentFailed

	return
}

func (osl *openSearchLane) SetEmergencyHandler(emergencyFn OslEmergencyFn) {
	osl.mu.Lock()
	defer osl.mu.Unlock()
	osl.openSearchConnection.emergencyFn = emergencyFn
}
