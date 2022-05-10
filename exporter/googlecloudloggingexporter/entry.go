package googlecloudloggingexporter

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"cloud.google.com/go/logging"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func logsToEntries(logger *zap.Logger, ld pdata.Logs) ([]logging.Entry, int) {
	entries := []logging.Entry{}
	dropped := 0
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resourceAttrs := attrsValue(rl.Resource().Attributes())
		ills := rl.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			logs := ils.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				entry, err := logToEntry(logger, resourceAttrs, log)
				if err != nil {
					logger.Debug("Failed to convert to Cloud Logging Entry", zap.Error(err))
					dropped++
				} else {
					entries = append(entries, entry)
				}
			}
		}
	}
	return entries, dropped
}

func logToEntry(
	logger *zap.Logger,
	attributes map[string]interface{},
	log pdata.LogRecord) (logging.Entry, error) {
	httpRequest, message, err := parseHttpRequest(logger, log.Body().AsString())
	if err != nil {
		logger.Debug("error parsing HTTPRequest", zap.Error(err))
	}

	entry := logging.Entry{
		HTTPRequest: httpRequest,
		Timestamp:   log.Timestamp().AsTime(),
		Severity:    logging.Severity(log.SeverityNumber()),
		Trace:       log.TraceID().HexString(),
		SpanID:      log.SpanID().HexString(),
	}

	if message != "" {
		type entryPayload struct {
			Message string `json:"message"`
		}
		payload := entryPayload{
			Message: message,
		}
		entry.Payload = payload
	}

	return entry, nil
}

// JSON keys derived from:
// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#httprequest
type httpRequestLog struct {
	RequestMethod                  string `json:"requestMethod"`
	RequestURL                     string `json:"requestUrl"`
	RequestSize                    int64  `json:"requestSize,string"`
	Status                         int    `json:"status,string"`
	ResponseSize                   int64  `json:"responseSize,string"`
	UserAgent                      string `json:"userAgent"`
	RemoteIP                       string `json:"remoteIp"`
	ServerIP                       string `json:"serverIp"`
	Referer                        string `json:"referer"`
	Latency                        string `json:"latency"`
	CacheLookup                    bool   `json:"cacheLookup"`
	CacheHit                       bool   `json:"cacheHit"`
	CacheValidatedWithOriginServer bool   `json:"cacheValidatedWithOriginServer"`
	CacheFillBytes                 int64  `json:"cacheFillBytes,string"`
	Protocol                       string `json:"protocol"`
}

func parseHttpRequest(logger *zap.Logger, message string) (*logging.HTTPRequest, string, error) {
	parsedLog, strippedMessage, err := extractHttpRequestFromLog(message)
	if err != nil {
		return nil, message, err
	}

	req, err := http.NewRequest(parsedLog.RequestMethod, parsedLog.RequestURL, nil)
	if err != nil {
		return nil, message, err
	}
	req.Header.Set("Referer", parsedLog.Referer)
	req.Header.Set("User-Agent", parsedLog.UserAgent)

	httpRequest := &logging.HTTPRequest{
		Request:                        req,
		RequestSize:                    parsedLog.RequestSize,
		Status:                         parsedLog.Status,
		ResponseSize:                   parsedLog.ResponseSize,
		LocalIP:                        parsedLog.ServerIP,
		RemoteIP:                       parsedLog.RemoteIP,
		CacheHit:                       parsedLog.CacheHit,
		CacheValidatedWithOriginServer: parsedLog.CacheValidatedWithOriginServer,
		CacheFillBytes:                 parsedLog.CacheFillBytes,
		CacheLookup:                    parsedLog.CacheLookup,
	}
	if parsedLog.Latency != "" {
		latency, err := time.ParseDuration(parsedLog.Latency)
		if err != nil {
			logger.Debug("Failed to parse latency", zap.Error(err))
		} else {
			httpRequest.Latency = latency
		}
	}

	return httpRequest, strippedMessage, nil
}

func extractHttpRequestFromLog(message string) (*httpRequestLog, string, error) {
	httpRequestKey := "httpRequest"

	unmarshalledMessage := make(map[string]interface{})
	if err := json.Unmarshal([]byte(message), &unmarshalledMessage); err != nil {
		return nil, message, err
	}

	httpRequestMap, ok := unmarshalledMessage[httpRequestKey]
	if !ok {
		return nil, message, errors.New("message has no key httpRequest")
	}
	httpRequestStr, err := json.Marshal(httpRequestMap)
	if err != nil {
		return nil, message, err
	}
	var httpRequest *httpRequestLog
	if err := json.Unmarshal([]byte(httpRequestStr), &httpRequest); err != nil {
		return nil, message, err
	}

	delete(unmarshalledMessage, httpRequestKey)
	if len(unmarshalledMessage) == 0 {
		return httpRequest, "", nil
	}
	strippedMessage, err := json.Marshal(unmarshalledMessage)
	if err != nil {
		return httpRequest, message, err
	}
	return httpRequest, string(strippedMessage), nil
}

func attrsValue(attrs pdata.AttributeMap) map[string]interface{} {
	if attrs.Len() == 0 {
		return nil
	}
	out := make(map[string]interface{}, attrs.Len())
	attrs.Range(func(k string, v pdata.AttributeValue) bool {
		out[k] = attrValue(v)
		return true
	})
	return out
}

func attrValue(value pdata.AttributeValue) interface{} {
	switch value.Type() {
	case pdata.AttributeValueTypeInt:
		return value.IntVal()
	case pdata.AttributeValueTypeBool:
		return value.BoolVal()
	case pdata.AttributeValueTypeDouble:
		return value.DoubleVal()
	case pdata.AttributeValueTypeString:
		return value.StringVal()
	case pdata.AttributeValueTypeMap:
		values := map[string]interface{}{}
		value.MapVal().Range(func(k string, v pdata.AttributeValue) bool {
			values[k] = attrValue(v)
			return true
		})
		return values
	case pdata.AttributeValueTypeArray:
		arrayVal := value.SliceVal()
		values := make([]interface{}, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			values[i] = attrValue(arrayVal.At(i))
		}
		return values
	case pdata.AttributeValueTypeEmpty:
		return nil
	default:
		return nil
	}
}
