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

type LogTemplate struct {
	AppName      string            `json:"appName"`
	ParentLaneId string            `json:"parentLaneId,omitempty"`
	JourneyID    string            `json:"journeyId,omitempty"`
	LaneID       string            `json:"laneId"`
	LogMessage   string            `json:"logMessage"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type openSearchLane struct {
	logLane
	openSearchConnection *openSearchConnection
	wg                   sync.WaitGroup
	metadata             map[string]string
}

type openSearchConnection struct {
	client     *opensearchapi.Client
	mu         sync.Mutex
	logBuffer  []*LogTemplate
	refCountCh chan *sync.WaitGroup
	refCount   int
	index      string
	appName    string
}

type OpenSearchLane interface {
	Lane
	Connect(openSearchUrl, openSearchPort, openSearchUser, openSearchPass, openSearchIndex, openSearchAppName string) (err error)
	SetEmergencyHandler()
}

func NewOpenSearchLane(ctx context.Context) (l OpenSearchLane) {
	ll := deriveLogLane(nil, ctx, []Lane{}, "")

	osl := openSearchLane{
		openSearchConnection: &openSearchConnection{
			logBuffer:  make([]*LogTemplate, 0),
			refCountCh: make(chan *sync.WaitGroup),
		},
	}
	osl.wg.Add(1)
	osl.openSearchConnection.refCount++
	go osl.refCountRoutine()

	ll.clone(&osl.logLane)
	osl.logLane.writer = log.New(&osl, "", 0)
	osl.wlog.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	l = &osl

	return

}

func (osl *openSearchLane) Connect(openSearchUrl, openSearchPort, openSearchUser, openSearchPass, openSearchIndex, openSearchAppName string) (err error) {
	if osl.openSearchConnection.client != nil {
		osl.Warn("there is already a connection to OpenSearch")
		return
	}

	client, err := newOpenSearchClient(openSearchUrl, openSearchPort, openSearchUser, openSearchPass)
	if err != nil {
		return
	}

	osl.openSearchConnection.client = client
	osl.openSearchConnection.index = openSearchIndex
	osl.openSearchConnection.appName = openSearchAppName
	go osl.openSearchConnection.processConnection(osl)

	return

}

func (osl *openSearchLane) Derive() Lane {
	ll := deriveLogLane(&osl.logLane, context.WithValue(osl.Context, parent_lane_id, osl.LaneId()), osl.tees, osl.cr)

	osl2 := openSearchLane{
		openSearchConnection: osl.openSearchConnection,
	}
	osl2.wg.Add(1)
	osl.openSearchConnection.refCount++

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
	osl.openSearchConnection.refCount++

	ll.clone(&osl2.logLane)
	osl2.logLane.writer = log.New(&osl2, "", 0)

	return &osl2
}

func (osl *openSearchLane) Metadata(key, value string) {
	osl.openSearchConnection.mu.Lock()
	defer osl.openSearchConnection.mu.Unlock()

	if osl.metadata == nil {
		osl.metadata = make(map[string]string)
		osl.metadata["timestamp"] = time.Now().String()
	}

	osl.metadata[key] = value

}

func (osl *openSearchLane) SetEmergencyHandler() {
	//TODO
	if len(osl.openSearchConnection.logBuffer) > 100 {
		fmt.Println("Write to disk")
	}
}

func (osl *openSearchLane) Close() {
	osl.openSearchConnection.refCountCh <- &osl.wg
	osl.wg.Wait()
}

func (osl *openSearchLane) refCountRoutine() {
	for {
		select {
		case wg := <-osl.openSearchConnection.refCountCh:
			osl.openSearchConnection.mu.Lock()

			osl.openSearchConnection.refCount--

			wg.Done()

			if osl.openSearchConnection.refCount == 0 {
				fmt.Println("hmmmmm")
				err := osl.openSearchConnection.flush(osl, osl.openSearchConnection.logBuffer)
				if err != nil {
					fmt.Print("error inserting logs in openSearch on lane termination")
				}
				return
			}

			osl.openSearchConnection.mu.Unlock()
		}
	}
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
		Metadata:     osl.metadata,
	}

	osl.openSearchConnection.logBuffer = append(osl.openSearchConnection.logBuffer, logData)

	return len(p), nil
}

func (osc *openSearchConnection) processConnection(l Lane) {
	var lb []*LogTemplate
	backoffDuration := 10 * time.Second
	ticker := time.NewTicker(backoffDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			
			osc.mu.Lock()
			lb = append(lb, osc.logBuffer...)
			osc.logBuffer = osc.logBuffer[:0]
			osc.mu.Unlock()

			if len(lb) > 1000 {
				fmt.Println("TODO insert data in disk")
			}
			
			err := osc.flush(l, lb)
			if err != nil {
				backoffDuration *= 2
				if backoffDuration > 10*time.Minute {
					backoffDuration = 10 * time.Minute
				}
				ticker.Reset(backoffDuration)
			} else {
				backoffDuration = 10 * time.Second
				ticker.Reset(backoffDuration)
				lb = lb[:0]
			}
		
		}
	}
}

func (osc *openSearchConnection) flush(l Lane, logBuffer []*LogTemplate) (err error) {
	if len(logBuffer) > 0 {
		err = osc.bulkInsert(l, logBuffer)
		if err != nil {
			return
		}
	}
	return
}

func (osc *openSearchConnection) bulkInsert(l Lane, logBuffer []*LogTemplate) (err error) {

	jsonData, err := osc.generateBulkJson(logBuffer)
	if err != nil {
		return
	}

	resp, err := osc.client.Bulk(l, opensearchapi.BulkReq{Body: strings.NewReader(jsonData)})
	if err != nil {
		fmt.Println("Error while storing values in opensearch:", err)
		return
	}

	fmt.Println(resp)

	return
}

func (osc *openSearchConnection) generateBulkJson(logBuffer []*LogTemplate) (jsonData string, err error) {
	var lines []string
	var createLine []byte
	var logDataLine []byte

	for _, logData := range logBuffer {
		createAction := map[string]interface{}{"create": map[string]interface{}{"_index": osc.index}}
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