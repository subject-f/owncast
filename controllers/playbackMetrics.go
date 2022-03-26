package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/owncast/owncast/metrics"
	log "github.com/sirupsen/logrus"
)

// ReportPlaybackMetrics will accept playback metrics from a client and save
// them for future video health reporting.
func ReportPlaybackMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != POST {
		WriteSimpleResponse(w, false, r.Method+" not supported")
		return
	}

	type rateLimitsTracking struct {
		Average float64 `json:"average"`
		Max     float64 `json:"max"`
		Min     float64 `json:"min"`
	}

	type reportPlaybackMetricsRequest struct {
		Bandwidth             float64            `json:"bandwidth"`
		Latency               float64            `json:"latency"`
		Errors                float64            `json:"errors"`
		DownloadDuration      float64            `json:"downloadDuration"`
		QualityVariantChanges float64            `json:"qualityVariantChanges"`
		RateLimits            rateLimitsTracking `json:"rateLimits"`
	}

	decoder := json.NewDecoder(r.Body)
	var request reportPlaybackMetricsRequest
	if err := decoder.Decode(&request); err != nil {
		log.Errorln("error decoding playback metrics payload:", err)
		WriteSimpleResponse(w, false, err.Error())
		return
	}

	metrics.RegisterPlaybackErrorCount(request.Errors)
	metrics.RegisterPlayerBandwidth(request.Bandwidth)
	metrics.RegisterPlayerLatency(request.Latency)
	metrics.RegisterPlayerSegmentDownloadDuration(request.DownloadDuration)
	metrics.RegisterQualityVariantChangesCount(request.QualityVariantChanges)
	metrics.RegisterPlayerRateLimitAvg(request.RateLimits.Average)
	metrics.RegisterPlayerRateLimitBoundaries(request.RateLimits.Min, request.RateLimits.Max)
}
