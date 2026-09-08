package connector

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// newRAGFlowFactory 创建 RAGFlow 数据集列表探测器。
func newRAGFlowFactory(client HTTPDoer) Factory {
	return func(config Config) (connectiontest.Probe, error) {
		request, err := newRequest(config.APIURL, "api/v1/datasets", config.APIKey, "Authorization", "Bearer ")
		if err != nil {
			return nil, connectiontest.InvalidConfigError(err)
		}
		query := request.URL.Query()
		query.Set("page", "1")
		query.Set("page_size", "1")
		request.URL.RawQuery = query.Encode()
		return &httpProbe{
			client:  client,
			primary: endpoint{request: request, validate: validateRAGFlowDatasets},
		}, nil
	}
}

// validateRAGFlowDatasets 校验 RAGFlow 业务状态和数据集列表。
func validateRAGFlowDatasets(reader io.Reader) error {
	var payload struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return err
	}
	if payload.Code == nil {
		return errors.New("RAGFlow response does not contain a code")
	}
	if code := *payload.Code; code != 0 {
		cause := fmt.Errorf("RAGFlow response code %d", code)
		switch code {
		case 109, 401:
			return connectiontest.NewError(connectiontest.StageAuthenticate, connectiontest.FailureUnauthorized, cause)
		case 108, 403:
			return connectiontest.NewError(connectiontest.StageAuthorize, connectiontest.FailureForbidden, cause)
		default:
			return connectiontest.NewError(connectiontest.StageCapability, connectiontest.FailureUnavailable, cause)
		}
	}
	if len(payload.Data) == 0 || payload.Data[0] != '[' {
		return errors.New("RAGFlow response does not contain a data array")
	}
	return nil
}
