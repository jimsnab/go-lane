package lane

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

type OpenSearchLogMessage struct {
	AppName      string            `json:"appName"`
	ParentLaneId string            `json:"parentLaneId,omitempty"`
	JourneyID    string            `json:"journeyId,omitempty"`
	LaneID       string            `json:"laneId,omitempty"`
	LogMessage   string            `json:"logMessage,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type OslEmergencyFn func(logBuffer []*OpenSearchLogMessage) (err error)

type OslStats struct {
	MessagesQueued     int `json:"messagesQueued"`
	MessagesSent       int `json:"messagesSent"`
	MessagesSentFailed int `json:"messagesSentFailed"`
}

type OslConfig struct {
	OpenSearchUrl     string        `json:"openSearchUrl"`
	OpenSearchPort    string        `json:"openSearchPort"`
	OpenSearchUser    string        `json:"openSearchUser"`
	OpenSearchPass    string        `json:"openSearchPass"`
	OpenSearchIndex   string        `json:"openSearchIndex"`
	OpenSearchAppName string        `json:"openSearchAppName"`
	LogThreshold      int           `json:"logThreshold,omitempty"`
	MaxBufferSize     int           `json:"maxBufferSize,omitempty"`
	BackoffInterval   time.Duration `json:"backoffInterval,omitempty"`
	BackOffLimit      time.Duration `json:"backoffLimit,omitempty"`
}

type openSearchLane struct {
	logLane
	openSearchConnection *openSearchConnection
	metadata             map[string]string
}

type openSearchConnection struct {
	client             *opensearchapi.Client
	mu                 sync.Mutex
	logBuffer          []*OpenSearchLogMessage
	stopCh             chan *sync.WaitGroup
	wakeCh             chan struct{}
	refCount           int
	config             OslConfig
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
	Connect(config OslConfig) (err error)
	SetEmergencyHandler(emergencyFn OslEmergencyFn)
	Stats() (stats OslStats)
}

func NewOpenSearchLane(ctx context.Context) (l OpenSearchLane) {
	ll := deriveLogLane(nil, ctx, []Lane{}, "")

	ctx, cancelFn := context.WithCancel(context.Background())

	osl := openSearchLane{
		openSearchConnection: &openSearchConnection{
			logBuffer: make([]*OpenSearchLogMessage, 0),
			stopCh:    make(chan *sync.WaitGroup, 1),
			wakeCh:    make(chan struct{}, 1),
			ctx:       ctx,
			cancelFn:  cancelFn,
		},
	}
	osl.openSearchConnection.attach()

	ll.setFlagsMask(log.Ldate | log.Ltime)
	ll.clone(&osl.logLane)
	osl.logLane.writer = log.New(&osl, "", 0)

	l = &osl

	return

}

func (osl *openSearchLane) Connect(config OslConfig) (err error) {
	if osl.openSearchConnection.client != nil {
		osl.openSearchConnection.wakeCh <- struct{}{}
	}

	client, err := newOpenSearchClient(config.OpenSearchUrl, config.OpenSearchPort, config.OpenSearchUser, config.OpenSearchPass)
	if err != nil {
		return
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

	go osl.openSearchConnection.processConnection(osl)

	return

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

	logEntry := string(p)

	if strings.Contains(logEntry, "\n") {
		logEntry = strings.ReplaceAll(logEntry, "\n", "")
	}

	// TODO remove this line
	fmt.Println(logEntry)

	parentLaneId, _ := osl.Context.Value(parent_lane_id).(string)

	if osl.metadata != nil {
		osl.metadata["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	}

	logData := &OpenSearchLogMessage{
		AppName:      osl.openSearchConnection.config.OpenSearchAppName,
		ParentLaneId: parentLaneId,
		JourneyID:    osl.journeyId,
		LaneID:       osl.LaneId(),
		LogMessage:   logEntry,
		Metadata:     osl.metadata,
	}

	osl.openSearchConnection.logBuffer = append(osl.openSearchConnection.logBuffer, logData)

	osl.openSearchConnection.messagesQueued++

	if ((1 + osl.openSearchConnection.messagesSent - osl.openSearchConnection.messagesQueued) % osl.openSearchConnection.config.LogThreshold) == 0 {
		osl.openSearchConnection.wakeCh <- struct{}{}
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

func (osc *openSearchConnection) processConnection(l Lane) {
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
						osc.logBuffer = make([]*OpenSearchLogMessage, 0)
					}
					osc.mu.Unlock()
					wg.Done()
				}()
				return
			}

			wg.Done()

		case <-osc.wakeCh:
			backoffDuration = osc.send(backoffDuration)
		case <-time.After(backoffDuration):
			backoffDuration = osc.send(backoffDuration)
		}
	}
}

func (osc *openSearchConnection) send(backoffDuration time.Duration) time.Duration {
	osc.mu.Lock()
	logBuffer := osc.logBuffer
	osc.logBuffer = make([]*OpenSearchLogMessage, 0)
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

func (osc *openSearchConnection) flush(logBuffer []*OpenSearchLogMessage) (err error) {
	if len(logBuffer) > 0 {
		err = osc.bulkInsert(logBuffer)
		if err != nil {
			return
		}
	}
	return
}

func (osc *openSearchConnection) bulkInsert(logBuffer []*OpenSearchLogMessage) (err error) {

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

func (osc *openSearchConnection) generateBulkJson(logBuffer []*OpenSearchLogMessage) (jsonData string, err error) {
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

	oslm := &OpenSearchLogMessage{
		AppName:    "OpenSearchLane",
		LogMessage: msg,
	}

	oslm.Metadata = make(map[string]string)
	oslm.Metadata["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	logBuffer := []*OpenSearchLogMessage{oslm}

	if osc.emergencyFn != nil {
		err := osc.emergencyFn(logBuffer)
		if err != nil {
			osc.messagesSentFailed += len(logBuffer)
		} else {
			osc.messagesSent += len(logBuffer)
		}
	}
}

func newOpenSearchClient(openSearchUrl, openSearchPort, openSearchUser, openSearchPass string) (client *opensearchapi.Client, err error) {
	client, err = opensearchapi.NewClient(
		opensearchapi.Config{
			Client: opensearch.Config{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
				Addresses: []string{fmt.Sprintf("%s:%s", openSearchUrl, openSearchPort)},
				Username:  openSearchUser,
				Password:  openSearchPass,
			},
		},
	)

	return
}
