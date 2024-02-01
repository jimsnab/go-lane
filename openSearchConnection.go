package lane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/opensearch-project/opensearch-go/v3"
	"github.com/opensearch-project/opensearch-go/v3/opensearchapi"
)

const (
	OscDefaultLogThreshold    = 5
	OscDefaultMaxBufferSize   = 10
	OscDefaultBackoffInterval = 10 * time.Second
	OscDefaultBackOffLimit    = 10 * time.Minute
)

type OscConfig struct {
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

type openSearchConnection struct {
	client             *opensearchapi.Client
    clientMu           sync.Mutex
	mu                 sync.Mutex
	logBuffer          []*OslMessage
	stopCh             chan *sync.WaitGroup
	wakeCh             chan struct{}
	refCount           int
	config             *OscConfig
	emergencyFn        OslEmergencyFn
	ctx                context.Context
	cancelFn           context.CancelFunc
	messagesQueued     int
	messagesSent       int
	messagesSentFailed int
}

func (osc *openSearchConnection) connect(config *OscConfig) (err error) {
    osc.clientMu.Lock()

    var client *opensearchapi.Client

    if !config.offline && osc.client != nil && osc.client.Client != nil {
        osc.client = nil
        osc.wakeCh <- struct{}{}
    }


	if !config.offline {
		client, err = newOpenSearchClient(config.OpenSearchUrl, config.OpenSearchPort, config.OpenSearchUser, config.OpenSearchPass, config.OpenSearchTransport)
		if err != nil {
			return
		}
	}

	osc.client = client
	osc.config = config

	if config.LogThreshold == 0 {
		osc.config.LogThreshold = OscDefaultLogThreshold
	}
	if config.MaxBufferSize == 0 {
		osc.config.MaxBufferSize = OscDefaultMaxBufferSize
	}
	if config.BackoffInterval == 0 {
		osc.config.BackoffInterval = OscDefaultBackoffInterval
	}
	if config.BackOffLimit == 0 {
		osc.config.BackOffLimit = OscDefaultBackOffLimit
	}
    osc.clientMu.Unlock()

	return

}

func (osc *openSearchConnection) reconnect(config *OscConfig) (err error) {
	return osc.connect(config)
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

		case <-osc.wakeCh:
			if !osc.config.offline {
				backoffDuration = osc.send(backoffDuration)
			}

		case <-time.After(backoffDuration):
			if !osc.config.offline {
				backoffDuration = osc.send(backoffDuration)
			}
		}
	}

}

func (osc *openSearchConnection) send(backoffDuration time.Duration) time.Duration {
	osc.mu.Lock()
	logBuffer := osc.logBuffer
	osc.logBuffer = make([]*OslMessage, 0)
	osc.mu.Unlock()

	if len(logBuffer) > 0 {
        osc.clientMu.Lock()
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
        osc.clientMu.Unlock()
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
