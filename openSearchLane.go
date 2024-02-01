package lane

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/opensearch-project/opensearch-go/v3"
	"github.com/opensearch-project/opensearch-go/v3/opensearchapi"
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
}

type openSearchConnection struct {
	client             *opensearchapi.Client
	mu                 sync.Mutex
	logBuffer          []*OslMessage
	stopCh             chan *sync.WaitGroup
	wakeCh             chan bool
	clientCh           chan *sync.WaitGroup
	refCount           int
	config             *OslConfig
	emergencyFn        OslEmergencyFn
	ctx                context.Context
	cancelFn           context.CancelFunc
	messagesQueued     int
	messagesSent       int
	messagesSentFailed int
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
			wakeCh:    make(chan bool, 1),
			clientCh:  make(chan *sync.WaitGroup, 1),
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
	}

	err := osl.connect(config)
	if err != nil {
		osl.Error(err)
	}

	go osl.openSearchConnection.processConnection()

	l = &osl

	return

}

func (osl *openSearchLane) connect(config *OslConfig) (err error) {
	var client *opensearchapi.Client

	if !config.offline && osl.openSearchConnection.client != nil && osl.openSearchConnection.client.Client != nil {
		var wg sync.WaitGroup
		wg.Add(1)
		osl.openSearchConnection.clientCh <- &wg
		wg.Wait()
	}

	if !config.offline {
		client, err = newOpenSearchClient(config.OpenSearchUrl, config.OpenSearchPort, config.OpenSearchUser, config.OpenSearchPass, config.OpenSearchTransport)
		if err != nil {
			return
		}
	}
	osl.openSearchConnection.client = client
	osl.openSearchConnection.config = config

	if config.LogThreshold == 0 {
		osl.openSearchConnection.config.LogThreshold = OslDefaultLogThreshold
	}
	if config.MaxBufferSize == 0 {
		osl.openSearchConnection.config.MaxBufferSize = OslDefaultMaxBufferSize
	}
	if config.BackoffInterval == 0 {
		osl.openSearchConnection.config.BackoffInterval = OslDefaultBackoffInterval
	}
	if config.BackOffLimit == 0 {
		osl.openSearchConnection.config.BackOffLimit = OslDefaultBackOffLimit
	}

	return

}

func (osl *openSearchLane) Reconnect(config *OslConfig) (err error) {
	return osl.connect(config)
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
	osl.openSearchConnection.mu.Lock()
	defer osl.openSearchConnection.mu.Unlock()

	if osl.metadata == nil {
		osl.metadata = make(map[string]string)
	}

	osl.metadata[key] = val
}

func (osl *openSearchLane) Close() {
	osl.openSearchConnection.detach()
}

func (osl *openSearchLane) Write(p []byte) (n int, err error) {
	osl.openSearchConnection.mu.Lock()
	defer osl.openSearchConnection.mu.Unlock()

	var mapCopy map[string]string

	logEntry := string(p)

	logEntry = strings.ReplaceAll(logEntry, "\n", " ")

	// TODO remove this line
	fmt.Println(logEntry)

	parentLaneId, _ := osl.Context.Value(parent_lane_id).(string)

	if len(osl.metadata) > 0 {
		mapCopy = make(map[string]string, len(osl.metadata))
		for k, v := range osl.metadata {
			mapCopy[k] = v
		}
		mapCopy["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	}

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
		osl.openSearchConnection.wakeCh <- false
	}

	return len(p), nil
}

func (osl *openSearchLane) Stats() (stats OslStats) {
	osl.openSearchConnection.mu.Lock()
	defer osl.openSearchConnection.mu.Unlock()

	stats.MessagesQueued = osl.openSearchConnection.messagesQueued
	stats.MessagesSent = osl.openSearchConnection.messagesSent
	stats.MessagesSentFailed = osl.openSearchConnection.messagesSentFailed

	return
}

func (osl *openSearchLane) SetEmergencyHandler(emergencyFn OslEmergencyFn) {
	osl.openSearchConnection.mu.Lock()
	defer osl.openSearchConnection.mu.Unlock()
	osl.openSearchConnection.emergencyFn = emergencyFn
}

func (osc *openSearchConnection) processConnection() {
	backoffDuration := osc.config.BackoffInterval

	for {
		select {
		case wg := <-osc.stopCh:
			shouldStop := false
			osc.mu.Lock()
			osc.refCount--
			shouldStop = (osc.refCount == 0)
			osc.mu.Unlock()

			if shouldStop {
				go func() {
					osc.mu.Lock()
					if len(osc.logBuffer) > 0 {
						err := osc.flush(osc.logBuffer)
						if err != nil {
							if osc.emergencyFn != nil {
								err = osc.emergencyFn(osc.logBuffer)
								if err != nil {
									osc.messagesSentFailed += len(osc.logBuffer)
								} else {
									osc.messagesSent += len(osc.logBuffer)
								}
							}
						}
						osc.logBuffer = make([]*OslMessage, 0)
					}
					osc.mu.Unlock()
					wg.Done()
				}()
				return
			}

			wg.Done()

		case condition := <-osc.wakeCh:
			if condition {
				backoffDuration = osc.send(backoffDuration)
			} else {
				if !osc.config.offline {
					backoffDuration = osc.send(backoffDuration)
				}
			}

		case <-time.After(backoffDuration):
			if !osc.config.offline {
				backoffDuration = osc.send(backoffDuration)
			}

		case wg := <-osc.clientCh:
			backoffDuration = osc.send(backoffDuration)
			osc.client = nil
			wg.Done()

		}
	}

}

func (osc *openSearchConnection) send(backoffDuration time.Duration) time.Duration {
	osc.mu.Lock()
	logBuffer := osc.logBuffer
	osc.logBuffer = make([]*OslMessage, 0)
	osc.mu.Unlock()

	if len(logBuffer) > 0 {
		err := osc.flush(logBuffer)
		if err != nil {
			if len(logBuffer) > osc.config.MaxBufferSize {
				err = osc.emergencyFn(logBuffer)
				if err != nil {
					osc.messagesSentFailed += len(logBuffer)
				} else {
					osc.messagesSent += len(logBuffer)
				}
				backoffDuration = osc.config.BackoffInterval
			} else {
				backoffDuration *= 2
				if backoffDuration > osc.config.BackOffLimit {
					backoffDuration = osc.config.BackOffLimit
				}
				osc.mu.Lock()
				osc.logBuffer = append(logBuffer, osc.logBuffer...)
				osc.mu.Unlock()
			}
		} else {
			osc.messagesSent += len(logBuffer)
			backoffDuration = osc.config.BackoffInterval
			osc.messagesQueued = 0
		}
	}

	return backoffDuration
}

func (osc *openSearchConnection) attach() {
	osc.mu.Lock()
	defer osc.mu.Unlock()
	osc.refCount++
}

func (osc *openSearchConnection) detach() {
	var wg sync.WaitGroup
	wg.Add(1)
	osc.stopCh <- &wg
	wg.Wait()
}

func (osc *openSearchConnection) flush(logBuffer []*OslMessage) (err error) {
	if len(logBuffer) > 0 {
		err = osc.bulkInsert(logBuffer)
		if err != nil {
			return
		}
	}
	return
}

func (osc *openSearchConnection) bulkInsert(logBuffer []*OslMessage) (err error) {

	jsonData, err := osc.generateBulkJson(logBuffer)
	if err != nil {
		return
	}

	resp, err := osc.client.Bulk(context.Background(), opensearchapi.BulkReq{Body: strings.NewReader(jsonData)})
	if err != nil {
		osc.emergencyLog("Error while storing values in opensearch: %v", err)
		return
	}

	//TODO remove this line
	fmt.Println(resp)

	return
}

func (osc *openSearchConnection) generateBulkJson(logBuffer []*OslMessage) (jsonData string, err error) {
	var lines []string
	var createLine []byte
	var logDataLine []byte

	for _, logData := range logBuffer {
		createAction := map[string]interface{}{"create": map[string]interface{}{"_index": osc.config.OpenSearchIndex}}
		createLine, err = json.Marshal(createAction)
		if err != nil {
			osc.emergencyLog("Error marshalling createAction JSON: %v", err)
			return
		}

		logDataLine, err = json.Marshal(logData)
		if err != nil {
			osc.emergencyLog("Error marshalling logData JSON: %v", err)
			return
		}

		lines = append(lines, string(createLine), string(logDataLine))
	}

	jsonData = strings.Join(lines, "\n") + "\n"

	return
}

func (osc *openSearchConnection) emergencyLog(formatStr string, args ...any) {
	msg := fmt.Sprintf(formatStr, args...)

	oslm := &OslMessage{
		AppName:    "OpenSearchLane",
		LogMessage: msg,
	}

	oslm.Metadata = make(map[string]string)
	oslm.Metadata["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	logBuffer := []*OslMessage{oslm}

	if osc.emergencyFn != nil {
		err := osc.emergencyFn(logBuffer)
		if err != nil {
			osc.messagesSentFailed += len(logBuffer)
		} else {
			osc.messagesSent += len(logBuffer)
		}
	}
}

func newOpenSearchClient(openSearchUrl, openSearchPort, openSearchUser, openSearchPass string, openSearchTransport *http.Transport) (client *opensearchapi.Client, err error) {
	client, err = opensearchapi.NewClient(
		opensearchapi.Config{
			Client: opensearch.Config{
				Transport: openSearchTransport,
				Addresses: []string{fmt.Sprintf("%s:%s", openSearchUrl, openSearchPort)},
				Username:  openSearchUser,
				Password:  openSearchPass,
			},
		},
	)
	return
}
