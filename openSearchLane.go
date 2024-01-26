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
	"sync/atomic"
	"time"

	"github.com/opensearch-project/opensearch-go/v3"
	"github.com/opensearch-project/opensearch-go/v3/opensearchapi"
)

type LogTemplate struct {
	AppName      string            `json:"appName"`
	ParentLaneId string            `json:"parentLaneId,omitempty"`
	JourneyID    string            `json:"journeyId,omitempty"`
	LaneID       string            `json:"laneId"`
	LogMessage   string            `json:"logMessage"`
	Metadata     map[string]string //TODO
}

type openSearchLane struct {
	logLane
	openSearchConnection *openSearchConnection
	wg                   sync.WaitGroup
}

type openSearchConnection struct {
	client     *opensearchapi.Client
	mu         sync.Mutex
	logBuffer  []*LogTemplate
	metadata   map[string]string
	refCountCh chan *sync.WaitGroup
	refCount   int32
	index      string
	appName    string
}

type OpenSearchLane interface {
	Lane
	Connect(openSearchUrl, openSearchPort, openSearchUser, openSearchPass, openSearchIndex, openSearchAppName string) (err error)
	Metadata(args ...any)
}

func (osl *openSearchLane) Metadata(args ...any) {
	sprint(args...)
}

func (osl *openSearchLane) emergencyWrite() {
	if len(osl.openSearchConnection.logBuffer) > 100 {
		fmt.Println("Write to disk")
	}
}

func (osl *openSearchLane) Close() {
	atomic.AddInt32(&osl.openSearchConnection.refCount, -1)
	osl.openSearchConnection.refCountCh <- &osl.wg
	osl.wg.Wait()
}

func NewOpenSearchLane(ctx context.Context) (l OpenSearchLane) {
	ll := deriveLogLane(nil, ctx, []Lane{}, "")

	osl := openSearchLane{
		openSearchConnection: &openSearchConnection{
			logBuffer: make([]*LogTemplate, 0),
			refCountCh: make(chan *sync.WaitGroup),
		},
	}
	osl.wg.Add(1)
	atomic.AddInt32(&osl.openSearchConnection.refCount, 1)

	ll.clone(&osl.logLane)
	osl.logLane.writer = log.New(&osl, "", 0)
	osl.logLane.logFlags = log.Flags() &^ (log.Ldate | log.Ltime)

	l = &osl

	return

}

func (osl *openSearchLane) Connect(openSearchUrl, openSearchPort, openSearchUser, openSearchPass, openSearchIndex, openSearchAppName string) (err error) {
	if osl.openSearchConnection.client != nil {
		//close method
	}

	client, err := newOpenSearchClient(openSearchUrl, openSearchPort, openSearchUser, openSearchPass)
	if err != nil {
		return
	}

	osl.openSearchConnection.client = client
	osl.openSearchConnection.index = openSearchIndex
	osl.openSearchConnection.appName = openSearchAppName
	go osl.flushLogBuffer()

	return

}

func (osl *openSearchLane) Derive() Lane {
	ll := deriveLogLane(&osl.logLane, context.WithValue(osl.Context, parent_lane_id, osl.LaneId()), osl.tees, osl.cr)

	osl2 := openSearchLane{
		openSearchConnection: osl.openSearchConnection,
	}
	osl2.wg.Add(1)
	atomic.AddInt32(&osl.openSearchConnection.refCount, 1)

	ll.clone(&osl2.logLane)
	osl2.logLane.writer = log.New(&osl2, "", 0)

	return &osl2
}

func (osl *openSearchLane) DeriveReplaceContext(ctx context.Context) Lane {
	ll := deriveLogLane(&osl.logLane, context.WithValue(ctx, parent_lane_id, osl.LaneId()), osl.tees, osl.cr)

	osl2 := openSearchLane{
		openSearchConnection: osl.openSearchConnection,
	}
	osl2.wg.Add(1)
	atomic.AddInt32(&osl.openSearchConnection.refCount, 1)

	ll.clone(&osl2.logLane)
	osl2.logLane.writer = log.New(&osl2, "", 0)

	return &osl2
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

func (osl *openSearchLane) Write(p []byte) (n int, err error) {
	osl.openSearchConnection.mu.Lock()
	defer osl.openSearchConnection.mu.Unlock()

	logEntry := string(p)

	fmt.Println(logEntry)

	parentLaneId, ok := osl.Context.Value(parent_lane_id).(string)
	if !ok {
		parentLaneId = ""
	}

	logData := &LogTemplate{
		AppName:      osl.openSearchConnection.appName,
		ParentLaneId: parentLaneId,
		JourneyID:    osl.journeyId,
		LaneID:       osl.LaneId(),
		LogMessage:   logEntry,
	}

	osl.openSearchConnection.logBuffer = append(osl.openSearchConnection.logBuffer, logData)

	return len(p), nil
}

func (osl *openSearchLane) flushLogBuffer() {
	backoffDuration := 10 * time.Second
	ticker := time.NewTicker(backoffDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			osl.openSearchConnection.mu.Lock()
			if len(osl.openSearchConnection.logBuffer) > 0 {
				err := osl.bulkInsert(osl.openSearchConnection.logBuffer)
				if err != nil {
					backoffDuration *= 2
					if backoffDuration > 10*time.Minute {
						backoffDuration = 10 * time.Minute
					}
					ticker.Reset(backoffDuration)
				} else {
					backoffDuration = 10 * time.Second
					osl.openSearchConnection.logBuffer = osl.openSearchConnection.logBuffer[:0]
					ticker.Reset(backoffDuration)
				}
			}
			osl.openSearchConnection.mu.Unlock()
		case wg := <-osl.openSearchConnection.refCountCh:
			osl.openSearchConnection.mu.Lock()

			refCounter := atomic.LoadInt32(&osl.openSearchConnection.refCount)
			fmt.Println(refCounter)

			if wg != nil {
				wg.Done()
			}

			if refCounter == 0 {
				fmt.Println("hmmm")
				if len(osl.openSearchConnection.logBuffer) > 0 {
					err := osl.bulkInsert(osl.openSearchConnection.logBuffer)
					if err != nil {
						fmt.Print("Error inserting logs in opensearch on channel closing")
					}
				}
				osl.openSearchConnection.mu.Unlock()
				return
			}

			osl.openSearchConnection.mu.Unlock()
		}

	}

}

func (osl *openSearchLane) bulkInsert(logDataSlice []*LogTemplate) (err error) {

	jsonData, err := osl.generateBulkJson(logDataSlice)
	if err != nil {
		return
	}

	resp, err := osl.openSearchConnection.client.Bulk(osl, opensearchapi.BulkReq{Body: strings.NewReader(jsonData)})
	if err != nil {
		fmt.Println("Error in storing values in opensearch:", err)
		return
	}

	fmt.Println(resp)

	return
}

func (osl *openSearchLane) generateBulkJson(logDataSlice []*LogTemplate) (jsonData string, err error) {
	var lines []string
	var createLine []byte
	var logDataLine []byte

	for _, logData := range logDataSlice {
		createAction := map[string]interface{}{"create": map[string]interface{}{"_index": osl.openSearchConnection.index}}
		createLine, err = json.Marshal(createAction)
		if err != nil {
			fmt.Println("Error marshalling createAction JSON:", err)
			return
		}

		logDataLine, err = json.Marshal(logData)
		if err != nil {
			fmt.Println("Error marshalling logData JSON:", err)
			return
		}

		lines = append(lines, string(createLine), string(logDataLine))
	}

	jsonData = strings.Join(lines, "\n") + "\n"

	return
}
