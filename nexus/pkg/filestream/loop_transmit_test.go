package filestream

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/wandb/wandb/nexus/internal/clienttest"
	"github.com/wandb/wandb/nexus/pkg/observability"
)

func requestMatch(t *testing.T, fsd FsTransmitData) func(*http.Request) (*http.Response, error) {
	resp := http.Response{
		StatusCode:    200,
		Body:          io.NopCloser(strings.NewReader("{}")),
		ContentLength: 0,
	}
	return func(req *http.Request) (*http.Response, error) {
		p := FsTransmitData{}
		err := json.NewDecoder(req.Body).Decode(&p)
		assert.Nil(t, err)
		assert.Equal(t, fsd.Files, p.Files)
		return &resp, nil
	}
}

type testObj struct {
	logger *observability.NexusLogger
	client *retryablehttp.Client
	m      *clienttest.MockRoundTripper
}

func newFsTest(t *testing.T) *testObj {
	ctrl := gomock.NewController(t)

	slogger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	logger := observability.NewNexusLogger(slogger, nil)
	m := clienttest.NewMockRoundTripper(ctrl)
	client := clienttest.NewMockRetryClient(m)
	client.Logger = logger

	return &testObj{
		logger: logger,
		client: client,
		m:      m,
	}
}

func testSendAndReceive(t *testing.T, chunks []processedChunk, fsd FsTransmitData) {
	fsTest := newFsTest(t)

	fsTest.m.EXPECT().
		RoundTrip(gomock.Any()).
		DoAndReturn(requestMatch(t, fsd)).
		AnyTimes()

	fs := NewFileStream(
		WithLogger(fsTest.logger),
		WithHttpClient(fsTest.client),
	)
	for _, d := range chunks {
		fs.transmitChan <- d
	}
	fs.Close()
}

func TestSendChunks(t *testing.T) {
	send := processedChunk{
		fileType: HistoryChunk,
		fileLine: "this is a line",
	}
	expect := FsTransmitData{
		Files: map[string]fsTransmitFileData{
			"wandb-history.jsonl": fsTransmitFileData{
				Offset:  0,
				Content: []string{"this is a line"},
			},
		},
	}
	testSendAndReceive(t, []processedChunk{send}, expect)
}
